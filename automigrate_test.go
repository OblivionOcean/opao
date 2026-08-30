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

package opao_test

import (
	"reflect"
	"testing"

	"github.com/OblivionOcean/opao/v2/support"
	"github.com/OblivionOcean/opao/v2/support/mysql"
	"github.com/OblivionOcean/opao/v2/support/pg"
	"github.com/OblivionOcean/opao/v2/support/sqlite"
)

// 测试用模型结构体
type TestUser struct {
	ID        int64  `db:"id" option:"autoIncrement;primaryKey"`
	Name      string `db:"name" option:"notNull"`
	Email     string `db:"email" option:"notNull"`
	Age       int    `db:"age"`
	IsActive bool   `db:"is_active" option:"default=true"`
}

type TestProduct struct {
	ID          int64   `db:"id" option:"autoIncrement;primaryKey"`
	Name        string  `db:"name" option:"notNull"`
	Price       float64 `db:"price" option:"notNull"`
	Description string  `db:"description"`
	Stock       int     `db:"stock" option:"default=0"`
}

func TestBuildCreateTableMySQL(t *testing.T) {
	// 创建测试元素
	elems := []support.Elem{
		{
			Index:        0,
			Type:         reflect.TypeOf(int64(0)),
			Tag:          "id",
			PrimaryKey:   true,
			AutoInc:      true,
			NotNull:      true,
		},
		{
			Index:        1,
			Type:         reflect.TypeOf(""),
			Tag:          "name",
			NotNull:      true,
		},
		{
			Index:        2,
			Type:         reflect.TypeOf(""),
			Tag:          "email",
			NotNull:      true,
		},
		{
			Index:        3,
			Type:         reflect.TypeOf(0),
			Tag:          "age",
		},
		{
			Index:        4,
			Type:         reflect.TypeOf(false),
			Tag:          "is_active",
			DefaultValue: "true",
		},
	}

	// 测试MySQL方言
	query, err := mysql.Dialect.BuildCreateTable("users", elems)
	if err != nil {
		t.Fatalf("MySQL BuildCreateTable失败: %v", err)
	}
	t.Logf("MySQL CREATE TABLE: %s", query)
	if query == "" {
		t.Fatal("生成的SQL不能为空")
	}
}

func TestBuildCreateTablePostgreSQL(t *testing.T) {
	// 创建测试元素
	elems := []support.Elem{
		{
			Index:        0,
			Type:         reflect.TypeOf(int64(0)),
			Tag:          "id",
			PrimaryKey:   true,
			AutoInc:      true,
			NotNull:      true,
		},
		{
			Index:        1,
			Type:         reflect.TypeOf(""),
			Tag:          "name",
			NotNull:      true,
		},
		{
			Index:        2,
			Type:         reflect.TypeOf(""),
			Tag:          "email",
			NotNull:      true,
		},
		{
			Index:        3,
			Type:         reflect.TypeOf(0),
			Tag:          "age",
		},
		{
			Index:        4,
			Type:         reflect.TypeOf(false),
			Tag:          "is_active",
			DefaultValue: "true",
		},
	}

	// 测试PostgreSQL方言
	query, err := pg.Dialect.BuildCreateTable("users", elems)
	if err != nil {
		t.Fatalf("PostgreSQL BuildCreateTable失败: %v", err)
	}
	t.Logf("PostgreSQL CREATE TABLE: %s", query)
	if query == "" {
		t.Fatal("生成的SQL不能为空")
	}
}

func TestBuildCreateTableSQLite(t *testing.T) {
	// 创建测试元素
	elems := []support.Elem{
		{
			Index:        0,
			Type:         reflect.TypeOf(int64(0)),
			Tag:          "id",
			PrimaryKey:   true,
			AutoInc:      true,
			NotNull:      true,
		},
		{
			Index:        1,
			Type:         reflect.TypeOf(""),
			Tag:          "name",
			NotNull:      true,
		},
		{
			Index:        2,
			Type:         reflect.TypeOf(""),
			Tag:          "email",
			NotNull:      true,
		},
		{
			Index:        3,
			Type:         reflect.TypeOf(0),
			Tag:          "age",
		},
		{
			Index:        4,
			Type:         reflect.TypeOf(false),
			Tag:          "is_active",
			DefaultValue: "true",
		},
	}

	// 测试SQLite方言
	query, err := sqlite.Dialect.BuildCreateTable("users", elems)
	if err != nil {
		t.Fatalf("SQLite BuildCreateTable失败: %v", err)
	}
	t.Logf("SQLite CREATE TABLE: %s", query)
	if query == "" {
		t.Fatal("生成的SQL不能为空")
	}
}

func TestBuildCreateTableEmpty(t *testing.T) {
	// 测试空元素列表
	_, err := mysql.Dialect.BuildCreateTable("users", []support.Elem{})
	if err == nil {
		t.Fatal("空元素列表应该返回错误")
	}
}
