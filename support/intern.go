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
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

// ============================================================
// L1：语句内联缓存（注册期形态的确定性语句集，无条件路径）
// ============================================================

// CompiledStmts 注册期形态的确定性语句集（无条件路径）。
// 三条语句只依赖（方言, 表名, 列元数据），与具体对象值无关，
// 因此可在首次使用时构建一次并全局复用。
type CompiledStmts struct {
	Insert    string // INSERT INTO ... (...) VALUES (?,...)
	SelectAll string // SELECT ... FROM ...（无 WHERE）
	CountAll  string // SELECT COUNT(*) FROM ...
}

// stmtInternKey L1 缓存键：方言注册名 + 表名 + 列元数据指纹。
// 指纹用于区分同一进程内注册到同名表但列集不同的多个模型
// （存量测试套件中即存在此形态），避免跨模型取到错误语句。
type stmtInternKey struct {
	dname string
	table string
	fp    uint64
}

// elemsFingerprint 对列元数据（标签、类型、自增标记）做 FNV-1a 指纹。
func elemsFingerprint(elems []Elem) uint64 {
	w := newCondHasher()
	for i := range elems {
		e := &elems[i]
		w.writeString(e.Tag)
		if e.Type != nil {
			w.writeString(e.Type.String())
		} else {
			w.writeString("?")
		}
		if e.AutoInc || e.Option["autoIncrement"] == "-" {
			w.writeByte('a')
		} else {
			w.writeByte('.')
		}
	}
	return w.h
}

// stmtInternCache L1 全局内联缓存。键为 (方言名, 表名, 元数据指纹)，并发安全；
// 模型×方言组合有限不需淘汰。同一键并发构建时 LoadOrStore 保证只保留一份。
var stmtInternCache sync.Map

// InternStmts 返回 (方言, 表) 对应的内联语句集，首次调用时用现有导出
// Builder 以 nil obj 构建一次后缓存。全部列均为自增等不可写场景下
// Insert 为空串——调用方应先经 InsertValues 空判定再取用。
func InternStmts(d Dialect, table string, elems []Elem) *CompiledStmts {
	return internStmtsAt(d, d.Name(), table, elemsFingerprint(elems), elems)
}

func internStmtsAt(d Dialect, dname, table string, fp uint64, elems []Elem) *CompiledStmts {
	key := stmtInternKey{dname: dname, table: table, fp: fp}
	if v, ok := stmtInternCache.Load(key); ok {
		return v.(*CompiledStmts)
	}
	ss := &CompiledStmts{}
	if q, _, err := BuildInsert(d, table, elems); err == nil {
		ss.Insert = q
	}
	if q, _, err := BuildSelect(d, table, elems, nil, nil); err == nil {
		ss.SelectAll = q
	}
	if q, _, err := BuildCount(d, table, elems, nil, nil); err == nil {
		ss.CountAll = q
	}
	actual, _ := stmtInternCache.LoadOrStore(key, ss)
	return actual.(*CompiledStmts)
}

// InsertValues 按插入列顺序收集绑定值（跳过 autoIncrement=="-" 字段），
// 顺序必须与 CompiledStmts.Insert 的列清单一致；无可写列时返回空切片。
func InsertValues(elems []Elem) []any {
	values := make([]any, 0, len(elems))
	for i := range elems {
		e := &elems[i]
		if e.AutoInc || e.Option["autoIncrement"] == "-" {
			continue
		}
		values = append(values, e.Get())
	}
	return values
}

// ============================================================
// L2：条件结构哈希与条件 SQL 文本缓存
// ============================================================

// FNV-1a 常数
const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
)

// condHasher FNV-1a 字节流累积器。
type condHasher struct{ h uint64 }

func newCondHasher() condHasher { return condHasher{h: fnvOffset64} }

func (w *condHasher) writeByte(b byte) {
	w.h ^= uint64(b)
	w.h *= fnvPrime64
}

// writeUint 固定 8 字节小端写入，避免变长编码的边界歧义。
func (w *condHasher) writeUint(v uint64) {
	for i := 0; i < 8; i++ {
		w.writeByte(byte(v >> (i * 8)))
	}
}

func (w *condHasher) writeInt(n int) { w.writeUint(uint64(n)) }

// writeString 长度前缀 + 内容：既写入完整内容，又防止 "ab"+"c" 与 "a"+"bc"
// 拼接歧义。
func (w *condHasher) writeString(s string) {
	w.writeInt(len(s))
	for i := 0; i < len(s); i++ {
		w.writeByte(s[i])
	}
}

// 结构标记字节：区分相邻字段的类别，防止跨字段拼接歧义。
const (
	markNode       byte = 'C' // 条件节点开始
	markString     byte = 'S' // 字符串内容（SQL 结构）
	markNil        byte = 'N' // nil 占位
	markOpaque     byte = 'T' // 非 string 非 Condition 的类型占位
	markRightNil   byte = 'r' // Right 为 nil
	markRightSlice byte = 'l' // Right 为切片 → 仅长度
	markRightOther byte = 'o' // Right 为其他 → Go 类型名
	markArgsLen    byte = 'A' // Args 仅写长度
	markArgsLimit  byte = 'L' // LIMIT：写每个参数实际值
	markBegin      byte = '(' // 子条件递归开始
	markEnd        byte = ')' // 子条件递归结束
)

// CondHash 条件树结构哈希（FNV-1a）。规则（必须严格执行）：
//   - 写入 Type(int)
//   - Left 为 string：写入其完整内容（字段名/自定义SQL/子查询属于 SQL 结构）
//     非 string：仅写类型占位字节
//   - Right：nil→写标记；切片→写长度；其他→仅写 Go 类型名字符串
//   - Args：默认仅写长度；【唯一例外】Type==LIMIT 时写每个参数的实际数值
//     （会被内联进 SQL）
//   - AND/OR 的 Args 与 NOT 的 Left 若为 Condition 则递归
//     （先写递归开始/结束标记防歧义）；非 Condition 的字段引用子项按
//     Left 规则处理（字符串引用渲染为 SQL 文本的一部分，同属结构；
//     指针引用无 obj 可解析，仅占位——不同指针字段间存在碰撞可能，
//     该遗留语义本就依赖对象实例状态，不作为缓存精确性承诺范围）。
//
// 已知边界：IN_SUBQUERY/NOT_IN_SUBQUERY 的子查询文本存于 Right，
// 按规则仅参与类型名哈希，不同子查询文本共享同一 SQL 缓存条目时
// 以首条渲染结果为准（值红线：绝不碰值内容的既定取舍）。
func CondHash(c Condition) uint64 {
	w := newCondHasher()
	hashCondition(&w, c)
	return w.h
}

func hashCondition(w *condHasher, c Condition) {
	w.writeByte(markNode)
	w.writeInt(int(c.Type))

	switch lv := c.Left.(type) {
	case string:
		w.writeByte(markString)
		w.writeString(lv)
	case nil:
		w.writeByte(markNil)
	default:
		w.writeByte(markOpaque)
	}

	switch rv := c.Right.(type) {
	case nil:
		w.writeByte(markRightNil)
	default:
		rt := reflect.TypeOf(rv)
		if rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array {
			w.writeByte(markRightSlice)
			w.writeInt(reflect.ValueOf(rv).Len())
		} else {
			w.writeByte(markRightOther)
			w.writeString(rt.String())
		}
	}

	if c.Type == LIMIT {
		w.writeByte(markArgsLimit)
		w.writeInt(len(c.Args))
		for _, a := range c.Args {
			writeLimitArg(w, a)
		}
	} else {
		w.writeByte(markArgsLen)
		w.writeInt(len(c.Args))
	}

	switch c.Type {
	case AND, OR:
		for _, child := range c.Args {
			writeCompositeChild(w, child)
		}
	case NOT:
		writeCompositeChild(w, c.Left)
	}
}

// writeCompositeChild 处理 AND/OR 的 Args 与 NOT 的 Left：
// Condition → 递归；字符串字段引用 → 完整内容（SQL 结构）；
// 其他 → 类型占位字节。
func writeCompositeChild(w *condHasher, child any) {
	switch cv := child.(type) {
	case Condition:
		w.writeByte(markBegin)
		hashCondition(w, cv)
		w.writeByte(markEnd)
	case string:
		w.writeByte(markString)
		w.writeString(cv)
	default:
		w.writeByte(markOpaque)
	}
}

// writeLimitArg LIMIT 参数实际值的确定性编码。
func writeLimitArg(w *condHasher, v any) {
	if v == nil {
		w.writeByte(markNil)
		return
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.writeByte('i')
		w.writeUint(uint64(rv.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.writeByte('u')
		w.writeUint(rv.Uint())
	case reflect.Float32, reflect.Float64:
		w.writeByte('f')
		w.writeUint(math.Float64bits(rv.Float()))
	case reflect.String:
		w.writeByte('s')
		w.writeString(rv.String())
	case reflect.Bool:
		w.writeByte('b')
		if rv.Bool() {
			w.writeByte('1')
		} else {
			w.writeByte('0')
		}
	default:
		w.writeByte('o')
		w.writeString(rv.Kind().String())
	}
}

// condCacheKey L2 条件缓存键：方言注册名 + 表名 + 条件结构哈希。
type condCacheKey struct {
	dname string
	table string
	h     uint64
}

// condCacheCap 条件缓存的容量上限。超出后整体置换新 map（简单淘汰）：
// 动态拼接的条件可能使键空间无限膨胀，与其做精细 LRU 不如一次性清空，
// 被清掉的热点条件会在下次调用时重新渲染并回填。
const condCacheCap = 4096

var (
	condMapPtr atomic.Pointer[sync.Map] // map[condCacheKey]string
	condSize   atomic.Int64             // 当前条目计数（近似值：置换瞬间允许短暂偏差）
)

func init() { condMapPtr.Store(new(sync.Map)) }

func condCacheGet(k condCacheKey) (string, bool) {
	if v, ok := condMapPtr.Load().Load(k); ok {
		return v.(string), true
	}
	return "", false
}

func condCachePut(k condCacheKey, sqlText string) {
	m := condMapPtr.Load()
	if _, loaded := m.LoadOrStore(k, sqlText); loaded {
		return
	}
	if condSize.Add(1) > condCacheCap {
		fresh := new(sync.Map)
		if condMapPtr.CompareAndSwap(m, fresh) {
			condSize.Store(0)
		}
	}
}

// testHookRenders 包内测试钩子：记录 RenderCondition 的实际渲染次数
// （即 L2 未命中次数）。生产代码不得依赖其数值；测试断言相同形状的
// 第二次调用不再增长。
var testHookRenders atomic.Int64

// isCheapCondition 叶子比较条件：渲染成本低于“哈希+查表+二次收集”，
// 直渲反而更快（实测 92ns vs 168ns），绕过 L2。
func isCheapCondition(c Condition) bool {
	switch c.Type {
	case EQ, NE, LT, LTE, GT, GTE, LIKE, NOT_LIKE:
		return true
	}
	return false
}

// CachedRender 以 (方言名, 表名, 条件结构哈希) 为键返回条件 SQL 文本与绑定值。
// 命中时仅按镜像规则收集绑定值（零渲染）；未命中时渲染、存缓存后返回。
// RenderCondition 输出的 "?" 数量恒等于 collectBindValues 长度（parity 由测试保证）。
func CachedRender(d Dialect, table string, elems []Elem, obj any, objType reflect.Type, c Condition) (string, []any, error) {
	if isCheapCondition(c) {
		return RenderCondition(elems, obj, objType, c)
	}
	k := condCacheKey{dname: d.Name(), table: table, h: CondHash(c)}
	if sqlText, ok := condCacheGet(k); ok {
		args := make([]any, 0, 8)
		if err := collectBindValuesInto(obj, objType, elems, c, &args); err != nil {
			return "", nil, err
		}
		return sqlText, args, nil
	}
	testHookRenders.Add(1)
	sqlText, args, err := RenderCondition(elems, obj, objType, c)
	if err != nil {
		return "", nil, err
	}
	condCachePut(k, sqlText)
	return sqlText, args, nil
}

// bindCollector 绑定值收集器：以与渲染器完全一致的顺序提取绑定值（不渲染）。
type bindCollector struct {
	obj     any
	objType reflect.Type
	elems   []Elem
}

// collectBindValues 镜像规则（与 builder.go 渲染分支一一对应）：
// 比较类取 Right；IN/NOT_IN 取 Right 展开值；BETWEEN 取 Args[0],Args[1]；
// IN_VALUES 取 Args；LIMIT 按 arity 取 Args[0..]；CUSTOM 取 Args 全部；
// EXISTS / IN_SUBQUERY 等纯文本形态无绑定值；AND/OR/NOT 递归子项；
// 字段引用子项(resolveFieldRef 命中)取对象当前值——该场景依赖 obj，
// 因此需要 obj/objType 参数。
func collectBindValuesInto(obj any, objType reflect.Type, elems []Elem, c Condition, dst *[]any) error {
	r := &bindCollector{obj: obj, objType: objType, elems: elems}
	return r.collect(dst, c)
}

func resolveCollectorRef(r *bindCollector, arg any) (string, any) {
	cr := conditionRenderer{obj: r.obj, objType: r.objType, elems: r.elems}
	return cr.resolveFieldRef(arg)
}

func (r *bindCollector) collect(args *[]any, c Condition) error {
	switch c.Type {
	case AND, OR:
		for _, child := range c.Args {
			if sub, ok := child.(Condition); ok {
				if err := r.collect(args, sub); err != nil {
					return err
				}
			} else if tag, val := resolveCollectorRef(r, child); tag != "" {
				*args = append(*args, val)
			} else {
				return fmt.Errorf("%w: 组合条件子项 %v", ErrUnsupportedCondition, child)
			}
		}
	case NOT:
		if sub, ok := c.Left.(Condition); ok {
			return r.collect(args, sub)
		}
		tag, val := resolveCollectorRef(r, c.Left)
		if tag == "" {
			return fmt.Errorf("%w: NOT 子项", ErrUnsupportedCondition)
		}
		*args = append(*args, val)
	case EQ, NE, LT, LTE, GT, GTE:
		if _, ok := c.Left.(string); !ok || c.Right == nil {
			return fmt.Errorf("%w: 比较条件需要字段名和值", ErrInvalidCondition)
		}
		*args = append(*args, c.Right)
	case IN, NOT_IN:
		if _, ok := c.Left.(string); !ok {
			return fmt.Errorf("%w: IN 条件需要字段名", ErrInvalidCondition)
		}
		values, err := toAnySlice(c.Right)
		if err != nil {
			return err
		}
		if len(values) == 0 {
			// 空集合语义：IN 恒假、NOT IN 恒真，均无占位符
			return nil
		}
		*args = append(*args, values...)
	case LIKE, NOT_LIKE:
		if _, ok := c.Left.(string); !ok || c.Right == nil {
			return fmt.Errorf("%w: LIKE 条件需要字段名和值", ErrInvalidCondition)
		}
		*args = append(*args, c.Right)
	case BETWEEN, NOT_BETWEEN:
		if _, ok := c.Left.(string); !ok || len(c.Args) != 2 {
			return fmt.Errorf("%w: Between 需要字段名和 [下界, 上界]", ErrInvalidCondition)
		}
		*args = append(*args, c.Args[0], c.Args[1])
	case EXISTS, NOT_EXISTS:
		if _, ok := c.Left.(string); !ok {
			return fmt.Errorf("%w: EXISTS 需要子查询字符串", ErrInvalidCondition)
		}
	case IN_SUBQUERY, NOT_IN_SUBQUERY:
		_, okL := c.Left.(string)
		_, okR := c.Right.(string)
		if !okL || !okR {
			return fmt.Errorf("%w: IN 子查询需要字段名和子查询字符串", ErrInvalidCondition)
		}
	case IN_VALUES, NOT_IN_VALUES:
		if _, ok := c.Left.(string); !ok || len(c.Args) == 0 {
			return fmt.Errorf("%w: IN_VALUES 需要字段名和值列表", ErrInvalidCondition)
		}
		*args = append(*args, c.Args...)
	case LIMIT:
		switch len(c.Args) {
		case 1:
			*args = append(*args, c.Args[0])
		case 2:
			*args = append(*args, c.Args[0], c.Args[1])
		default:
			return fmt.Errorf("%w: Limit 需要 1 或 2 个参数", ErrInvalidCondition)
		}
	case CUSTOM:
		if _, ok := c.Left.(string); !ok {
			return fmt.Errorf("%w: Custom 需要 SQL 片段字符串", ErrInvalidCondition)
		}
		*args = append(*args, c.Args...)
	default:
		return fmt.Errorf("%w: 未知的条件类型 %d", ErrUnsupportedCondition, c.Type)
	}
	return nil
}

// ---------- L1+：语句形态缓存 ----------

// setPatternKey SET 子句缓存键：位模式编码“本次包含哪些列”。
type setPatternKey struct {
	dname string
	table string
	fp    uint64 // 列元数据指纹（与 stmtInternKey 一致，防同表异模型碰撞）
	pat   uint64
	incl  bool // true=Save(含零值)
}

var setPatternCache sync.Map // setPatternKey -> string（形如 `a` = ?,`b` = ?）

// patternBits 计算列包含位模式并同时收集绑定值（写入 dst）。
// 位 i = 第 i 个 Elem 是否参与本次 SET。
func patternBits(elems []Elem, includeZero bool, dst *[]any) uint64 {
	var pat uint64
	for i := range elems {
		e := &elems[i]
		if e.AutoInc {
			continue
		}
		if !includeZero && e.Zero() {
			continue
		}
		pat |= 1 << uint(i&63)
		*dst = append(*dst, e.Get())
	}
	return pat
}

// CachedSetClause 返回 SET 子句文本（不含 " SET " 前缀），命中零构建。
func CachedSetClause(d Dialect, table string, elems []Elem, includeZero bool, vals *[]any) (string, error) {
	return cachedSetClauseAt(d, d.Name(), table, elemsFingerprint(elems), elems, includeZero, vals)
}

func cachedSetClauseAt(d Dialect, dname, table string, fp uint64, elems []Elem, includeZero bool, vals *[]any) (string, error) {
	pat := patternBits(elems, includeZero, vals)
	if pat == 0 {
		return "", ErrNoFieldsToUpdate
	}
	k := setPatternKey{dname, table, fp, pat, includeZero}
	if v, ok := setPatternCache.Load(k); ok {
		return v.(string), nil
	}
	buf := getBuf()
	defer putBuf(buf)
	wrote := false
	for i := range elems {
		if pat&(1<<uint(i&63)) == 0 {
			continue
		}
		if wrote {
			buf.WriteByte(',')
		}
		buf.WriteString(d.Quote(elems[i].Tag))
		buf.WriteString(" = ?")
		wrote = true
	}
	q := strings.Clone(buf.String())
	setPatternCache.LoadOrStore(k, q)
	return q, nil
}

// staticPrefixKey SELECT/COUNT/DELETE 的静态前缀缓存键
type staticPrefixKey struct {
	dname string
	table string
	fp    uint64
	kind  uint8 // 0=selectCols 1=count 2=deleteBase 3=setEmpty?
}

var staticPrefixCache sync.Map

// CachedPrefix kind: 0=SELECT cols FROM t  1=SELECT COUNT(*) FROM t  2=DELETE FROM t
func CachedPrefix(d Dialect, table string, elems []Elem, kind uint8) string {
	return cachedPrefixAt(d, d.Name(), table, elemsFingerprint(elems), elems, kind)
}

func cachedPrefixAt(d Dialect, dname, table string, fp uint64, elems []Elem, kind uint8) string {
	k := staticPrefixKey{dname, table, fp, kind}
	if v, ok := staticPrefixCache.Load(k); ok {
		return v.(string)
	}
	buf := getBuf()
	defer putBuf(buf)
	switch kind {
	case 0:
		buf.WriteString("SELECT ")
		for i := range elems {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(d.Quote(elems[i].Tag))
		}
		buf.WriteString(" FROM ")
		buf.WriteString(d.Quote(table))
	case 1:
		buf.WriteString("SELECT COUNT(*) FROM ")
		buf.WriteString(d.Quote(table))
	default:
		buf.WriteString("DELETE FROM ")
		buf.WriteString(d.Quote(table))
	}
	q := strings.Clone(buf.String())
	staticPrefixCache.LoadOrStore(k, q)
	return q
}

// collectBindValues 分配版本（兼容既有测试）：见 collectBindValuesInto。
func collectBindValues(obj any, objType reflect.Type, elems []Elem, c Condition) []any {
	dst := make([]any, 0, 8)
	collectBindValuesInto(obj, objType, elems, c, &dst)
	return dst
}

// cachedWhereAppend 条件 SQL 与绑定值一步到位：值追加进调用方池化切片。
func cachedWhereAppend(d Dialect, table string, elems []Elem, obj any, objType reflect.Type, c Condition, dst *[]any) (string, error) {
	return cachedWhereAppendAt(d.Name(), table, elems, obj, objType, c, dst)
}

func cachedWhereAppendAt(dname, table string, elems []Elem, obj any, objType reflect.Type, c Condition, dst *[]any) (string, error) {
	if isCheapCondition(c) {
		sqlText, args, err := RenderCondition(elems, obj, objType, c)
		if err != nil {
			return "", err
		}
		*dst = append(*dst, args...)
		return sqlText, nil
	}
	k := condCacheKey{dname: dname, table: table, h: CondHash(c)}
	if sqlText, ok := condCacheGet(k); ok {
		if err := (&bindCollector{obj: obj, objType: objType, elems: elems}).collect(dst, c); err != nil {
			return "", err
		}
		return sqlText, nil
	}
	testHookRenders.Add(1)
	sqlText, args, err := RenderCondition(elems, obj, objType, c)
	if err != nil {
		return "", err
	}
	condCachePut(k, sqlText)
	*dst = append(*dst, args...)
	return sqlText, nil
}
