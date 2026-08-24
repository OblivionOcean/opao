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
	"reflect"
	"testing"
	"unsafe"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"github.com/OblivionOcean/opao"
	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/mysql"
	"github.com/OblivionOcean/opao/support/pg"
)

const (
	mySelectAll = "SELECT `id`,`name`,`age` FROM `users`"
	pgSelectAll = "SELECT \"id\",\"name\",\"age\" FROM \"users\""
)

// Bug01: PG Update 的 SET 子句缺少列名，生成 `"=$2,"=$3`
func TestBug01_PgUpdateMissingColumnName(t *testing.T) {
	orm, mock := newMockORM(t, pg.New)
	o := mustLoad(t, orm, &User{Id: 1, Name: "李四"})
	mock.ExpectExec(`UPDATE "users" SET "name" = $1 WHERE id = $2`).
		WithArgs("李四", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := o.Update("id = ?", 1); err != nil {
		t.Fatalf("PG Update 应生成含列名的 SET 子句: %v", err)
	}
}

// Bug02: MySQL Create 占位符遍历全部字段，但值跳过自增字段，数量不匹配
func TestBug02_MySQLCreatePlaceholderMismatch(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	o := mustLoad(t, orm, &User{Name: "张三", Age: 25})
	mock.ExpectExec("INSERT INTO `users` (`name`,`age`) VALUES (?,?)").
		WithArgs("张三", 25).
		WillReturnResult(sqlmock.NewResult(9, 1))
	if err := noPanic(t, "Create", o.Create); err != nil {
		t.Fatalf("MySQL Create 占位符应与列数一致(跳过自增): %v", err)
	}
}

// Bug03: PG Create 列表包含全部字段但 $n 占位符跳过自增且编号错位
func TestBug03_PgCreateColumnPlaceholderMismatch(t *testing.T) {
	orm, mock := newMockORM(t, pg.New)
	o := mustLoad(t, orm, &User{Name: "张三", Age: 25})
	mock.ExpectExec(`INSERT INTO "users" ("name","age") VALUES ($1,$2)`).
		WithArgs("张三", 25).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := noPanic(t, "Create", o.Create); err != nil {
		t.Fatalf("PG Create 列与占位符应一致(跳过自增): %v", err)
	}
}

// Bug04: Between 条件在解析端要求 len(Args)==3 而构造器只放 2 个，必然 panic
func TestBug04_BetweenPanics(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	o := mustLoad(t, orm, &User{})
	mock.ExpectQuery(mySelectAll+" WHERE age BETWEEN ? AND ?").
		WithArgs(18, 30).
		WillReturnRows(newUserRows().AddRow(1, "x", 20))
	err := noPanic(t, "FindAll(Between)", func() error {
		_, err := o.FindAll(opao.Between("age", 18, 30))
		return err
	})
	if err != nil {
		t.Fatalf("Between 查询失败: %v", err)
	}
}

// Bug05: Custom 把 Args[0]（第一个绑定参数）当条件文本渲染
func TestBug05_CustomRendersWrongPart(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	o := mustLoad(t, orm, &User{})
	mock.ExpectQuery(mySelectAll + " WHERE JSON_EXTRACT(data, '$.key') = ?").
		WithArgs("value").
		WillReturnRows(newUserRows())
	err := noPanic(t, "FindAll(Custom)", func() error {
		_, err := o.FindAll(opao.Custom("JSON_EXTRACT(data, '$.key') = ?", "value"))
		return err
	})
	if err != nil {
		t.Fatalf("Custom 条件渲染错误: %v", err)
	}
}

// Bug06: Limit(n) 因 Left==nil 在解析端被静默忽略
func TestBug06_LimitSilentlyIgnored(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	o := mustLoad(t, orm, &User{})
	mock.ExpectQuery(mySelectAll + " LIMIT ?").
		WithArgs(10).
		WillReturnRows(newUserRows().AddRow(1, "x", 20))
	err := noPanic(t, "FindAll(Limit)", func() error {
		_, err := o.FindAll(opao.Limit(10))
		return err
	})
	if err != nil {
		t.Fatalf("Limit 未生效: %v", err)
	}
}

// Bug06b: LimitOffset 渲染为 `LIMIT ?, ?` 且参数序为 limit,offset（语义颠倒）
func TestBug06b_LimitOffsetOrder(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	o := mustLoad(t, orm, &User{})
	mock.ExpectQuery(mySelectAll+" LIMIT ? OFFSET ?").
		WithArgs(10, 20).
		WillReturnRows(newUserRows().AddRow(1, "x", 20))
	err := noPanic(t, "FindAll(LimitOffset)", func() error {
		_, err := o.FindAll(opao.LimitOffset(10, 20))
		return err
	})
	if err != nil {
		t.Fatalf("LimitOffset 未按 LIMIT n OFFSET m 语义生效: %v", err)
	}
}

// Bug07: In() 渲染单占位符 IN (?)，切片作为单参数绑定必然运行时报错
func TestBug07_InSinglePlaceholder(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	o := mustLoad(t, orm, &User{})
	mock.ExpectQuery(mySelectAll+" WHERE id IN (?,?,?)").
		WithArgs(1, 2, 3).
		WillReturnRows(newUserRows().AddRow(2, "y", 19))
	err := noPanic(t, "FindAll(In)", func() error {
		_, err := o.FindAll(opao.In("id", []any{1, 2, 3}))
		return err
	})
	if err != nil {
		t.Fatalf("In 应展开为 N 个占位符: %v", err)
	}
}

// Bug08: AND 嵌套 OR 时无括号，运算优先级错误
func TestBug08_AndOrMissingParentheses(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	o := mustLoad(t, orm, &User{})
	cond := opao.And(
		opao.Or(opao.Eq("name", "张三"), opao.Eq("name", "李四")),
		opao.Gte("age", 18),
	)
	mock.ExpectQuery(mySelectAll+" WHERE (name = ? OR name = ?) AND age >= ?").
		WithArgs("张三", "李四", 18).
		WillReturnRows(newUserRows().AddRow(1, "x", 20))
	err := noPanic(t, "FindAll(And/Or)", func() error {
		_, err := o.FindAll(cond)
		return err
	})
	if err != nil {
		t.Fatalf("嵌套条件应自动加括号: %v", err)
	}
}

// Bug09: Load 直接改写共享缓存 []Elem 的 Ptr，同类型第二次 Load 污染先前 ORM 对象
func TestBug09_LoadSharedCachePollution(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	orm := &support.ORM{}
	orm.Init(db, mysql.New)
	if err := orm.Register("users", &User{}); err != nil {
		t.Fatal(err)
	}

	u1 := &User{Id: 1, Name: "a"}
	u2 := &User{Id: 2, Name: "b"}
	o1 := orm.Load(u1)
	if o1.Error() != nil {
		t.Fatal(o1.Error())
	}
	o2 := orm.Load(u2)
	if o2.Error() != nil {
		t.Fatal(o2.Error())
	}

	m1 := o1.(*mysql.MySQL)
	base1 := reflect.ValueOf(u1).UnsafePointer()
	for _, e := range m1.Elems {
		if e.Ptr != unsafe.Add(base1, e.Offset) {
			t.Fatalf("加载第二个对象后，第一个 ORM 实例的字段指针被污染 (elem=%s)", e.Tag)
		}
	}
	m2 := o2.(*mysql.MySQL)
	base2 := reflect.ValueOf(u2).UnsafePointer()
	for _, e := range m2.Elems {
		if e.Ptr != unsafe.Add(base2, e.Offset) {
			t.Fatalf("第二个 ORM 实例字段指针指向错误对象 (elem=%s)", e.Tag)
		}
	}
}

// Bug10 回归：Update/Save 后不得再用 LastInsertId 回写自增主键（历史缺陷会把
// 内存 ID 清零）。使用精确匹配器锁定最终 SQL 形态。
func TestBug10_UpdateZeroesInMemoryID(t *testing.T) {
	orm, mock := newMockORM(t, mysql.New)
	u := &User{Id: 7, Name: "x"}
	o := mustLoad(t, orm, u)
	mock.ExpectExec("UPDATE `users` SET `name` = ? WHERE id = ?").
		WithArgs("x", 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := o.Update("id = ?", 7); err != nil {
		t.Fatalf("Update 失败: %v", err)
	}
	if u.Id != 7 {
		t.Fatalf("Update 后内存中 ID 被清零: got %d", u.Id)
	}
}

// Bug11: 无条件 Delete 静默清空全表，应当拒绝并要求显式放行（M2 引入 ErrMissingWhereClause）
func TestBug11_DeleteWithoutWhereWipesTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	orm := &support.ORM{}
	orm.Init(db, mysql.New)
	if err := orm.Register("users", &User{}); err != nil {
		t.Fatal(err)
	}
	o := mustLoad(t, orm, &User{Name: "keepme"}) // 保证有非零字段可触发执行路径
	mock.ExpectExec("^DELETE FROM `users`$").
		WillReturnResult(sqlmock.NewResult(0, 5))
	if err := noPanic(t, "Delete", func() error { return o.Delete() }); err == nil {
		t.Fatal("无条件 Delete 应当返回 ErrMissingWhereClause 而非清空全表")
	}
}
