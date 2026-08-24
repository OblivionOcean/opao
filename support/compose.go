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
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// 本文件为链式糖层与原生路径共用的「语句组装」原语：
// 前缀(L1 静态缓存) + WHERE(L2 条件哈希缓存/叶子直渲) + 尾巴(三元组内联缓存)，
// 缓冲池化，调用方零字符串构建成本。

// WhereAppend 条件 SQL 渲染并把绑定值追加进 dst（叶子条件自动直渲）。
func WhereAppend(d Dialect, table string, elems []Elem, obj any, objType reflect.Type, c Condition, dst *[]any) (string, error) {
	return cachedWhereAppendAt(d.Name(), table, elems, obj, objType, c, dst)
}

// tailKey ORDER BY/LIMIT/OFFSET 尾巴的缓存键。
type tailKey struct {
	order   string
	limitN  int16 // -1 表示无 LIMIT
	offsetN int16
}

var tailCache sync.Map // tailKey -> string

// CachedTail 生成并缓存排序分页尾巴。order 为已通过上层校验的片段；
// limit<=0 视为无 LIMIT（此时 offset 必须为 0）。
func CachedTail(order string, limit, offset int) string {
	l := int16(limit)
	f := int16(offset)
	if limit <= 0 {
		l = -1
		f = 0
	}
	k := tailKey{order: order, limitN: l, offsetN: f}
	if v, ok := tailCache.Load(k); ok {
		return v.(string)
	}
	buf := getBuf()
	defer putBuf(buf)
	if order != "" {
		buf.WriteString(" ORDER BY ")
		buf.WriteString(order)
	}
	if limit > 0 {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(limit))
		if offset > 0 {
			buf.WriteString(" OFFSET ")
			buf.WriteString(strconv.Itoa(offset))
		}
	}
	q := strings.Clone(buf.String()) // 池化数组会被复用，必须克隆后存储
	tailCache.Store(k, q)
	return q
}

// ComposeStatement 组装 SELECT/COUNT/DELETE 语句：
//   - kind: 0=SELECT cols  1=SELECT COUNT(*)  2=DELETE FROM
//   - cond 与 rawSQL 二选一（均可为空表示无条件）
//   - tail 为已生成的尾巴文本（如 CachedTail 的返回值），仅对 SELECT 有意义
//
// 返回的绑定值切片来自池化复用，调用方在对应 Exec 返回后不得继续持有。
func ComposeStatement(d Dialect, kind uint8, table string, elems []Elem, obj any, objType reflect.Type,
	cond Condition, rawSQL string, rawArgs []any, tail string) (string, []any, error) {

	pa := getArgs()
	buf := getBuf()
	*buf = (*buf)[:0] // 防御性重置：池化缓冲严禁携带残留
	release := true
	defer func() {
		if release {
			putBuf(buf)
		}
	}()

	buf.WriteString(CachedPrefix(d, table, elems, kind))
	hasCond := cond.Type != 0 // Type 零值即未设置（合法条件类型从 1 起）
	switch {
	case hasCond:
		w, err := cachedWhereAppendAt(d.Name(), table, elems, obj, objType, cond, pa)
		if err != nil {
			putArgs(pa)
			return "", nil, err
		}
		if w != "" {
			appendClause(buf, w)
		}
	case rawSQL != "":
		appendClause(buf, rawSQL)
		*pa = append(*pa, rawArgs...)
	}
	if tail != "" && kind == 0 {
		buf.WriteString(tail)
	}
	// 出口必须克隆：池化数组会被后续调用复用，零拷贝字符串会被覆写污染
	sqlText := strings.Clone(Rebind(d, buf.String()))
	argsOut := make([]any, len(*pa))
	copy(argsOut, *pa)
	putBuf(buf)
	putArgs(pa)
	release = false
	_ = release
	return sqlText, argsOut, nil
}

// CachedColsClause 显式列映射的 SET 子句缓存：键为排序后的列名集合。
func CachedColsClause(d Dialect, table string, keys []string) (string, error) {
	h := uint64(14695981039346656037)
	for _, k := range keys {
		for i := 0; i < len(k); i++ {
			h ^= uint64(k[i])
			h *= 1099511628211
		}
		h ^= 0xff
		h *= 1099511628211
	}
	type ck struct {
		dname, table string
		hh           uint64
	}
	k := ck{d.Name(), table, h}
	if v, ok := colsCache.Load(k); ok {
		return v.(string), nil
	}
	buf := getBuf()
	defer putBuf(buf)
	for i, name := range keys {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(d.Quote(name))
		buf.WriteString(" = ?")
	}
	q := buf.String()
	q = strings.Clone(q)
	colsCache.LoadOrStore(k, q)
	return q, nil
}

var colsCache sync.Map

// SortMapKeys 返回 map 键的排序副本（保证 SET 子句确定性）。
func SortMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
