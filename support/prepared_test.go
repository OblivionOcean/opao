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
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
)

// ---------- 极简 fake 驱动：计数 Prepare/Close/Exec/Query，纯标准库 ----------

var prepCounters = &fakeCounters{}

// fakeCounters 全局驱动层计数器（互斥保护；并发用例下依然精确）
type fakeCounters struct {
	mu             sync.Mutex
	prepares       int    // conn.Prepare 总次数
	closes         int    // driver.Stmt.Close 总次数
	execs          int    // driver.Stmt.Exec 总次数
	queries        int    // driver.Stmt.Query 总次数
	failPrepareSQL string // 非空时对该 SQL 的 Prepare 一律失败（覆盖 wrapper 与驱动层）
}

func (c *fakeCounters) shouldFail(query string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failPrepareSQL == query
}

func (c *fakeCounters) setFail(query string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failPrepareSQL = query
}

func (c *fakeCounters) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prepares, c.closes, c.execs, c.queries = 0, 0, 0, 0
}

func (c *fakeCounters) snapshot() (prepares, closes, execs, queries int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prepares, c.closes, c.execs, c.queries
}

type prepFakeDriver struct{}

func (prepFakeDriver) Open(string) (driver.Conn, error) { return prepFakeConn{}, nil }

func init() { sql.Register("fake-prepared-opao", prepFakeDriver{}) }

type prepFakeConn struct{}

func (prepFakeConn) Prepare(query string) (driver.Stmt, error) {
	prepCounters.mu.Lock()
	prepCounters.prepares++
	fail := prepCounters.failPrepareSQL == query
	prepCounters.mu.Unlock()
	if fail {
		return nil, errors.New("fake-driver: 注入的驱动层 Prepare 失败")
	}
	return &prepFakeStmt{}, nil
}
func (prepFakeConn) Close() error              { return nil }
func (prepFakeConn) Begin() (driver.Tx, error) { return nil, driver.ErrSkip }

// prepFakeStmt 计数型语句：NumInput 返回 -1 跳过参数个数校验
type prepFakeStmt struct{}

func (*prepFakeStmt) Close() error {
	prepCounters.mu.Lock()
	prepCounters.closes++
	prepCounters.mu.Unlock()
	return nil
}

func (*prepFakeStmt) NumInput() int { return -1 }

func (*prepFakeStmt) Exec([]driver.Value) (driver.Result, error) {
	prepCounters.mu.Lock()
	prepCounters.execs++
	prepCounters.mu.Unlock()
	return prepFakeResult{}, nil
}

func (*prepFakeStmt) Query([]driver.Value) (driver.Rows, error) {
	prepCounters.mu.Lock()
	prepCounters.queries++
	prepCounters.mu.Unlock()
	return &prepFakeRows{}, nil
}

type prepFakeResult struct{}

func (prepFakeResult) LastInsertId() (int64, error) { return 1, nil }
func (prepFakeResult) RowsAffected() (int64, error) { return 1, nil }

// prepFakeRows 最小合法结果集：单列 id，单行 7
type prepFakeRows struct{ pos int }

func (*prepFakeRows) Columns() []string { return []string{"id"} }
func (*prepFakeRows) Close() error      { return nil }
func (r *prepFakeRows) Next(dest []driver.Value) error {
	if r.pos >= 1 {
		return io.EOF
	}
	dest[0] = int64(7)
	r.pos++
	return nil
}

// ---------- fakePreparer：包装 *sql.DB，按 SQL 文本计数 PrepareContext ----------

// fakePreparer 统计每次 PrepareContext（缓存命中不应触达 inner），
// 同时以委托实现 Execer，用于验证 QueryRowContext 的降级直通路径。
type fakePreparer struct {
	db       *sql.DB
	mu       sync.Mutex
	prepares map[string]int
	total    int
}

var (
	_ Preparer = (*fakePreparer)(nil)
	_ Execer   = (*fakePreparer)(nil)
	_ Execer   = (*PreparedExec)(nil)
)

func (f *fakePreparer) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	f.mu.Lock()
	f.total++
	f.prepares[query]++
	f.mu.Unlock()
	if prepCounters.shouldFail(query) {
		return nil, errors.New("fake-preparer: 注入的 Prepare 失败")
	}
	return f.db.PrepareContext(ctx, query)
}

func (f *fakePreparer) count(query string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prepares[query]
}

func (f *fakePreparer) sum() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.total
}

func (f *fakePreparer) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return f.db.ExecContext(ctx, query, args...)
}

func (f *fakePreparer) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return f.db.QueryContext(ctx, query, args...)
}

func (f *fakePreparer) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return f.db.QueryRowContext(ctx, query, args...)
}

// newFakeDB 创建挂接计数驱动的单连接池与配套 Preparer。
// 单连接保证驱动层 stmt 生命周期确定，计数可做精确断言。
func newFakeDB(t *testing.T) (*fakePreparer, *PreparedExec) {
	t.Helper()
	prepCounters.reset()
	conn, err := sql.Open("fake-prepared-opao", "")
	if err != nil {
		t.Fatalf("打开 fake 驱动失败: %v", err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	fp := &fakePreparer{db: conn, prepares: make(map[string]int)}
	return fp, NewPreparedExec(fp, 0)
}

// ---------- 单测 ----------

// inner 为 nil 时构造应 panic
func TestPreparedExecNilInnerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewPreparedExec(nil) 应当 panic")
		}
	}()
	NewPreparedExec(nil, 0)
}

// 相同 SQL 执行两次：Prepare 仅一次，命中路径不触达 inner
func TestPreparedExecSameSQLPreparedOnce(t *testing.T) {
	fp, p := newFakeDB(t)
	ctx := context.Background()
	const ins = "INSERT INTO users(name) VALUES (?)"
	for i := 0; i < 2; i++ {
		if _, err := p.ExecContext(ctx, ins, "张三"); err != nil {
			t.Fatalf("ExecContext 第 %d 次: %v", i+1, err)
		}
	}
	if got := fp.count(ins); got != 1 {
		t.Fatalf("相同 SQL 应只 Prepare 一次, 实际 %d 次", got)
	}
	if _, closes, execs, _ := prepCounters.snapshot(); closes != 0 || execs != 2 {
		t.Fatalf("驱动层计数不符: closes=%d execs=%d (期望 0/2)", closes, execs)
	}

	var id int64
	if err := p.QueryRowContext(ctx, "SELECT id FROM users").Scan(&id); err != nil {
		t.Fatalf("QueryRowContext: %v", err)
	}
	if id != 7 {
		t.Fatalf("结果集应为 7, got %d", id)
	}
	if got := fp.count("SELECT id FROM users"); got != 1 {
		t.Fatalf("查询语句也应只 Prepare 一次, 实际 %d 次", got)
	}
}

// 不同 SQL 各自独立 Prepare；同形 SQL 复用同一份编译结果
func TestPreparedExecDistinctSQL(t *testing.T) {
	fp, p := newFakeDB(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		sql := fmt.Sprintf("SELECT %d", i%2) // 两种 SQL 各复用
		if _, err := p.ExecContext(ctx, sql); err != nil {
			t.Fatalf("ExecContext(%s): %v", sql, err)
		}
	}
	if got := fp.sum(); got != 2 {
		t.Fatalf("两种不同 SQL 共应 Prepare 2 次, 实际 %d 次", got)
	}
	for i := 0; i < 2; i++ {
		if got := fp.count(fmt.Sprintf("SELECT %d", i)); got != 1 {
			t.Fatalf("SQL %q 应恰好 Prepare 1 次, 实际 %d 次", fmt.Sprintf("SELECT %d", i), got)
		}
	}
}

// cap=2 连发 3 种 SQL：最旧者被淘汰并 Close；
// 淘汰后再执行旧 SQL 会重新 Prepare，未淘汰者仍命中缓存。
func TestPreparedExecLRUEvictionClosesOldest(t *testing.T) {
	fp := func() *fakePreparer {
		prepCounters.reset()
		conn, err := sql.Open("fake-prepared-opao", "")
		if err != nil {
			t.Fatal(err)
		}
		conn.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = conn.Close() })
		return &fakePreparer{db: conn, prepares: make(map[string]int)}
	}()
	p := NewPreparedExec(fp, 2)
	ctx := context.Background()

	exec := func(q string) {
		t.Helper()
		if _, err := p.ExecContext(ctx, q); err != nil {
			t.Fatalf("ExecContext(%s): %v", q, err)
		}
	}
	exec("S1")
	exec("S2")
	exec("S3") // 触发淘汰：最旧的 S1 被 Close

	if got := fp.count("S1"); got != 1 {
		t.Fatalf("S1 应 Prepare 1 次, 实际 %d", got)
	}
	if _, closes, _, _ := prepCounters.snapshot(); closes != 1 {
		t.Fatalf("超容量后应有 1 条语句被 Close, 实际 %d", closes)
	}

	exec("S2") // 未被淘汰，仍命中
	if got := fp.count("S2"); got != 1 {
		t.Fatalf("S2 应命中缓存不再 Prepare, 实际 %d 次", got)
	}

	exec("S1") // 已被淘汰，重新 Prepare（同时挤掉 S2）
	if got := fp.count("S1"); got != 2 {
		t.Fatalf("S1 被淘汰后应重新 Prepare, 实际共 %d 次", got)
	}
	if _, closes, _, _ := prepCounters.snapshot(); closes != 2 {
		t.Fatalf("第二次淘汰后累计 Close 应为 2, 实际 %d", closes)
	}
}

// Close 幂等可重入：二次 Close 无 panic、不重复关闭；
// 关闭后的执行视为缓存为空，会重新 Prepare。
func TestPreparedExecCloseIdempotent(t *testing.T) {
	fp, p := newFakeDB(t)
	ctx := context.Background()
	for _, q := range []string{"A1", "A2"} {
		if _, err := p.ExecContext(ctx, q); err != nil {
			t.Fatalf("ExecContext(%s): %v", q, err)
		}
	}
	if err := p.Close(); err != nil {
		t.Fatalf("首次 Close: %v", err)
	}
	if _, closes, _, _ := prepCounters.snapshot(); closes != 2 {
		t.Fatalf("两条缓存语句均应被 Close, 实际 %d", closes)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("二次 Close 不应报错: %v", err)
	}
	if _, closes, _, _ := prepCounters.snapshot(); closes != 2 {
		t.Fatalf("二次 Close 不应重复关闭语句, 实际 %d", closes)
	}

	// 缓存已清空：再次执行重新 Prepare 且功能正常
	if _, err := p.ExecContext(ctx, "A1"); err != nil {
		t.Fatalf("Close 后重新执行: %v", err)
	}
	if got := fp.count("A1"); got != 2 {
		t.Fatalf("Close 后 A1 应重新 Prepare, 实际共 %d 次", got)
	}
}

// 并发 8 goroutine 同查一条 SQL：互斥保证只 Prepare 一次，无 race
func TestPreparedExecConcurrentSameSQL(t *testing.T) {
	fp, p := newFakeDB(t)
	ctx := context.Background()
	const sel = "SELECT id FROM users"

	const goroutines, rounds = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				var id int64
				if err := p.QueryRowContext(ctx, sel).Scan(&id); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("并发执行出错: %v", err)
	}
	if got := fp.count(sel); got != 1 {
		t.Fatalf("并发同查应只 Prepare 一次, 实际 %d 次", got)
	}
	if _, _, _, queries := prepCounters.snapshot(); queries != goroutines*rounds {
		t.Fatalf("查询次数应为 %d, 实际 %d", goroutines*rounds, queries)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("收尾 Close: %v", err)
	}
}

// Prepare 失败时 QueryRowContext 经 inner 的 Execer 直通降级：
// 不 panic，错误经标准 Row 语义在 Scan 时暴露。
func TestPreparedExecQueryRowFallbackOnPrepareError(t *testing.T) {
	_, p := newFakeDB(t)
	ctx := context.Background()
	const bad = "SELECT boom FROM nowhere"
	prepCounters.setFail(bad)
	t.Cleanup(func() { prepCounters.setFail("") })

	row := p.QueryRowContext(ctx, bad)
	var id int64
	if err := row.Scan(&id); err == nil {
		t.Fatal("Prepare 失败的 SQL 经降级直通也应在 Scan 时返回错误")
	}

	// Exec/Query 路径对失败 SQL 直接透传错误而非 panic
	if _, err := p.ExecContext(ctx, bad); err == nil {
		t.Fatal("ExecContext 对 Prepare 失败的 SQL 应回传错误")
	}
	if _, err := p.QueryContext(ctx, bad); err == nil {
		t.Fatal("QueryContext 对 Prepare 失败的 SQL 应回传错误")
	}
}
