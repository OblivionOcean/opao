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
	"context"
	"database/sql"
	"errors"
	"reflect"

	"github.com/OblivionOcean/opao/v2/support"
	"github.com/OblivionOcean/opao/v2/support/mysql"
	"github.com/OblivionOcean/opao/v2/support/pg"
	"github.com/OblivionOcean/opao/v2/support/sqlite"
)

const (
	EmptyQ = ""
)

type Database struct {
	Conn          *sql.DB
	sqlDriverName string
	shared        bool // FromSQLDB 共享模式：Close 不关闭外部连接池
	support.ORM
}

// driverFor 根据驱动名返回对应的 ORM 构造函数
func driverFor(name string) (func(*sql.DB, any, reflect.Type, string, []support.Elem, error) support.ObjectORM, error) {
	switch name {
	case "mysql":
		return mysql.New, nil
	case "postgres", "pg", "pgsql":
		return pg.New, nil
	case "sqlite3", "sqlite":
		return sqlite.New, nil
	default:
		return nil, errors.New("opao: 不支持的数据库驱动: " + name)
	}
}

func NewDatabase(sqlDriverName, linkInfo string) (*Database, error) {
	driver, err := driverFor(sqlDriverName)
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(sqlDriverName, linkInfo)
	if err != nil {
		return nil, err
	}
	db := &Database{Conn: conn, sqlDriverName: sqlDriverName}
	db.ORM.Init(conn, driver)
	return db, nil
}

// FromSQLDB 复用外部 *sql.DB 连接池（例如 GORM 项目中 gormDB.DB() 的句柄），
// 与既有数据层共存于同一连接池。dialectName 取值：mysql / postgres(pg) / sqlite3(sqlite)。
// 共享模式下 Close() 仅解绑引用，不会关闭外部连接池。
func FromSQLDB(dialectName string, conn *sql.DB) (*Database, error) {
	driver, err := driverFor(dialectName)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("opao: conn 不能为空")
	}
	db := &Database{Conn: conn, sqlDriverName: dialectName, shared: true}
	db.ORM.Init(conn, driver)
	return db, nil
}

// New 是对 NewDatabase 的别名，以匹配 README 用法
func New(sqlDriverName, linkInfo string) (*Database, error) {
	return NewDatabase(sqlDriverName, linkInfo)
}

// RegisterSnake 以蛇形命名回退注册模型：无显式列名标签（db / gorm column）的
// 字段按 SnakeCase(字段名) 推断列名，用于零改动迁移存量 GORM 模型。
// 注意：开关作用于整个 Database 实例，建议在首个 Register 之前调用。
func (db *Database) RegisterSnake(tableName string, object any) error {
	db.ORM.EnableSnakeCase()
	return db.Register(tableName, object)
}

// Close 关闭连接池；FromSQLDB 共享模式下仅解绑引用。
func (db *Database) Close() error {
	if db.Conn == nil {
		return errors.New("opao: 数据库未初始化")
	}
	if db.shared {
		db.Conn = nil
		return nil
	}
	return db.Conn.Close()
}

func (db *Database) GetConn() *sql.DB {
	return db.Conn
}

// Session 返回绑定事务的 Database 视图：其 Load 出的 ObjectORM 操作走 tx。
// 浅拷贝 Database（含嵌入 ORM 值），共享同一 ORM 缓存与 shared 标记，
// 不污染原视图的后续 Load；事务提交/回滚由调用方持有的 tx 控制。
func (db *Database) Session(tx *sql.Tx) (*Database, error) {
	if tx == nil {
		return nil, errors.New("opao: Session 的事务不能为空")
	}
	view := *db
	view.ORM.BindTx(tx)
	return &view, nil
}

// WithContext 返回携带上下文的 Database 视图：此后该视图所有 Load 出的
// 驱动实例与糖层操作透传 ctx（已取消的 ctx 在执行前中断）。
// 浅拷贝实现，不污染原视图。
func (db *Database) WithContext(ctx context.Context) *Database {
	if ctx == nil {
		ctx = context.Background()
	}
	view := *db
	view.ORM.BindCtx(ctx)
	return &view
}

// AllowAll 显式放行无 WHERE 条件的全表写操作哨兵，透传 support.AllowAll()：
// 例如 db.Load(&User{}).Delete(opao.AllowAll()) 执行全表删除。
func AllowAll() any { return support.AllowAll() }

// PrepareStmtConn 返回带预编译缓存的连接视图（高级用法，供手动管理 prepared 生命周期）：
// 调用方持有返回值并负责 Close。cap 取 0 → 默认 LRU 容量 512。
//
// 说明（暂缓边界）：受驱动构造签名限制——Load 出的 ObjectORM 由各驱动包的
// 固定签名构造函数创建、无法统一注入自定义 Execer——ORM 内部全链路透明接线
// 暂缓；当前语句缓存仅在糖层 (*Query).PrepareStmt 与本手动入口可用。
func (db *Database) PrepareStmtConn() (*support.PreparedExec, error) {
	if db.Conn == nil {
		return nil, errors.New("opao: 数据库未初始化")
	}
	return support.NewPreparedExec(db.Conn, 0), nil
}
