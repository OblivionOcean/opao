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
	"testing"

	_ "modernc.org/sqlite" // 注册名为 "sqlite" 的纯 Go 驱动（test-only 依赖）

	"github.com/OblivionOcean/opao"
)

// TestSQLiteCRUDRoundtrip 使用真实 SQLite（modernc.org 纯 Go 驱动，test-only 依赖）
// 跑通 CRUD 全链路，作为正确性回归的骨架。
func TestSQLiteCRUDRoundtrip(t *testing.T) {
	db, err := opao.NewDatabase("sqlite", "file:opao_dev_it?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.Conn.SetMaxOpenConns(1) // 内存库多连接不共享数据，限制为单连接

	if _, err := db.Conn.Exec(`CREATE TABLE users (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT    NOT NULL DEFAULT '',
		age  INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Register("users", &User{}); err != nil {
		t.Fatal(err)
	}

	// Create + 自增回填
	u := &User{Name: "张三", Age: 25}
	if err := db.Load(u).Create(); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.Id == 0 {
		t.Fatal("自增主键未回填")
	}

	// Find 单条
	finder := db.Load(&User{})
	v, err := finder.Find("id = ?", u.Id)
	if err != nil || v == nil {
		t.Fatalf("Find: %v", err)
	}
	if got := v.(*User); got.Name != "张三" || got.Age != 25 {
		t.Fatalf("Find 回读不符: %+v", got)
	}

	// Count 条件统计
	n, err := finder.Count("age > ?", 18)
	if err != nil || n != 1 {
		t.Fatalf("Count: %v n=%d", err, n)
	}

	// Update 按条件
	up := db.Load(&User{Id: u.Id, Name: "王五"})
	if err := up.Update("id = ?", u.Id); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// FindAll + Delete + 复查
	all, err := finder.FindAll("name = ?", "王五")
	if err != nil || len(all) != 1 {
		t.Fatalf("FindAll: %v n=%d", err, len(all))
	}
	if err := db.Load(&User{}).Delete("id = ?", u.Id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	n2, err := finder.Count()
	if err != nil || n2 != 0 {
		t.Fatalf("删除后应为 0 条: %v n=%d", err, n2)
	}
}

// TestSQLiteConditionBuilders 在真实库上验证常用条件构造器的往返行为。
func TestSQLiteConditionBuilders(t *testing.T) {
	db, err := opao.NewDatabase("sqlite", "file:opao_dev_cond?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.Conn.SetMaxOpenConns(1)
	if _, err := db.Conn.Exec(`CREATE TABLE users (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT    NOT NULL DEFAULT '',
		age  INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Register("users", &User{}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []*User{
		{Name: "张三", Age: 25},
		{Name: "李四", Age: 30},
		{Name: "王五", Age: 16},
	} {
		if err := db.Load(u).Create(); err != nil {
			t.Fatalf("seed Create: %v", err)
		}
	}
	q := db.Load(&User{})

	cases := []struct {
		name string
		cond any
		want int
	}{
		{"Eq", opao.Eq("name", "李四"), 1},
		{"Gt", opao.Gt("age", 18), 2},
		{"Lte", opao.Lte("age", 16), 1},
		{"Like", opao.Like("name", "%三%"), 1},
		{"InValues", opao.InValues("age", []any{25, 30}), 2},
		{"Between", opao.Between("age", 20, 35), 2},
		{"And", opao.And(opao.Gte("age", 18), opao.Eq("name", "李四")), 1},
		{"Or", opao.Or(opao.Eq("name", "张三"), opao.Eq("name", "王五")), 2},
		{"Not", opao.Not(opao.Gte("age", 18)), 1},
		{"Custom", opao.Custom("age > ? AND age < ?", 24, 31), 2},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := q.FindAll(tc.cond)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if len(got) != tc.want {
				t.Fatalf("%s: 期望 %d 条, 得到 %d 条", tc.name, tc.want, len(got))
			}
		})
	}
}
