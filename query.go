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
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unsafe"

	"github.com/OblivionOcean/opao/v2/support"
	"github.com/OblivionOcean/opao/v2/support/mysql"
	"github.com/OblivionOcean/opao/v2/support/pg"
	"github.com/OblivionOcean/opao/v2/support/sqlite"
)

// errSelectNotSupported 是链式糖层的明确功能边界（非缺陷）：
// support.BuildSelect 固定输出全字段列清单，无法安全注入自定义投影，
// 因此 Select 投影暂不支持，调用方应回退到原生 API。
var errSelectNotSupported = errors.New("opao: 糖层暂不支持 Select 投影，请使用 FindAll/原生 API")

// Query GORM 风格链式构建器。每次 Model() 返回全新实例，天然并发安全；
// 这是 opao 的语法糖层，非 GORM 克隆：不支持 Hooks/软删除/关联/Session 克隆语义。
//
// 所有构建方法在遇到问题时记录惰性错误（见 Err），由终结操作统一返回，
// 以保持链式调用的书写流畅性。
type Query struct {
	db      *Database
	table   string
	elems   []support.Elem
	objType reflect.Type
	obj     any // Find 目标对象（Model 传入的指针），用于条件中的字段引用解析
	dialect support.Dialect

	cond    any    // support.Condition 或 nil
	rawSQL  string // 原始条件串（与 cond 二选一）
	rawArgs []any
	order   []string // 已含 ASC/DESC 的原始片段
	limit   any      // 正整数或 nil；渲染为内联数值（防注入）
	offset  any      // 正整数或 nil
	err     error    // 惰性错误，Err() 取出

	ctx  context.Context // Ctx 绑定的上下文；nil 视为 context.Background()
	exec support.Execer  // Tx 绑定的事务执行器；nil 时回退 db.Conn

	prepare  bool                  // PrepareStmt 开启的预编译缓存开关
	prepared *support.PreparedExec // 惰性创建的预编译执行器（首次取执行器时包装一次并复用）
}

// Model 以已注册对象为模型创建全新的查询构建器。
// 对象未注册或驱动名无法映射方言时，错误被记录并由 Err() 与后续操作返回。
func (db *Database) Model(obj any) *Query {
	q := &Query{db: db}
	table, elems, objType, err := db.Snapshot(obj)
	if err != nil {
		q.err = err
		return q
	}
	switch db.sqlDriverName {
	case "mysql":
		q.dialect = mysql.Dialect
	case "postgres", "pg", "pgsql":
		q.dialect = pg.Dialect
	case "sqlite3", "sqlite":
		q.dialect = sqlite.Dialect
	default:
		q.err = fmt.Errorf("opao: 未知的驱动名 %q，无法选择方言", db.sqlDriverName)
		return q
	}
	q.table = table
	q.elems = elems
	q.objType = objType
	q.obj = obj
	return q
}

// Where 追加查询条件：
//   - support.Condition：多次调用以 AND 合并（组合子项自动加括号保证优先级）；
//   - 字符串："col = ?" 形式的原始片段，占位符对应 args，按序透传；
//
// 条件与原始片段混用时，原始片段经 Custom 包装后参与 AND 合并。
// 多余的 args 参数会被忽略（与底层 BuildQuery 的历史行为一致）。
func (q *Query) Where(c any, args ...any) *Query {
	if q.err != nil {
		return q
	}
	var nc support.Condition
	switch v := c.(type) {
	case support.Condition:
		nc = v
	case string:
		nc = Custom(v, args...)
	default:
		q.err = fmt.Errorf("opao: 不支持的查询条件参数类型 %T", c)
		return q
	}
	// 归一化既有状态：raw 片段转为 Custom 条件后统一走 And 合并
	var parts []any
	if q.cond != nil {
		parts = append(parts, q.cond)
	} else if q.rawSQL != "" {
		parts = append(parts, Custom(q.rawSQL, q.rawArgs...))
		q.rawSQL, q.rawArgs = "", nil
	}
	parts = append(parts, nc)
	if len(parts) == 1 {
		q.cond = parts[0]
	} else {
		q.cond = And(parts...)
	}
	return q
}

// Order 追加排序片段："created_at DESC" 原样保留；"-col" 自动转换为 "col DESC"。
// 片段字符集白名单校验（字母、数字、_ . , 空格 -），非法内容记入惰性错误以防注入。
func (q *Query) Order(s string) *Query {
	if q.err != nil {
		return q
	}
	s = strings.TrimSpace(s)
	if s == "" {
		q.err = errors.New("opao: Order 片段不能为空")
		return q
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r == '_' || r == '.' || r == ',' || r == ' ' || r == '-')
	}) >= 0 {
		q.err = fmt.Errorf("opao: Order 片段包含非法字符（仅允许字母、数字、_ . , 空格 -）：%q", s)
		return q
	}
	if strings.HasPrefix(s, "-") {
		col := strings.TrimSpace(strings.TrimPrefix(s, "-"))
		if col == "" {
			q.err = fmt.Errorf("opao: Order 片段缺少列名：%q", s)
			return q
		}
		s = col + " DESC"
	}
	q.order = append(q.order, s)
	return q
}

// Limit 设置返回行数上限。负数记入惰性错误；零值视为未设置（不拼接子句）。
func (q *Query) Limit(n int) *Query {
	if q.err != nil {
		return q
	}
	if n < 0 {
		q.err = fmt.Errorf("opao: Limit 不能为负数：%d", n)
		return q
	}
	if n > 0 {
		q.limit = n
	}
	return q
}

// Offset 设置偏移量。负数记入惰性错误；零值视为未设置。
// 注意：MySQL/SQLite 不支持脱离 LIMIT 的独立 OFFSET，因此未配合 Limit 使用时
// 终结操作会返回错误，而不是生成跨方言不可执行的 SQL。
func (q *Query) Offset(n int) *Query {
	if q.err != nil {
		return q
	}
	if n < 0 {
		q.err = fmt.Errorf("opao: Offset 不能为负数：%d", n)
		return q
	}
	if n > 0 {
		q.offset = n
	}
	return q
}

// Select 在链式糖层不受支持：记录惰性错误，由 Err() 与任一终结操作返回。
// 这是明确的功能边界而非缺陷，投影需求请回退原生 API。
func (q *Query) Select(cols ...string) *Query {
	if q.err == nil {
		q.err = errSelectNotSupported
	}
	return q
}

// Err 返回链式调用过程中累积的惰性错误（无错时为 nil）。
func (q *Query) Err() error {
	return q.err
}

// Ctx 为该查询绑定上下文：终结操作透传给底层驱动，
// 已取消的 ctx 在执行前中断并返回 context 错误。
func (q *Query) Ctx(ctx context.Context) *Query {
	if ctx == nil {
		ctx = context.Background()
	}
	q.ctx = ctx
	return q
}

// Tx 将该查询的所有读写绑定到指定事务执行；
// 提交/回滚由调用方持有的 tx 控制。nil 事务记入惰性错误。
func (q *Query) Tx(tx *sql.Tx) *Query {
	if tx == nil {
		if q.err == nil {
			q.err = errors.New("opao: Tx 的事务不能为空")
		}
		return q
	}
	q.exec = tx
	return q
}

// PrepareStmt 为本查询启用预编译语句缓存（重复执行同形 SQL 时消除服务端 parse 开销）。
// 与 .Tx 组合时包装事务对象；终结操作首次取执行器时惰性创建缓存并在本构建器内复用。
// 仅当生效执行器支持 PrepareContext（*sql.DB/*sql.Tx 恒满足）时启用，否则原样直通。
func (q *Query) PrepareStmt() *Query {
	q.prepare = true
	return q
}

// session 解析糖层生效的上下文与执行器：显式 Tx/Ctx 覆盖优先，
// 否则回退 Database 连接池 + Background。
// PrepareStmt 开启时，把底层执行器（连接池或事务）惰性包装为 LRU 预编译执行器：
// 同一构建器多次终结操作共享缓存，命中路径不重复 Prepare。
func (q *Query) session() (context.Context, support.Execer) {
	ctx := q.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	e := q.exec
	if e == nil {
		e = q.db.Conn
	}
	if q.prepare && e != nil {
		if pr, ok := e.(support.Preparer); ok {
			if q.prepared == nil {
				q.prepared = support.NewPreparedExec(pr, 0)
			}
			return ctx, q.prepared
		}
	}
	return ctx, e
}

// ready 校验构建器已经过 Model() 初始化且连接可用。
func (q *Query) ready() error {
	if q.db == nil || q.db.Conn == nil {
		return errors.New("opao: Query 必须通过 Database.Model() 创建")
	}
	return nil
}

// boundDriver 获取绑定当前会话的驱动实例（糖层委托入口）。
func (q *Query) boundDriver() (support.ObjectORM, error) {
	o := q.db.Load(q.obj)
	if err := o.Error(); err != nil {
		return nil, err
	}
	ctx, e := q.session()
	if b, ok := o.(interface {
		BindExec(support.Execer, context.Context)
	}); ok {
		b.BindExec(e, ctx)
	}
	return o, nil
}

// whereParts 组装传给 support 构建器的可变参数：
// 无条件 → 空；Condition → 单元素；原始片段 → [片段]+参数（保持占位符顺序）。
func (q *Query) whereParts() []any {
	switch {
	case q.cond != nil:
		return []any{q.cond}
	case q.rawSQL != "":
		parts := make([]any, 0, len(q.rawArgs)+1)
		parts = append(parts, q.rawSQL)
		parts = append(parts, q.rawArgs...)
		return parts
	default:
		return nil
	}
}

// conditionOrRaw 把内部条件状态归一化为单个 Condition，
// 供 RenderCondition 直接消费（原始片段经 Custom 包装）。
func (q *Query) conditionOrRaw() support.Condition {
	if q.cond != nil {
		return q.cond.(support.Condition)
	}
	return Custom(q.rawSQL, q.rawArgs...)
}

// hasCondition 报告是否设置了任何过滤条件。
func (q *Query) hasCondition() bool {
	return q.cond != nil || q.rawSQL != ""
}

// effectiveLimitOffset 解析生效的 LIMIT/OFFSET 数值。
// extraLimit 由 First 注入并优先于用户设置；OFFSET 缺少 LIMIT 时报错以保证跨方言可执行。
func (q *Query) effectiveLimitOffset(extraLimit int) (limit, offset int, err error) {
	if v, ok := q.limit.(int); ok && v > 0 {
		limit = v
	}
	if extraLimit > 0 {
		limit = extraLimit
	}
	if v, ok := q.offset.(int); ok && v > 0 {
		offset = v
		if limit == 0 {
			return 0, 0, errors.New("opao: Offset 需要与 Limit 配合使用")
		}
	}
	return limit, offset, nil
}

// selectSQL 生成完整 SELECT 语句：
// WHERE 部分复用 support.BuildSelect（含方言重绑定），ORDER BY / LIMIT / OFFSET
// 由本层追加——排序片段经字符集校验、分页为内联正整数，均无注入面。
func (q *Query) selectSQL(extraLimit int) (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	if err := q.ready(); err != nil {
		return "", nil, err
	}
	limit, offset, err := q.effectiveLimitOffset(extraLimit)
	if err != nil {
		return "", nil, err
	}
	tail := ""
	if limit > 0 || len(q.order) > 0 {
		order := strings.Join(q.order, ", ")
		tail = support.CachedTail(order, limit, offset)
	}
	if q.err != nil {
		return "", nil, q.err
	}
	var cond support.Condition
	rawSQL, rawArgs := "", []any(nil)
	switch {
	case q.cond != nil:
		cond = q.cond.(support.Condition)
	case q.rawSQL != "":
		rawSQL = q.rawSQL
		rawArgs = q.rawArgs
	}
	return support.ComposeStatement(q.dialect, 0, q.table, q.elems, q.obj, q.objType,
		cond, rawSQL, rawArgs, tail)
}

// repointedElems 复制一份元素并把 Ptr 重定位到 target 结构体实例的字段上，
// 使 ScanOne 扫描与自增主键回填作用于调用方传入的对象（Snapshot 返回的元数据不含有效指针）。
func (q *Query) repointedElems(target reflect.Value) []support.Elem {
	base := target.Addr().UnsafePointer()
	elems := make([]support.Elem, len(q.elems))
	copy(elems, q.elems)
	for i := range elems {
		elems[i].Ptr = unsafe.Add(base, elems[i].Offset)
	}
	return elems
}

// derefTarget 校验 dest 为非空指针并返回其指向的值（具体 Kind 由调用方校验）。
func derefTarget(dest any, what string) (reflect.Value, error) {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return reflect.Value{}, fmt.Errorf("opao: %s 的目标必须是非空指针", what)
	}
	return rv.Elem(), nil
}

// Find 执行查询并把结果填充到 dest（必须是指向模型切片的指针）。
// 无条件时查询全表。
func (q *Query) Find(dest any) error {
	sv, err := derefTarget(dest, "Find")
	if err != nil {
		return err
	}
	var objs []any
	if !q.hasTailParts() && len(q.order) == 0 {
		o, derr := q.boundDriver()
		if derr != nil {
			return derr
		}
		if objs, err = o.FindAll(q.whereParts()...); err != nil {
			return err
		}
	} else {
		query, args, serr := q.selectSQL(0)
		if serr != nil {
			return serr
		}
		ctx, e := q.session()
		if objs, err = support.ScanAllCtx(ctx, e, query, args, q.objType, q.elems); err != nil {
			return err
		}
	}
	out := reflect.MakeSlice(sv.Type(), 0, len(objs))
	for _, o := range objs {
		out = reflect.Append(out, reflect.ValueOf(o))
	}
	sv.Set(out)
	return nil
}

// FindAll 执行查询并返回对象值切片，适合不需要强类型接收方的场景。
func (q *Query) FindAll() ([]any, error) {
	if q.err != nil {
		return nil, q.err
	}
	if !q.hasTailParts() {
		o, derr := q.boundDriver()
		if derr != nil {
			return nil, derr
		}
		return o.FindAll(q.whereParts()...)
	}
	query, args, err := q.selectSQL(0)
	if err != nil {
		return nil, err
	}
	ctx, e := q.session()
	return support.ScanAllCtx(ctx, e, query, args, q.objType, q.elems)
}

// hasTailParts 是否存在排序/分页约束。
func (q *Query) hasTailParts() bool {
	li, _ := q.limit.(int)
	of, _ := q.offset.(int)
	return len(q.order) > 0 || li > 0 || of > 0
}

// First 执行查询并把第一行扫描进 dest（必须是与模型同类型的结构体指针）。
// 强制 LIMIT 1（优先于用户设置的 Limit）；未命中时透传 sql.ErrNoRows。
func (q *Query) First(dest any) error {
	query, args, err := q.selectSQL(1)
	if err != nil {
		return err
	}
	sv, err := derefTarget(dest, "First")
	if err != nil {
		return err
	}
	if sv.Kind() != reflect.Struct || sv.Type() != q.objType {
		return fmt.Errorf("opao: First 的目标类型 %v 与模型类型 %v 不符", sv.Type(), q.objType)
	}
	ctx, e := q.session()
	return support.ScanOneCtx(ctx, e, query, args, q.repointedElems(sv))
}

// Count 统计满足条件的行数。COUNT 语义下忽略 Order/Limit/Offset。
func (q *Query) Count() (int, error) {
	if q.err != nil {
		return 0, q.err
	}
	if err := q.ready(); err != nil {
		return 0, err
	}
	var cond support.Condition
	rawSQL, rawArgs := "", []any(nil)
	switch {
	case q.cond != nil:
		cond = q.cond.(support.Condition)
	case q.rawSQL != "":
		rawSQL = q.rawSQL
		rawArgs = q.rawArgs
	}
	query, args, err := support.ComposeStatement(q.dialect, 1, q.table, q.elems, q.obj, q.objType,
		cond, rawSQL, rawArgs, "")
	if err != nil {
		return 0, err
	}
	ctx, e := q.session()
	return support.ScanCountCtx(ctx, e, query, args)
}

// Delete 删除满足条件的行。无条件时拒绝执行并返回 support.ErrMissingWhereClause。
func (q *Query) Delete() error {
	if q.err != nil {
		return q.err
	}
	if err := q.ready(); err != nil {
		return err
	}
	if !q.hasCondition() {
		return support.ErrMissingWhereClause
	}
	var cond support.Condition
	rawSQL, rawArgs := "", []any(nil)
	switch {
	case q.cond != nil:
		cond = q.cond.(support.Condition)
	case q.rawSQL != "":
		rawSQL = q.rawSQL
		rawArgs = q.rawArgs
	}
	query, args, err := support.ComposeStatement(q.dialect, 2, q.table, q.elems, q.obj, q.objType,
		cond, rawSQL, rawArgs, "")
	if err != nil {
		return err
	}
	ctx, e := q.session()
	return support.ExecCtx(ctx, e, query, args, q.elems, false)
}

// Updates 按显式列映射执行部分更新，绕过结构体零值语义。
// 列名经方言 Quote 引用；键排序后生成 SET 子句以保证语句确定性；
// 占位符先统一为 "?" 再由 Rebind 做方言转换。无条件时拒绝执行。
func (q *Query) Updates(cols map[string]any) error {
	if q.err != nil {
		return q.err
	}
	if err := q.ready(); err != nil {
		return err
	}
	if !q.hasCondition() {
		return support.ErrMissingWhereClause
	}
	if len(cols) == 0 {
		return fmt.Errorf("%w", support.ErrNoFieldsToUpdate)
	}
	keys := make([]string, 0, len(cols))
	for k := range cols {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	whereSQL, whereArgs, err := support.RenderCondition(q.elems, q.obj, q.objType, q.conditionOrRaw())
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(q.dialect.Quote(q.table))
	sb.WriteString(" SET ")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(q.dialect.Quote(k))
		sb.WriteString(" = ?")
	}
	if whereSQL != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(whereSQL)
	}
	args := make([]any, 0, len(keys)+len(whereArgs))
	for _, k := range keys {
		args = append(args, cols[k])
	}
	args = append(args, whereArgs...)
	ctx, e := q.session()
	return support.ExecCtx(ctx, e, support.Rebind(q.dialect, sb.String()), args, q.elems, false)
}

// Create 插入传入的对象（快照取自该对象本身，可与 Model 的模型不同实例）。
// 自增主键自动跳过写入并在插入成功后回填到对象字段。
func (q *Query) Create(obj any) error {
	if q.err != nil {
		return q.err
	}
	if err := q.ready(); err != nil {
		return err
	}
	o := q.db.Load(obj)
	if e := o.Error(); e != nil {
		return e
	}
	ctx, ex := q.session()
	if b, ok := o.(interface {
		BindExec(support.Execer, context.Context)
	}); ok {
		b.BindExec(ex, ctx)
	}
	return o.Create()
}
