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
	"context"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/OblivionOcean/opao/v2"
)

// newPreparedDB 创建 M4-L3 Prepared Statement 场景专用的 sqlite 内存库
// （独立 DSN 隔离数据；单连接保证与 tx 及语句缓存串行）
func newPreparedDB(t *testing.T, name string) *opao.Database {
	t.Helper()
	db, err := opao.NewDatabase("sqlite", "file:"+name+"?mode=memory&cache=shared")
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
	return db
}

// TestSugarPrepareStmtCRUDMatchesPlain 糖层 .PrepareStmt() 的 CRUD 往返
// 与非预编译路径结果一致：自增回填、条件查询、部分更新、计数、删除。
func TestSugarPrepareStmtCRUDMatchesPlain(t *testing.T) {
	db := newPreparedDB(t, "opao_dev_m4_prepared_crud")

	a := &User{Name: "预编译", Age: 20}
	if err := db.Model(&User{}).PrepareStmt().Create(a); err != nil {
		t.Fatalf("预编译 Create: %v", err)
	}
	if a.Id != 1 {
		t.Fatalf("预编译路径自增回填应为 1, got %d", a.Id)
	}

	b := &User{Name: "普通", Age: 21}
	if err := db.Model(&User{}).Create(b); err != nil {
		t.Fatalf("非预编译 Create: %v", err)
	}
	if b.Id != 2 {
		t.Fatalf("普通路径自增回填应为 2, got %d", b.Id)
	}

	// 两条路径的条件查询互为镜像，结果集语义一致
	var gotPrepared []User
	if err := db.Model(&User{}).PrepareStmt().Where(opao.Eq("name", "预编译")).Find(&gotPrepared); err != nil {
		t.Fatalf("预编译 Find: %v", err)
	}
	var gotPlain []User
	if err := db.Model(&User{}).Where(opao.Eq("id", int64(1))).Find(&gotPlain); err != nil {
		t.Fatalf("普通 Find: %v", err)
	}
	if len(gotPrepared) != 1 || len(gotPlain) != 1 ||
		gotPrepared[0].Name != gotPlain[0].Name || gotPrepared[0].Age != gotPlain[0].Age {
		t.Fatalf("两路径结果不一致: prepared=%+v plain=%+v", gotPrepared, gotPlain)
	}

	// 预编译 Updates + First 读回校验
	if err := db.Model(&User{}).PrepareStmt().Where(opao.Eq("id", a.Id)).
		Updates(map[string]any{"age": 22}); err != nil {
		t.Fatalf("预编译 Updates: %v", err)
	}
	var one User
	if err := db.Model(&User{}).PrepareStmt().Where(opao.Eq("id", a.Id)).First(&one); err != nil {
		t.Fatalf("预编译 First: %v", err)
	}
	if one.Age != 22 {
		t.Fatalf("更新未生效: %+v", one)
	}

	// 预编译 Count 与 Delete 与普通路径口径一致
	n, err := db.Model(&User{}).PrepareStmt().Count()
	if err != nil || n != 2 {
		t.Fatalf("预编译 Count 应为 2: %v n=%d", err, n)
	}
	if n2, err := db.Model(&User{}).Count(); err != nil || n2 != 2 {
		t.Fatalf("普通 Count 应为 2: %v n=%d", err, n2)
	}
	if err := db.Model(&User{}).PrepareStmt().Where(opao.Eq("id", b.Id)).Delete(); err != nil {
		t.Fatalf("预编译 Delete: %v", err)
	}
	if n3, err := db.Model(&User{}).Count(); err != nil || n3 != 1 {
		t.Fatalf("删除后 Count 应为 1: %v n=%d", err, n3)
	}
}

// TestSugarPrepareStmtLoopStable 同一构建器循环执行 10 次：
// 命中同一份预编译语句，结果稳定且与首查一致。
func TestSugarPrepareStmtLoopStable(t *testing.T) {
	db := newPreparedDB(t, "opao_dev_m4_prepared_loop")
	for _, f := range []*User{
		{Name: "a", Age: 25},
		{Name: "b", Age: 25},
		{Name: "c", Age: 30},
	} {
		if err := db.Load(f).Create(); err != nil {
			t.Fatal(err)
		}
	}

	q := db.Model(&User{}).PrepareStmt().Where(opao.Gte("age", 25)).Order("id").Limit(2)
	for i := 0; i < 10; i++ {
		rows, err := q.FindAll()
		if err != nil {
			t.Fatalf("第 %d 次查询: %v", i+1, err)
		}
		if len(rows) != 2 {
			t.Fatalf("第 %d 次查询应返回 2 行, 得到 %d", i+1, len(rows))
		}
		first, second := rows[0].(User), rows[1].(User)
		if first.Name != "a" || second.Name != "b" {
			t.Fatalf("第 %d 次结果漂移: %+v %+v", i+1, first, second)
		}

		var one User
		if err := db.Model(&User{}).PrepareStmt().Where(opao.Eq("name", "c")).First(&one); err != nil {
			t.Fatalf("第 %d 次单行查询: %v", i+1, err)
		}
		if one.Age != 30 {
			t.Fatalf("第 %d 次单行结果漂移: %+v", i+1, one)
		}
	}
}

// TestSugarPrepareStmtTxRollback .PrepareStmt() 与 .Tx 组合：
// 事务内写入随 Rollback 消失，外部计数不变；两种链式顺序等价。
func TestSugarPrepareStmtTxRollback(t *testing.T) {
	db := newPreparedDB(t, "opao_dev_m4_prepared_tx")

	tx, err := db.Conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	u := &User{Name: "rollback"}
	if err := db.Model(&User{}).PrepareStmt().Tx(tx).Create(u); err != nil {
		t.Fatalf("预编译事务内 Create: %v", err)
	}
	if n, err := db.Model(&User{}).Tx(tx).PrepareStmt().Count(); err != nil || n != 1 {
		t.Fatalf("事务内计数应为 1（反向链序）: %v n=%d", err, n)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	n, err := db.Model(&User{}).Count()
	if err != nil || n != 0 {
		t.Fatalf("回滚后计数应不变(0): %v n=%d", err, n)
	}
}

// TestPrepareStmtConn 手动入口可用：ExecContext/QueryRowContext/QueryContext
// 经 LRU 缓存完成 CRUD；未初始化的 Database 返回错误。
func TestPrepareStmtConn(t *testing.T) {
	db := newPreparedDB(t, "opao_dev_m4_prepared_conn")

	if _, err := (&opao.Database{}).PrepareStmtConn(); err == nil {
		t.Fatal("未初始化的 Database 应返回错误")
	}

	pe, err := db.PrepareStmtConn()
	if err != nil {
		t.Fatalf("PrepareStmtConn: %v", err)
	}
	ctx := context.Background()

	res, err := pe.ExecContext(ctx, "INSERT INTO users(name, age) VALUES (?, ?)", "conn", 33)
	if err != nil {
		t.Fatalf("ExecContext 插入: %v", err)
	}
	if id, err := res.LastInsertId(); err != nil || id != 1 {
		t.Fatalf("LastInsertId 应为 1: %v id=%d", err, id)
	}
	// 同形语句再次执行：命中缓存路径仍正确
	if _, err := pe.ExecContext(ctx, "INSERT INTO users(name, age) VALUES (?, ?)", "conn", 34); err != nil {
		t.Fatalf("ExecContext 复用: %v", err)
	}

	var total int
	if err := pe.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE name = ?", "conn").Scan(&total); err != nil || total != 2 {
		t.Fatalf("QueryRowContext 计数应为 2: %v n=%d", err, total)
	}

	rows, err := pe.QueryContext(ctx, "SELECT id, age FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	defer rows.Close()
	ages := []int{}
	for rows.Next() {
		var id, age int64
		if err := rows.Scan(&id, &age); err != nil {
			t.Fatal(err)
		}
		ages = append(ages, int(age))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(ages) != 2 || ages[0] != 33 || ages[1] != 34 {
		t.Fatalf("行集不符: %v", ages)
	}

	if err := pe.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
