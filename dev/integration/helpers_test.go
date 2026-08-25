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

// Package integration 包含真实 SQL 往返与已知缺陷的红灯回归测试。
// 依赖均为测试专用（sqlmock / modernc.org/sqlite），不影响根模块零依赖承诺。
package integration

import (
	"database/sql"
	"reflect"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"github.com/OblivionOcean/opao/v2/support"
)

type User struct {
	Id   int64  `db:"id" option:"autoIncrement"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

// newMockORM 创建精确匹配 SQL 文本（QueryMatcherEqual）的 sqlmock 环境
func newMockORM(t *testing.T, driver func(*sql.DB, any, reflect.Type, string, []support.Elem, error) support.ObjectORM) (*support.ORM, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	orm := &support.ORM{}
	orm.Init(db, driver)
	if err := orm.Register("users", &User{}); err != nil {
		t.Fatal(err)
	}
	return orm, mock
}

func mustLoad(t *testing.T, orm *support.ORM, u *User) support.ObjectORM {
	t.Helper()
	o := orm.Load(u)
	if err := o.Error(); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	return o
}

func newUserRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "name", "age"})
}

// noPanic 将条件解析阶段的 panic 转为明确的测试失败
func noPanic(t *testing.T, name string, fn func() error) error {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s 发生 panic: %v", name, r)
		}
	}()
	return fn()
}
