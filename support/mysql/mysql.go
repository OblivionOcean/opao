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
	"fmt"
	"reflect"
	"strings"

	"github.com/OblivionOcean/opao/v2/support"
)

// MySQLDialect MySQL 方言
type MySQLDialect struct{}

func (MySQLDialect) Name() string                   { return "mysql" }
func (MySQLDialect) Quote(identifier string) string { return "`" + identifier + "`" }
func (MySQLDialect) BindVar(n int) string           { return "?" }

// GoTypeToSQLType 将 Go 类型映射为 MySQL SQL 类型
func (MySQLDialect) GoTypeToSQLType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Bool:
		return "TINYINT(1)"
	case reflect.Int:
		return "INT"
	case reflect.Int8:
		return "TINYINT"
	case reflect.Int16:
		return "SMALLINT"
	case reflect.Int32:
		return "INT"
	case reflect.Int64:
		return "BIGINT"
	case reflect.Uint:
		return "INT UNSIGNED"
	case reflect.Uint8:
		return "TINYINT UNSIGNED"
	case reflect.Uint16:
		return "SMALLINT UNSIGNED"
	case reflect.Uint32:
		return "INT UNSIGNED"
	case reflect.Uint64:
		return "BIGINT UNSIGNED"
	case reflect.Float32:
		return "FLOAT"
	case reflect.Float64:
		return "DOUBLE"
	case reflect.String:
		return "VARCHAR(255)"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "BLOB"
		}
		return "TEXT"
	default:
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return "DATETIME"
		}
		return "TEXT"
	}
}

// BuildCreateTable 生成 MySQL 的 CREATE TABLE IF NOT EXISTS 语句
func (MySQLDialect) BuildCreateTable(table string, elems []support.Elem) (string, error) {
	if len(elems) == 0 {
		return "", fmt.Errorf("opao: 表 %s 没有定义任何字段", table)
	}

	var sb strings.Builder
	sb.WriteString("CREATE TABLE IF NOT EXISTS ")
	sb.WriteString("`" + table + "` (")
	sb.WriteString("\n")

	for _, elem := range elems {
		sb.WriteString("  `")
		sb.WriteString(elem.Tag)
		sb.WriteString("` ")
		sb.WriteString(MySQLDialect{}.GoTypeToSQLType(elem.Type))

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
			sb.WriteString(" AUTO_INCREMENT")
		}

		sb.WriteString(",\n")
	}

	// 添加主键约束
	var primaryKeys []string
	for _, elem := range elems {
		if elem.PrimaryKey {
			primaryKeys = append(primaryKeys, "`"+elem.Tag+"`")
		}
	}
	if len(primaryKeys) > 0 {
		// 如果最后一个元素不是主键，需要补逗号
		if !elems[len(elems)-1].PrimaryKey {
			// 回退换行符，添加逗号+换行
			content := sb.String()
			content = content[:len(content)-2] + ",\n"
			sb.Reset()
			sb.WriteString(content)
		} else {
			// 回退换行符
			content := sb.String()
			content = content[:len(content)-2] + "\n"
			sb.Reset()
			sb.WriteString(content)
		}
		sb.WriteString("  PRIMARY KEY (")
		sb.WriteString(strings.Join(primaryKeys, ", "))
		sb.WriteString(")")
	}

	sb.WriteString(") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")
	return sb.String(), nil
}

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
