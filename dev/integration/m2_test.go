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

package integration

import (
	"context"
	"errors"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/OblivionOcean/opao/v2"
	"github.com/OblivionOcean/opao/v2/support"
	"github.com/OblivionOcean/opao/v2/support/sqlite"
)

// newM2DB 创建 M2 场景专用的 sqlite 内存库（每用例独立 DSN 隔离数据）
func newM2DB(t *testing.T, name string) *opao.Database {
	t.Helper()
	db, err := opao.NewDatabase("sqlite", "file:"+name+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.Conn.SetMaxOpenConns(1) // 内存库多连接不共享数据；单连接保证与 tx 串行
	if _, err := db.Conn.Exec(`CREATE TABLE users (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT    NOT NULL DEFAULT '',
		age  INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Register("users", &User{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestTransactionRollback Session 视图的写操作随事务回滚/提交：
// 回滚路径计数不变，提交路径计数+1；原视图不受 Session 污染。
func TestTransactionRollback(t *testing.T) {
	db := newM2DB(t, "opao_dev_m2_tx")

	tx, err := db.Conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	sess, err := db.Session(tx)
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Load(&User{Name: "rollback"}).Create(); err != nil {
		t.Fatalf("tx 内 Create: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	n, err := db.Load(&User{}).Count()
	if err != nil || n != 0 {
		t.Fatalf("回滚后计数应为 0: %v n=%d", err, n)
	}

	tx2, err := db.Conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	sess2, err := db.Session(tx2)
	if err != nil {
		t.Fatal(err)
	}
	if err := sess2.Load(&User{Name: "commit"}).Create(); err != nil {
		t.Fatalf("tx 内 Create: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatal(err)
	}
	n2, err := db.Load(&User{}).Count()
	if err != nil || n2 != 1 {
		t.Fatalf("提交后计数应为 1: %v n=%d", err, n2)
	}
}

// TestWithContextCancel 已取消的 ctx 应在执行前中断，
// 驱动层（WithContext→Load）与糖层（Ctx）均返回 context 错误。
func TestWithContextCancel(t *testing.T) {
	db := newM2DB(t, "opao_dev_m2_ctx")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	finder := db.WithContext(ctx).Load(&User{})
	if _, err := finder.FindAll(); !errors.Is(err, context.Canceled) {
		t.Fatalf("驱动层期望 context.Canceled，得到 %v", err)
	}

	var out []User
	if err := db.Model(&User{}).Ctx(ctx).Find(&out); !errors.Is(err, context.Canceled) {
		t.Fatalf("糖层期望 context.Canceled，得到 %v", err)
	}
}

// TestAllowFullTableDelete 无条件 Delete 拒绝执行；
// 显式 AllowAll() 哨兵放行全表删除。
func TestAllowFullTableDelete(t *testing.T) {
	db := newM2DB(t, "opao_dev_m2_all")
	for _, u := range []*User{{Name: "a"}, {Name: "b"}} {
		if err := db.Load(u).Create(); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Load(&User{}).Delete(); !errors.Is(err, support.ErrMissingWhereClause) {
		t.Fatalf("无条件 Delete 应被拒绝，得到 %v", err)
	}
	if err := db.Load(&User{}).Delete(opao.AllowAll()); err != nil {
		t.Fatalf("AllowAll 全表删除应放行，得到 %v", err)
	}
	n, err := db.Load(&User{}).Count()
	if err != nil || n != 0 {
		t.Fatalf("全表删除后计数应为 0: %v n=%d", err, n)
	}
}

// TestSugarTx 糖层 Query.Tx(tx) 的写操作随事务回滚。
func TestSugarTx(t *testing.T) {
	db := newM2DB(t, "opao_dev_m2_sugar")

	tx, err := db.Conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	u := &User{Name: "sugar"}
	if err := db.Model(&User{}).Tx(tx).Create(u); err != nil {
		t.Fatalf("糖层 tx 内 Create: %v", err)
	}
	n, err := db.Model(&User{}).Tx(tx).Count()
	if err != nil || n != 1 {
		t.Fatalf("事务内计数应为 1: %v n=%d", err, n)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	n2, err := db.Model(&User{}).Count()
	if err != nil || n2 != 0 {
		t.Fatalf("回滚后计数应为 0: %v n=%d", err, n2)
	}
}

// TestDriverWithTxCtx 驱动具体类型的 WithTx/WithCtx 副本方法：
// 共享元数据、独立执行器；WithCtx 透传已取消 ctx。
func TestDriverWithTxCtx(t *testing.T) {
	db := newM2DB(t, "opao_dev_m2_drv")

	tx, err := db.Conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	o := db.Load(&User{Name: "drv"})
	if err := o.Error(); err != nil {
		t.Fatal(err)
	}
	drv, ok := o.(*sqlite.Sqlite)
	if !ok {
		t.Fatalf("驱动具体类型断言失败: %T", o)
	}
	if err := drv.WithTx(tx).Create(); err != nil {
		t.Fatalf("WithTx Create: %v", err)
	}
	n, err := db.Model(&User{}).Tx(tx).Count()
	if err != nil || n != 1 {
		t.Fatalf("事务内计数应为 1: %v n=%d", err, n)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	drv2, ok := db.Load(&User{}).(*sqlite.Sqlite)
	if !ok {
		t.Fatal("驱动具体类型断言失败")
	}
	if _, err := drv2.WithCtx(ctx).FindAll(); !errors.Is(err, context.Canceled) {
		t.Fatalf("WithCtx 期望 context.Canceled，得到 %v", err)
	}
}
