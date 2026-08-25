// Copyright 2024 OblivionOcean
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/OblivionOcean/opao/v2"
)

// GormUser 模拟存量 GORM 项目模型：gorm 标签、无 db 标签、隐式命名
type GormUser struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	UserName  string `gorm:"column:user_name;size:128"`
	UserAge   int    // 无任何标签 → SnakeCase 回退 user_age
	CreatedAt string `gorm:"default:current_timestamp"` // 未支持选项应被忽略
}

func newSQLiteDB(t *testing.T, dsn string) *opao.Database {
	t.Helper()
	db, err := opao.NewDatabase("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.Conn.SetMaxOpenConns(1)
	if _, err := db.Conn.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_name TEXT NOT NULL DEFAULT '',
		user_age INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestRegisterSnakeGormModel 验证存量 gorm 模型零改动注册与 CRUD
func TestRegisterSnakeGormModel(t *testing.T) {
	db := newSQLiteDB(t, "file:opao_dev_snake?mode=memory&cache=shared")
	if err := db.RegisterSnake("users", &GormUser{}); err != nil {
		t.Fatalf("RegisterSnake: %v", err)
	}

	u := &GormUser{UserName: "张三", UserAge: 25}
	if err := db.Load(u).Create(); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("自增主键未回填")
	}
	got, err := db.Load(&GormUser{}).Find("id = ?", u.ID)
	if err != nil || got == nil {
		t.Fatalf("Find: %v", err)
	}
	if g := got.(*GormUser); g.UserName != "张三" || g.UserAge != 25 {
		t.Fatalf("列映射错误: %+v", g)
	}
}

// TestFromSQLDBSharedPool 验证外部连接池复用与共享 Close 语义
func TestFromSQLDBSharedPool(t *testing.T) {
	raw, err := sql.Open("sqlite", "file:opao_dev_shared?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_name TEXT NOT NULL DEFAULT '',
		user_age INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatal(err)
	}

	db, err := opao.FromSQLDB("sqlite", raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RegisterSnake("users", &GormUser{}); err != nil {
		t.Fatal(err)
	}
	u := &GormUser{UserName: "李四"}
	if err := db.Load(u).Create(); err != nil {
		t.Fatal(err)
	}

	// 共享模式 Close 仅解绑，外部池仍可用
	if err := db.Close(); err != nil {
		t.Fatalf("shared Close: %v", err)
	}
	var one int
	if err := raw.QueryRow("SELECT COUNT(*) FROM users").Scan(&one); err != nil || one != 1 {
		t.Fatalf("外部连接池应保持可用: %v n=%d", err, one)
	}
}

// TestGormTagPriority 验证标签优先级：db > gorm column > SnakeCase
type MixedTagUser struct {
	A int64  `db:"a_col" option:"autoIncrement"`
	B string `gorm:"column:b_col"`
	C int    // → snake: c
}
