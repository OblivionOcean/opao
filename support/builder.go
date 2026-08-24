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
	"reflect"
	"strings"

	"unsafe"

	"github.com/OblivionOcean/opao/utils"
)

// BuildQuery 将 variadic 查询参数归一化为 (条件, 原始参数)：
//   - 无参数            → nil 条件
//   - 单个 Condition    → 该条件树（多余参数忽略，保持历史行为）
//   - 字符串 + 可变参数 → 原样透传，由调用方拼接
func BuildQuery(queryParts ...any) (cond any, rawArgs []any, err error) {
	switch len(queryParts) {
	case 0:
		return nil, nil, nil
	case 1:
		p := queryParts[0]
		if p == nil {
			return nil, nil, nil
		}
		switch v := p.(type) {
		case Condition:
			return v, nil, nil
		case string:
			return v, nil, nil
		default:
			return nil, nil, fmt.Errorf("%w: %T", ErrUnsupportedCondition, p)
		}
	default:
		s, ok := queryParts[0].(string)
		if !ok {
			return nil, nil, fmt.Errorf("%w: 多参数形式首参必须为字符串", ErrUnsupportedCondition)
		}
		return s, queryParts[1:], nil
	}
}

// conditionRenderer 条件树渲染器。统一输出 "?" 占位符，
// 由 Rebind 在语句级转换为方言形式。
type conditionRenderer struct {
	obj     any
	objType reflect.Type
	elems   []Elem
}

// RenderCondition 将条件树渲染为 SQL 片段（不含 WHERE 关键字与 LIMIT 由条件自身携带）。
// 返回错误而非 panic；嵌套的 AND/OR 子组合条件自动加括号以保证运算优先级。
func RenderCondition(elems []Elem, obj any, objType reflect.Type, c Condition) (string, []any, error) {
	r := &conditionRenderer{obj: obj, objType: objType, elems: elems}
	buf := utils.NewBuffer(128)
	args, err := r.render(&buf, nil, c)
	if err != nil {
		return "", nil, err
	}
	return buf.String(), args, nil
}

func (r *conditionRenderer) writeBind(buf *utils.Buffer, args []any, v any) []any {
	buf.WriteString("?")
	return append(args, v)
}

// ifaceWord 读取接口值的动态数据字（两字布局的第二字）。
// 该布局是 Go 运行时与 cgo 共同依赖的基础表示，事实上稳定。
type ifaceWord struct {
	typ, data unsafe.Pointer
}

// resolveFieldRef 兼容历史语义的字段引用解析：
//   - 字符串：匹配 Elem 标签 → (列名, 对象当前字段值)
//   - 指针：与关联对象做偏移匹配（支持 And(&user.Age) 形式）
//
// 未命中返回 ("", nil)。
func (r *conditionRenderer) resolveFieldRef(arg any) (string, any) {
	if arg == nil {
		return "", nil
	}
	switch v := arg.(type) {
	case string:
		for i := range r.elems {
			if r.elems[i].Tag == v {
				return v, r.elems[i].Get()
			}
		}
		return "", nil
	}
	at := reflect.TypeOf(arg)
	if at.Kind() != reflect.Ptr || r.obj == nil {
		return "", nil
	}
	objData := (*ifaceWord)(unsafe.Pointer(&r.obj)).data
	argData := (*ifaceWord)(unsafe.Pointer(&arg)).data
	if objData == nil || argData == nil {
		return "", nil
	}
	offset := uintptr(argData) - uintptr(objData)
	for i := range r.elems {
		if r.elems[i].Offset == offset {
			return r.elems[i].Tag, r.elems[i].Get()
		}
	}
	return "", nil
}

func (r *conditionRenderer) render(buf *utils.Buffer, args []any, c Condition) (argsOut []any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: 条件解析异常: %v", ErrInvalidCondition, p)
			argsOut = args
		}
	}()

	switch c.Type {
	case AND, OR:
		op := " AND "
		if c.Type == OR {
			op = " OR "
		}
		for i, child := range c.Args {
			if i > 0 {
				buf.WriteString(op)
			}
			if sub, ok := child.(Condition); ok {
				args, err = r.renderCompositeChild(buf, args, sub)
			} else if tag, val := r.resolveFieldRef(child); tag != "" {
				// 兼容历史语义：字段名/字段指针引用 → 以对象当前值做等值比较
				buf.WriteString(tag)
				buf.WriteString(" = ")
				args = r.writeBind(buf, args, val)
			} else {
				return args, fmt.Errorf("%w: 组合条件子项 %v", ErrUnsupportedCondition, child)
			}
		}
	case NOT:
		buf.WriteString("NOT (")
		if sub, ok := c.Left.(Condition); ok {
			args, err = r.render(buf, args, sub)
			if err != nil {
				return args, err
			}
		} else if tag, val := r.resolveFieldRef(c.Left); tag != "" {
			buf.WriteString(tag)
			buf.WriteString(" = ")
			args = r.writeBind(buf, args, val)
		} else {
			return args, fmt.Errorf("%w: NOT 子项", ErrUnsupportedCondition)
		}
		buf.WriteByte(')')
	case EQ, NE, LT, LTE, GT, GTE:
		left, ok := c.Left.(string)
		if !ok || c.Right == nil {
			return args, fmt.Errorf("%w: 比较条件需要字段名和值", ErrInvalidCondition)
		}
		buf.WriteString(left)
		switch c.Type {
		case EQ:
			buf.WriteString(" = ")
		case NE:
			buf.WriteString(" <> ")
		case LT:
			buf.WriteString(" < ")
		case LTE:
			buf.WriteString(" <= ")
		case GT:
			buf.WriteString(" > ")
		case GTE:
			buf.WriteString(" >= ")
		}
		args = r.writeBind(buf, args, c.Right)
	case IN, NOT_IN:
		left, ok := c.Left.(string)
		if !ok {
			return args, fmt.Errorf("%w: IN 条件需要字段名", ErrInvalidCondition)
		}
		values, err := toAnySlice(c.Right)
		if err != nil {
			return args, err
		}
		if len(values) == 0 {
			// 空集合语义：IN 恒假，NOT IN 恒真
			if c.Type == IN {
				buf.WriteString("1 = 0")
			} else {
				buf.WriteString("1 = 1")
			}
			return args, nil
		}
		buf.WriteString(left)
		if c.Type == IN {
			buf.WriteString(" IN (")
		} else {
			buf.WriteString(" NOT IN (")
		}
		for i, v := range values {
			if i > 0 {
				buf.WriteByte(',')
			}
			args = r.writeBind(buf, args, v)
		}
		buf.WriteByte(')')
	case LIKE, NOT_LIKE:
		left, ok := c.Left.(string)
		if !ok || c.Right == nil {
			return args, fmt.Errorf("%w: LIKE 条件需要字段名和值", ErrInvalidCondition)
		}
		buf.WriteString(left)
		if c.Type == LIKE {
			buf.WriteString(" LIKE ")
		} else {
			buf.WriteString(" NOT LIKE ")
		}
		args = r.writeBind(buf, args, c.Right)
	case BETWEEN, NOT_BETWEEN:
		left, ok := c.Left.(string)
		if !ok || len(c.Args) != 2 {
			return args, fmt.Errorf("%w: Between 需要字段名和 [下界, 上界]", ErrInvalidCondition)
		}
		buf.WriteString(left)
		if c.Type == BETWEEN {
			buf.WriteString(" BETWEEN ")
		} else {
			buf.WriteString(" NOT BETWEEN ")
		}
		args = r.writeBind(buf, args, c.Args[0])
		buf.WriteString(" AND ")
		args = r.writeBind(buf, args, c.Args[1])
	case EXISTS, NOT_EXISTS:
		sub, ok := c.Left.(string)
		if !ok {
			return args, fmt.Errorf("%w: EXISTS 需要子查询字符串", ErrInvalidCondition)
		}
		if c.Type == EXISTS {
			buf.WriteString("EXISTS (")
		} else {
			buf.WriteString("NOT EXISTS (")
		}
		buf.WriteString(sub)
		buf.WriteByte(')')
	case IN_SUBQUERY, NOT_IN_SUBQUERY:
		left, okL := c.Left.(string)
		sub, okR := c.Right.(string)
		if !okL || !okR {
			return args, fmt.Errorf("%w: IN 子查询需要字段名和子查询字符串", ErrInvalidCondition)
		}
		if c.Type == IN_SUBQUERY {
			buf.WriteString(left + " IN (" + sub + ")")
		} else {
			buf.WriteString(left + " NOT IN (" + sub + ")")
		}
	case IN_VALUES, NOT_IN_VALUES:
		left, ok := c.Left.(string)
		if !ok || len(c.Args) == 0 {
			return args, fmt.Errorf("%w: IN_VALUES 需要字段名和值列表", ErrInvalidCondition)
		}
		buf.WriteString(left)
		if c.Type == IN_VALUES {
			buf.WriteString(" IN (")
		} else {
			buf.WriteString(" NOT IN (")
		}
		for i, v := range c.Args {
			if i > 0 {
				buf.WriteByte(',')
			}
			args = r.writeBind(buf, args, v)
		}
		buf.WriteByte(')')
	case LIMIT:
		switch len(c.Args) {
		case 1:
			buf.WriteString("LIMIT ")
			args = r.writeBind(buf, args, c.Args[0])
		case 2:
			buf.WriteString("LIMIT ")
			args = r.writeBind(buf, args, c.Args[0])
			buf.WriteString(" OFFSET ")
			args = r.writeBind(buf, args, c.Args[1])
		default:
			return args, fmt.Errorf("%w: Limit 需要 1 或 2 个参数", ErrInvalidCondition)
		}
	case CUSTOM:
		text, ok := c.Left.(string)
		if !ok {
			return args, fmt.Errorf("%w: Custom 需要 SQL 片段字符串", ErrInvalidCondition)
		}
		buf.WriteString(text)
		args = append(args, c.Args...)
	default:
		return args, fmt.Errorf("%w: 未知的条件类型 %d", ErrUnsupportedCondition, c.Type)
	}
	return args, nil
}

// renderCompositeChild 渲染 AND/OR 的 Condition 子项：组合类型自动加括号
func (r *conditionRenderer) renderCompositeChild(buf *utils.Buffer, args []any, c Condition) ([]any, error) {
	if c.Type == AND || c.Type == OR {
		buf.WriteByte('(')
		args, err := r.render(buf, args, c)
		if err != nil {
			return args, err
		}
		buf.WriteByte(')')
		return args, nil
	}
	return r.render(buf, args, c)
}

// toAnySlice 将切片形式的值统一为 []any
func toAnySlice(v any) ([]any, error) {
	if vs, ok := v.([]any); ok {
		return vs, nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = rv.Index(i).Interface()
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: IN 条件的值必须是切片", ErrInvalidCondition)
	}
}

// assembleWhere 组装 WHERE 片段（不含 LIMIT 之外的尾部处理），返回空串表示无条件。
// 占位符统一为 "?"，由 Rebind 做方言转换。
// Condition 分支经 CachedRender 走 L2 条件结构哈希缓存：
// 相同 (方言, 表, 结构) 的条件复用已渲染 SQL 文本，仅重新收集绑定值。
func assembleWhere(d Dialect, table string, elems []Elem, obj any, objType reflect.Type, whereParts []any) (string, []any, error) {
	cond, rawArgs, err := BuildQuery(whereParts...)
	if err != nil || cond == nil {
		return "", nil, err
	}
	switch c := cond.(type) {
	case string:
		return c, rawArgs, nil
	case Condition:
		sqlText, args, err := CachedRender(d, table, elems, obj, objType, c)
		if err != nil {
			return "", nil, err
		}
		return sqlText, args, nil
	default:
		return "", nil, fmt.Errorf("%w", ErrUnsupportedCondition)
	}
}

// Rebind 将语句中的 "?" 占位符按方言重写（当前仅 postgres 需要 $n 形式）。
func Rebind(d Dialect, query string) string {
	if d.Name() != "postgres" {
		return query
	}
	var sb strings.Builder
	sb.Grow(len(query) + 8)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			sb.WriteString(d.BindVar(n))
		} else {
			sb.WriteByte(query[i])
		}
	}
	return sb.String()
}

// BuildInsert 生成 INSERT 语句，跳过自增字段；列与占位符同源遍历保证一致。
func BuildInsert(d Dialect, table string, elems []Elem) (query string, values []any, err error) {
	buf := utils.NewBuffer(48 + len(table))
	buf.WriteString("INSERT INTO ")
	buf.WriteString(d.Quote(table))
	buf.WriteString(" (")
	values = make([]any, 0, len(elems))
	wrote := false
	for i := range elems {
		e := &elems[i]
		if e.AutoInc {
			continue
		}
		if wrote {
			buf.WriteByte(',')
		}
		buf.WriteString(d.Quote(e.Tag))
		values = append(values, e.Get())
		wrote = true
	}
	if !wrote {
		return "", nil, fmt.Errorf("%w", ErrNoFieldsToUpdate)
	}
	buf.WriteString(") VALUES (")
	for i := range values {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('?')
	}
	buf.WriteByte(')')
	return Rebind(d, buf.String()), values, nil
}

// BuildSelect 生成 SELECT 语句
func BuildSelect(d Dialect, table string, elems []Elem, obj any, objType reflect.Type, whereParts ...any) (string, []any, error) {
	buf := utils.NewBuffer(64 + len(table))
	buf.WriteString("SELECT ")
	for i := range elems {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(d.Quote(elems[i].Tag))
	}
	buf.WriteString(" FROM ")
	buf.WriteString(d.Quote(table))

	where, args, err := assembleWhere(d, table, elems, obj, objType, whereParts)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		appendClause(&buf, where)
	}
	return Rebind(d, buf.String()), args, nil
}

// BuildUpdate 生成 UPDATE 语句。includeZero=false 时跳过零值列；
// 自增列始终跳过。无可写列时返回 ErrNoFieldsToUpdate。
func BuildUpdate(d Dialect, table string, elems []Elem, includeZero bool, obj any, objType reflect.Type, whereParts ...any) (string, []any, error) {
	setVals := make([]any, 0, len(elems))
	buf := utils.NewBuffer(64 + len(table))
	buf.WriteString("UPDATE ")
	buf.WriteString(d.Quote(table))
	buf.WriteString(" SET ")
	wrote := false
	for i := range elems {
		e := &elems[i]
		if e.AutoInc {
			continue
		}
		if !includeZero && e.Zero() {
			continue
		}
		if wrote {
			buf.WriteByte(',')
		}
		buf.WriteString(d.Quote(e.Tag))
		buf.WriteString(" = ?")
		setVals = append(setVals, e.Get())
		wrote = true
	}
	if !wrote {
		return "", nil, fmt.Errorf("%w", ErrNoFieldsToUpdate)
	}

	where, whereArgs, err := assembleWhere(d, table, elems, obj, objType, whereParts)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		appendClause(&buf, where)
	}
	return Rebind(d, buf.String()), append(setVals, whereArgs...), nil
}

// BuildDelete 生成 DELETE 语句
func BuildDelete(d Dialect, table string, elems []Elem, obj any, objType reflect.Type, whereParts ...any) (string, []any, error) {
	buf := utils.NewBuffer(24 + len(table))
	buf.WriteString("DELETE FROM ")
	buf.WriteString(d.Quote(table))
	where, args, err := assembleWhere(d, table, elems, obj, objType, whereParts)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		appendClause(&buf, where)
	}
	return Rebind(d, buf.String()), args, nil
}

// BuildCount 生成 SELECT COUNT(*) 语句
func BuildCount(d Dialect, table string, elems []Elem, obj any, objType reflect.Type, whereParts ...any) (string, []any, error) {
	buf := utils.NewBuffer(32 + len(table))
	buf.WriteString("SELECT COUNT(*) FROM ")
	buf.WriteString(d.Quote(table))
	where, args, err := assembleWhere(d, table, elems, obj, objType, whereParts)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		appendClause(&buf, where)
	}
	return Rebind(d, buf.String()), args, nil
}

// appendClause 追加 WHERE 或独立的 LIMIT 子句
// （LIMIT 类条件由渲染器直接产出 "LIMIT ..." 片段，不能挂接 WHERE 前缀）
func appendClause(buf *utils.Buffer, clause string) {
	if strings.HasPrefix(clause, "LIMIT") {
		buf.WriteByte(' ')
	} else {
		buf.WriteString(" WHERE ")
	}
	buf.WriteString(clause)
}
