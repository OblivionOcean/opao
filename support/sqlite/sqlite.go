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
	"fmt"
	"reflect"
	"strings"

	"github.com/OblivionOcean/opao/v2/support"
)

// SqliteDialect SQLite 方言
type SqliteDialect struct{}

func (SqliteDialect) Name() string                   { return "sqlite" }
func (SqliteDialect) Quote(identifier string) string { return "\"" + identifier + "\"" }
func (SqliteDialect) BindVar(n int) string           { return "?" }

// GoTypeToSQLType 将 Go 类型映射为 SQLite SQL 类型
func (SqliteDialect) GoTypeToSQLType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return "INTEGER"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "INTEGER"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "INTEGER"
	case reflect.Float32, reflect.Float64:
		return "REAL"
	case reflect.String:
		return "TEXT"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "BLOB"
		}
		return "TEXT"
	default:
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return "TEXT"
		}
		return "TEXT"
	}
}

// BuildCreateTable 生成 SQLite 的 CREATE TABLE IF NOT EXISTS 语句
func (SqliteDialect) BuildCreateTable(table string, elems []support.Elem) (string, error) {
	if len(elems) == 0 {
		return "", fmt.Errorf("opao: 表 %s 没有定义任何字段", table)
	}

	var sb strings.Builder
	sb.WriteString("CREATE TABLE IF NOT EXISTS ")
	sb.WriteString("\"" + table + "\"")
	sb.WriteString(" (")
	sb.WriteString("\n")

	for _, elem := range elems {
		sb.WriteString("  \"")
		sb.WriteString(elem.Tag)
		sb.WriteString("\" ")
		sb.WriteString(SqliteDialect{}.GoTypeToSQLType(elem.Type))

		if elem.NotNull || elem.PrimaryKey {
			sb.WriteString(" NOT NULL")
		}

		if elem.DefaultValue != "" {
			sb.WriteString(" DEFAULT ")
			if elem.DefaultValue == "CURRENT_TIMESTAMP" || strings.HasPrefix(elem.DefaultValue, "CURRENT_TIMESTAMP") {
				sb.WriteString(elem.DefaultValue)
			} else {
				sb.WriteString("'")
				sb.WriteString(elem.DefaultValue)
				sb.WriteString("'")
			}
		}

		if elem.AutoInc {
			sb.WriteString(" PRIMARY KEY AUTOINCREMENT")
		}

		sb.WriteString(",\n")
	}

	// 如果没有自增主键，添加主键约束
	hasAutoIncPrimaryKey := false
	for _, elem := range elems {
		if elem.AutoInc && elem.PrimaryKey {
			hasAutoIncPrimaryKey = true
			break
		}
	}

	if !hasAutoIncPrimaryKey {
		var primaryKeys []string
		for _, elem := range elems {
			if elem.PrimaryKey {
				primaryKeys = append(primaryKeys, "\""+elem.Tag+"\"")
			}
		}
		if len(primaryKeys) > 0 {
			content := sb.String()
			content = content[:len(content)-2] + ",\n"
			sb.Reset()
			sb.WriteString(content)
			sb.WriteString("  PRIMARY KEY (")
			sb.WriteString(strings.Join(primaryKeys, ", "))
			sb.WriteString(")")
		} else {
			content := sb.String()
			content = content[:len(content)-2]
			sb.Reset()
			sb.WriteString(content)
		}
	} else {
		content := sb.String()
		content = content[:len(content)-2]
		sb.Reset()
		sb.WriteString(content)
	}

	sb.WriteString(")")
	return sb.String(), nil
}

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
