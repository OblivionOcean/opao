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

package support

import (
	"container/list"
	"context"
	"database/sql"
	"sync"
)

// Preparer 具备 Prepare 能力的最小接口：*sql.DB 与 *sql.Tx 均满足。
// PreparedExec 借助它直接对底层连接 PrepareContext，绕过 Execer 快捷路径
// 拿不到 *sql.Stmt 的限制。
type Preparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// defaultPreparedCap LRU 默认容量（cap<=0 时启用）
const defaultPreparedCap = 512

// preparedEntry 一条缓存项：SQL 文本与其预编译语句
type preparedEntry struct {
	sql  string
	stmt *sql.Stmt
}

// PreparedExec 包装任意 Preparer，按 SQL 文本 Prepare 并以 LRU 缓存 *sql.Stmt，
// 消除重复语句的服务端 parse/plan 开销。并发安全。
//
// 自身实现 Execer，可直接作为糖层/驱动层的执行器注入：
// 三方法先查缓存，未命中经 inner.PrepareContext 编译后入缓存并按容量淘汰，
// 被逐出的语句立即 Close。命中路径零分配、不触达 inner。
//
// 约束与注意：
//   - inner 为 nil 时 NewPreparedExec panic（非 Preparer 的取值无法通过编译）；
//   - QueryRowContext 在 Prepare 失败且 inner 未实现 Execer 时 panic——
//     标准库不允许合成携带任意错误的 *sql.Row；*sql.DB/*sql.Tx 恒实现
//     Execer，此时退化为直连查询，错误经标准 Row 语义延迟暴露；
//   - 逐出时对仍在使用中的 stmt 调用 Close 属于该简单实现的已知边界，
//     高并发热点语句应保证 cap 覆盖工作集；
//   - Close 之后继续使用会重新 Prepare（缓存视为空）。
type PreparedExec struct {
	mu    sync.Mutex
	inner Preparer
	cap   int

	cache map[string]*list.Element // SQL 文本 → lru 元素
	lru   *list.List               // 队首最新，队尾最旧；元素值为 *preparedEntry
}

// NewPreparedExec 创建带 LRU 容量的预编译执行器。
// inner 为 nil 时 panic；cap<=0 取默认值 512。
func NewPreparedExec(inner Preparer, cap int) *PreparedExec {
	if inner == nil {
		panic("opao/support: NewPreparedExec 的 inner 不能为 nil")
	}
	if cap <= 0 {
		cap = defaultPreparedCap
	}
	return &PreparedExec{
		inner: inner,
		cap:   cap,
		cache: make(map[string]*list.Element, cap),
		lru:   list.New(),
	}
}

// stmt 返回 query 对应的缓存语句，未命中时经 inner 编译并计入 LRU。
// 全程持锁：同一 SQL 并发首查也只触发一次 PrepareContext。
func (p *PreparedExec) stmt(ctx context.Context, query string) (*sql.Stmt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if el, ok := p.cache[query]; ok {
		p.lru.MoveToFront(el)
		return el.Value.(*preparedEntry).stmt, nil
	}
	stmt, err := p.inner.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	p.cache[query] = p.lru.PushFront(&preparedEntry{sql: query, stmt: stmt})
	for p.lru.Len() > p.cap {
		el := p.lru.Back()
		if el == nil {
			break
		}
		entry := el.Value.(*preparedEntry)
		delete(p.cache, entry.sql)
		p.lru.Remove(el)
		_ = entry.stmt.Close() // 逐出即释放服务端句柄；错误无更好去处，静默
	}
	return stmt, nil
}

// ExecContext 经缓存语句执行写操作。
func (p *PreparedExec) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	stmt, err := p.stmt(ctx, query)
	if err != nil {
		return nil, err
	}
	return stmt.ExecContext(ctx, args...)
}

// QueryContext 经缓存语句执行查询。
func (p *PreparedExec) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	stmt, err := p.stmt(ctx, query)
	if err != nil {
		return nil, err
	}
	return stmt.QueryContext(ctx, args...)
}

// QueryRowContext 经缓存语句执行单行查询，返回标准 *sql.Row。
func (p *PreparedExec) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	stmt, err := p.stmt(ctx, query)
	if err != nil {
		// Prepare 失败时标准库无法合成携带任意错误的 *sql.Row：
		// inner 同时是 Execer（*sql.DB/*sql.Tx 恒成立）则直连退化，
		// 错误经标准 Row 语义在 Scan/Err 时暴露；否则只能显式失败。
		if e, ok := p.inner.(Execer); ok {
			return e.QueryRowContext(ctx, query, args...)
		}
		panic("opao/support: PreparedExec 的 inner 未实现 Execer 且 Prepare 失败，无法返回携带错误的 *sql.Row")
	}
	return stmt.QueryRowContext(ctx, args...)
}

// Close 关闭全部缓存语句并清空缓存，返回首个遇到的关闭错误。幂等可重入。
func (p *PreparedExec) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for el := p.lru.Front(); el != nil; {
		entry := el.Value.(*preparedEntry)
		next := el.Next()
		p.lru.Remove(el)
		delete(p.cache, entry.sql)
		if err := entry.stmt.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		el = next
	}
	return firstErr
}
