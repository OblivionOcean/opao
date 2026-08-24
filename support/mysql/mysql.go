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

// Package mysql 提供 MySQL 方言与驱动包装。
package mysql

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/OblivionOcean/opao/support"
)

// MySQLDialect MySQL 方言
type MySQLDialect struct{}

func (MySQLDialect) Name() string                   { return "mysql" }
func (MySQLDialect) Quote(identifier string) string { return "`" + identifier + "`" }
func (MySQLDialect) BindVar(n int) string           { return "?" }

// Dialect MySQL 方言单例
var Dialect support.Dialect = MySQLDialect{}

// MySQL MySQL 数据库 ORM 实现
type MySQL struct{ *support.Driver }

// New 创建 MySQL ORM 实例（兼容旧名 NewMySQL 的别名见 AliasNew）
func New(conn *sql.DB, obj any, objType reflect.Type, table string, elems []support.Elem, err error) support.ObjectORM {
	return &MySQL{support.NewDriver(Dialect, conn, obj, objType, table, elems, err)}
}

// WithTx 返回共享 Table/Elems 元数据但绑定事务执行的副本。
func (qt *MySQL) WithTx(tx *sql.Tx) *MySQL {
	c := *qt.Driver
	c.Exec = tx
	return &MySQL{&c}
}

// WithCtx 返回仅替换上下文的副本，后续所有调用透传该 ctx。
func (qt *MySQL) WithCtx(ctx context.Context) *MySQL {
	c := *qt.Driver
	if ctx == nil {
		ctx = context.Background()
	}
	c.Ctx = ctx
	return &MySQL{&c}
}
