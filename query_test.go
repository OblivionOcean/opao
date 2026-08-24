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
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/sqlite"
)

// ---------- 极简 stub 驱动：捕获 SQL 与参数，避免引入任何外部测试依赖 ----------

type queryUser struct {
	ID   int64  `db:"id" option:"autoIncrement"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

// stubCall 记录一次被捕获的数据库调用
type stubCall struct {
	op   string // "query" 或 "exec"
	sql  string
	args []any
}

var (
	stubMu    sync.Mutex
	stubCalls []stubCall
	stubNextQ *stubRows // 下一次 Query 消费的一次性结果集（可为 nil）
)

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) { return stubConn{}, nil }

func init() {
	sql.Register("stub-opao", stubDriver{})
}

type stubConn struct{}

func (stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("stub-opao: 未实现 Prepare，应走 Queryer/Execer 快捷路径")
}
func (stubConn) Close() error              { return nil }
func (stubConn) Begin() (driver.Tx, error) { return nil, driver.ErrSkip }

func (stubConn) QueryContext(_ context.Context, query string, nargs []driver.NamedValue) (driver.Rows, error) {
	stubMu.Lock()
	rows := stubNextQ
	stubNextQ = nil
	stubCalls = append(stubCalls, stubCall{op: "query", sql: query, args: namedValues(nargs)})
	stubMu.Unlock()
	if rows != nil {
		return rows.clone(), nil
	}
	return &stubRows{cols: []string{"placeholder"}}, nil
}

func (stubConn) ExecContext(_ context.Context, query string, nargs []driver.NamedValue) (driver.Result, error) {
	stubMu.Lock()
	stubCalls = append(stubCalls, stubCall{op: "exec", sql: query, args: namedValues(nargs)})
	stubMu.Unlock()
	return stubResult{}, nil
}

// stubResult 固定返回自增主键 1 与影响行数 1
type stubResult struct{}

func (stubResult) LastInsertId() (int64, error) { return 1, nil }
func (stubResult) RowsAffected() (int64, error) { return 1, nil }

// stubRows 可配置的内存结果集
type stubRows struct {
	cols []string
	data [][]driver.Value
	pos  int
}

func (r *stubRows) Columns() []string { return r.cols }
func (r *stubRows) Close() error      { return nil }

func (r *stubRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}

// clone 深拷贝，防止多次消费同一份配置时数据被迭代器破坏
func (r *stubRows) clone() *stubRows {
	data := make([][]driver.Value, len(r.data))
	for i := range r.data {
		data[i] = append([]driver.Value(nil), r.data[i]...)
	}
	return &stubRows{cols: append([]string(nil), r.cols...), data: data}
}

func namedValues(nargs []driver.NamedValue) []any {
	vals := make([]any, len(nargs))
	for i, a := range nargs {
		vals[i] = a.Value
	}
	return vals
}

// stubSetRows 配置下一次 Query 返回的结果集（一次性）
func stubSetRows(cols []string, data [][]driver.Value) {
	stubMu.Lock()
	defer stubMu.Unlock()
	stubNextQ = &stubRows{cols: cols, data: data}
}

func lastStubCall(t *testing.T) stubCall {
	t.Helper()
	stubMu.Lock()
	defer stubMu.Unlock()
	if len(stubCalls) == 0 {
		t.Fatal("没有捕获到任何数据库调用")
	}
	return stubCalls[len(stubCalls)-1]
}

func stubCallCount(t *testing.T) int {
	t.Helper()
	stubMu.Lock()
	defer stubMu.Unlock()
	return len(stubCalls)
}

func assertArgs(t *testing.T, got []any, want ...any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("绑定参数不符:\n got=%v\nwant=%v", got, want)
	}
}

// newQueryDB 创建挂接 stub 驱动的 Database 并注册测试模型。
// 不能走 NewDatabase：它只接受三种真实驱动名；这里手动装配以复用方言映射逻辑。
func newQueryDB(t *testing.T) *Database {
	t.Helper()
	stubMu.Lock()
	stubCalls = nil
	stubNextQ = nil
	stubMu.Unlock()
	conn, err := sql.Open("stub-opao", "")
	if err != nil {
		t.Fatalf("打开 stub 驱动失败: %v", err)
	}
	db := &Database{Conn: conn, sqlDriverName: "sqlite3"}
	db.ORM.Init(conn, sqlite.New) // 真实驱动：糖层委托路径与 SQL 断言共同生效
	if err := db.Register("users", &queryUser{}); err != nil {
		t.Fatalf("注册模型失败: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return db
}

const userCols = `"id","name","age"`

// ---------- 单测 ----------

// 1. Condition + Order("-id") + Limit(10)：SQL 含 ORDER BY id DESC 与内联 LIMIT 10
func TestQueryConditionOrderLimitSQL(t *testing.T) {
	db := newQueryDB(t)
	q := db.Model(&queryUser{}).Where(Eq("name", "张三")).Order("-id").Limit(10)
	if err := q.Err(); err != nil {
		t.Fatalf("链式构建出现惰性错误: %v", err)
	}
	if _, err := q.FindAll(); err != nil {
		t.Fatalf("FindAll 失败: %v", err)
	}
	want := "SELECT " + userCols + " FROM \"users\" WHERE name = ? ORDER BY id DESC LIMIT 10"
	if got := lastStubCall(t).sql; got != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", got, want)
	}
	assertArgs(t, lastStubCall(t).args, "张三")
}

// 2. 原始条件串与参数按序透传
func TestQueryWhereRawArgsPassthrough(t *testing.T) {
	db := newQueryDB(t)
	if _, err := db.Model(&queryUser{}).Where("name = ?", "x").FindAll(); err != nil {
		t.Fatalf("FindAll 失败: %v", err)
	}
	call := lastStubCall(t)
	if want := "SELECT " + userCols + " FROM \"users\" WHERE name = ?"; call.sql != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", call.sql, want)
	}
	assertArgs(t, call.args, "x")
}

// 3. Delete 无条件 → ErrMissingWhereClause，且不产生任何执行
func TestQueryDeleteWithoutWhereRejected(t *testing.T) {
	db := newQueryDB(t)
	err := db.Model(&queryUser{}).Delete()
	if !errors.Is(err, support.ErrMissingWhereClause) {
		t.Fatalf("期望 ErrMissingWhereClause, got %v", err)
	}
	if n := stubCallCount(t); n != 0 {
		t.Fatalf("无条件 Delete 不应触达数据库, 实际捕获 %d 次调用", n)
	}
}

// 4. Updates map：SET 列经 Quote 引用且按键排序保证语句确定，WHERE 条件保留
func TestQueryUpdatesMapSQL(t *testing.T) {
	db := newQueryDB(t)
	if err := db.Model(&queryUser{}).Where(Eq("id", 7)).
		Updates(map[string]any{"b": "vb", "a": 1}); err != nil {
		t.Fatalf("Updates 失败: %v", err)
	}
	call := lastStubCall(t)
	if call.op != "exec" {
		t.Fatalf("Updates 应走 exec 路径, got %s", call.op)
	}
	want := "UPDATE \"users\" SET \"a\" = ?, \"b\" = ? WHERE id = ?"
	if call.sql != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", call.sql, want)
	}
	assertArgs(t, call.args, int64(1), "vb", int64(7))
}

// 5. 未注册模型的 Model → 惰性错误可由 Err() 取出
func TestQueryModelUnregistered(t *testing.T) {
	db := newQueryDB(t)
	q := db.Model(&struct {
		X int `db:"x"`
	}{})
	if q.Err() == nil {
		t.Fatal("未注册对象应当记录错误")
	}
}

// 6. 多次 Where 合并为 AND；混合 Condition 与原始片段时组合子项自动加括号
func TestQueryMultiWhereMergedWithAnd(t *testing.T) {
	db := newQueryDB(t)
	q := db.Model(&queryUser{}).
		Where(Gte("age", 18)).
		Where(Lt("age", 60)).
		Where("name <> ?", "")
	if _, err := q.FindAll(); err != nil {
		t.Fatalf("FindAll 失败: %v", err)
	}
	call := lastStubCall(t)
	want := "SELECT " + userCols + " FROM \"users\" WHERE (age >= ? AND age < ?) AND name <> ?"
	if call.sql != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", call.sql, want)
	}
	assertArgs(t, call.args, int64(18), int64(60), "")
}

// Select 投影是明确的功能边界：记录惰性错误并由终结操作返回
func TestQuerySelectProjectionUnsupported(t *testing.T) {
	db := newQueryDB(t)
	q := db.Model(&queryUser{}).Select("id")
	if q.Err() == nil || !strings.Contains(q.Err().Error(), "Select") {
		t.Fatalf("Select 应记录不支持错误, got %v", q.Err())
	}
	if _, err := q.FindAll(); !errors.Is(err, errSelectNotSupported) {
		t.Fatalf("FindAll 应透传投影错误, got %v", err)
	}
	if n := stubCallCount(t); n != 0 {
		t.Fatalf("带错误的查询不应触达数据库, 实际捕获 %d 次", n)
	}
}

// Offset 缺少 Limit 时跨方言不可执行，应报错而非静默生成坏 SQL
func TestQueryOffsetRequiresLimit(t *testing.T) {
	db := newQueryDB(t)
	if _, err := db.Model(&queryUser{}).Offset(5).FindAll(); err == nil {
		t.Fatal("无 Limit 的 Offset 应当报错")
	}
}

// Order 片段字符集校验拦截注入尝试
func TestQueryOrderRejectsInjection(t *testing.T) {
	db := newQueryDB(t)
	q := db.Model(&queryUser{}).Order("id; DROP TABLE users")
	if q.Err() == nil {
		t.Fatal("含非法字符的排序片段应当报错")
	}
}

// First 强制 LIMIT 1 并把行扫描进目标对象
func TestQueryFirstScansRow(t *testing.T) {
	db := newQueryDB(t)
	stubSetRows([]string{"id", "name", "age"}, [][]driver.Value{{int64(2), "李四", int64(25)}})
	var u queryUser
	if err := db.Model(&queryUser{}).Where(Eq("id", 2)).First(&u); err != nil {
		t.Fatalf("First 失败: %v", err)
	}
	call := lastStubCall(t)
	want := "SELECT " + userCols + " FROM \"users\" WHERE id = ? LIMIT 1"
	if call.sql != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", call.sql, want)
	}
	if u.ID != 2 || u.Name != "李四" || u.Age != 25 {
		t.Fatalf("扫描结果不符: %+v", u)
	}
}

// Create 插入跳过自增列，并把 LastInsertId 回填到对象字段
func TestQueryCreateBackfillsID(t *testing.T) {
	db := newQueryDB(t)
	u := queryUser{Name: "王五", Age: 30}
	if err := db.Model(&queryUser{}).Create(&u); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	call := lastStubCall(t)
	if call.op != "exec" {
		t.Fatalf("Create 应走 exec 路径, got %s", call.op)
	}
	want := "INSERT INTO \"users\" (\"name\",\"age\") VALUES (?,?)"
	if call.sql != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", call.sql, want)
	}
	assertArgs(t, call.args, "王五", int64(30))
	if u.ID != 1 {
		t.Fatalf("自增主键未回填: got %d want 1", u.ID)
	}
}

// Count 带 WHERE 条件并忽略排序分页语义
func TestQueryCountWithCondition(t *testing.T) {
	db := newQueryDB(t)
	stubSetRows([]string{"count"}, [][]driver.Value{{int64(42)}})
	n, err := db.Model(&queryUser{}).Where(Gte("age", 18)).Order("-id").Limit(5).Count()
	if err != nil {
		t.Fatalf("Count 失败: %v", err)
	}
	if n != 42 {
		t.Fatalf("计数不符: got %d want 42", n)
	}
	want := "SELECT COUNT(*) FROM \"users\" WHERE age >= ?"
	if got := lastStubCall(t).sql; got != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", got, want)
	}
}

// Find 把多行结果填充进切片指针
func TestQueryFindFillsSlice(t *testing.T) {
	db := newQueryDB(t)
	stubSetRows([]string{"id", "name", "age"}, [][]driver.Value{
		{int64(1), "张三", int64(20)},
		{int64(2), "李四", int64(30)},
	})
	users := []queryUser{}
	if err := db.Model(&queryUser{}).Find(&users); err != nil {
		t.Fatalf("Find 失败: %v", err)
	}
	if len(users) != 2 || users[1].Name != "李四" {
		t.Fatalf("填充结果不符: %+v", users)
	}
}

// Where 参数类型不受支持时记入惰性错误
func TestQueryWhereUnsupportedType(t *testing.T) {
	db := newQueryDB(t)
	q := db.Model(&queryUser{}).Where(123)
	if q.Err() == nil {
		t.Fatal("不支持的参数类型应当报错")
	}
}

// 无条件查询生成不带 WHERE 的全表语句
func TestQueryFindAllWithoutCondition(t *testing.T) {
	db := newQueryDB(t)
	if _, err := db.Model(&queryUser{}).FindAll(); err != nil {
		t.Fatalf("FindAll 失败: %v", err)
	}
	want := "SELECT " + userCols + " FROM \"users\""
	if got := lastStubCall(t).sql; got != want {
		t.Fatalf("SQL 不符:\n got=%s\nwant=%s", got, want)
	}
}
