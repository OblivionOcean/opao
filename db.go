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

package opao

import (
	"database/sql"
	"errors"
	"reflect"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/mysql"
	"github.com/OblivionOcean/opao/support/pg"
	"github.com/OblivionOcean/opao/support/sqlite"
)

const (
	EmptyQ = ""
)

type Database struct {
	Conn          *sql.DB
	sqlDriverName string
	support.ORM
}

func NewDatabase(sqlDriverName, linkInfo string) (*Database, error) {
	conn, err := sql.Open(sqlDriverName, linkInfo)
	if err != nil {
		return nil, err
	}
	db := &Database{Conn: conn}
	db.sqlDriverName = sqlDriverName
	db.ORM = support.ORM{}
	var driver func(*sql.DB, any, reflect.Type, string, []support.Elem, error) support.ObjectORM

	switch db.sqlDriverName {
	case "mysql":
		driver = mysql.NewMySQL
	case "postgres", "pg", "pgsql":
		driver = pg.NewPg
	case "sqlite3", "sqlite":
		driver = sqlite.NewSqlite
	default:
		return nil, errors.New("driver not supported")
	}

	db.ORM.Init(db.Conn, driver)

	return db, nil
}

// New 是对 NewDatabase 的别名，以匹配 README 用法
func New(sqlDriverName, linkInfo string) (*Database, error) {
	return NewDatabase(sqlDriverName, linkInfo)
}

func (db *Database) Close() error {
	if db.Conn == nil {
		return errors.New("database is not initialized")
	}
	return db.Conn.Close()
}

func (db *Database) GetConn() *sql.DB {
	return db.Conn
}
