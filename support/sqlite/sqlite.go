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

// Package sqlite 提供 SQLite 方言与驱动包装。
package sqlite

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/OblivionOcean/opao/v2/support"
)

// SqliteDialect SQLite 方言
type SqliteDialect struct{}

func (SqliteDialect) Name() string                   { return "sqlite" }
func (SqliteDialect) Quote(identifier string) string { return "\"" + identifier + "\"" }
func (SqliteDialect) BindVar(n int) string           { return "?" }

// Dialect SQLite 方言单例
var Dialect support.Dialect = SqliteDialect{}

// Sqlite SQLite 数据库 ORM 实现
type Sqlite struct{ *support.Driver }

// New 创建 SQLite ORM 实例
func New(conn *sql.DB, obj any, objType reflect.Type, table string, elems []support.Elem, err error) support.ObjectORM {
	return &Sqlite{support.NewDriver(Dialect, conn, obj, objType, table, elems, err)}
}

// WithTx 返回绑定事务执行的副本。
func (qt *Sqlite) WithTx(tx *sql.Tx) *Sqlite {
	c := *qt.Driver
	c.Exec = tx
	return &Sqlite{&c}
}

// WithCtx 返回仅替换上下文的副本。
func (qt *Sqlite) WithCtx(ctx context.Context) *Sqlite {
	c := *qt.Driver
	if ctx == nil {
		ctx = context.Background()
	}
	c.Ctx = ctx
	return &Sqlite{&c}
}
