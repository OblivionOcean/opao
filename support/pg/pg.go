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
	"strings"

	"github.com/OblivionOcean/opao/v2/support"
)

// PgDialect PostgreSQL 方言
type PgDialect struct{}

func (PgDialect) Name() string                   { return "postgres" }
func (PgDialect) Quote(identifier string) string { return "\"" + identifier + "\"" }
func (PgDialect) BindVar(n int) string           { return fmt.Sprintf("$%d", n) }

// GoTypeToSQLType 将 Go 类型映射为 PostgreSQL SQL 类型
func (PgDialect) GoTypeToSQLType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return "BOOLEAN"
	case reflect.Int, reflect.Int32:
		return "INTEGER"
	case reflect.Int8:
		return "SMALLINT"
	case reflect.Int16:
		return "SMALLINT"
	case reflect.Int64:
		return "BIGINT"
	case reflect.Uint, reflect.Uint32:
		return "INTEGER"
	case reflect.Uint8:
		return "SMALLINT"
	case reflect.Uint16:
		return "INTEGER"
	case reflect.Uint64:
		return "BIGINT"
	case reflect.Float32:
		return "REAL"
	case reflect.Float64:
		return "DOUBLE PRECISION"
	case reflect.String:
		return "VARCHAR(255)"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "BYTEA"
		}
		return "TEXT"
	default:
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return "TIMESTAMP"
		}
		return "TEXT"
	}
}

// BuildCreateTable 生成 PostgreSQL 的 CREATE TABLE IF NOT EXISTS 语句
func (PgDialect) BuildCreateTable(table string, elems []support.Elem) (string, error) {
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
		sb.WriteString(PgDialect{}.GoTypeToSQLType(elem.Type))

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
			// PostgreSQL 使用 SERIAL 或 GENERATED ALWAYS AS IDENTITY
			// 这里使用 SERIAL 方式
			colType := PgDialect{}.GoTypeToSQLType(elem.Type)
			if colType == "INTEGER" {
				sb.WriteString(" SERIAL")
			} else if colType == "BIGINT" {
				sb.WriteString(" BIGSERIAL")
			}
		}

		sb.WriteString(",\n")
	}

	// 添加主键约束
	var primaryKeys []string
	for _, elem := range elems {
		if elem.PrimaryKey {
			primaryKeys = append(primaryKeys, "\""+elem.Tag+"\"")
		}
	}
	if len(primaryKeys) > 0 {
		if !elems[len(elems)-1].PrimaryKey {
			content := sb.String()
			content = content[:len(content)-2] + ",\n"
			sb.Reset()
			sb.WriteString(content)
		} else {
			content := sb.String()
			content = content[:len(content)-2] + "\n"
			sb.Reset()
			sb.WriteString(content)
		}
		sb.WriteString("  PRIMARY KEY (")
		sb.WriteString(strings.Join(primaryKeys, ", "))
		sb.WriteString(")")
	}

	sb.WriteString(")")
	return sb.String(), nil
}

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
