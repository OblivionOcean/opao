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

// Package pg 提供 PostgreSQL 方言与驱动包装。
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/OblivionOcean/opao/support"
)

// PgDialect PostgreSQL 方言
type PgDialect struct{}

func (PgDialect) Name() string                   { return "postgres" }
func (PgDialect) Quote(identifier string) string { return "\"" + identifier + "\"" }
func (PgDialect) BindVar(n int) string           { return fmt.Sprintf("$%d", n) }

// Dialect PostgreSQL 方言单例
var Dialect support.Dialect = PgDialect{}

// PgSQL PostgreSQL 数据库 ORM 实现
type PgSQL struct{ *support.Driver }

// New 创建 PostgreSQL ORM 实例
func New(conn *sql.DB, obj any, objType reflect.Type, table string, elems []support.Elem, err error) support.ObjectORM {
	return &PgSQL{support.NewDriver(Dialect, conn, obj, objType, table, elems, err)}
}

// WithTx 返回绑定事务执行的副本。
func (qt *PgSQL) WithTx(tx *sql.Tx) *PgSQL {
	c := *qt.Driver
	c.Exec = tx
	return &PgSQL{&c}
}

// WithCtx 返回仅替换上下文的副本。
func (qt *PgSQL) WithCtx(ctx context.Context) *PgSQL {
	c := *qt.Driver
	if ctx == nil {
		ctx = context.Background()
	}
	c.Ctx = ctx
	return &PgSQL{&c}
}
