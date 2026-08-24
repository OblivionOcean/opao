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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

// —— 测试用本地方言桩，避免子包循环导入 ——
type tdialect struct {
	name    string
	quote   func(string) string
	bindvar func(int) string
}

func (d tdialect) Name() string           { return d.name }
func (d tdialect) Quote(id string) string { return d.quote(id) }
func (d tdialect) BindVar(n int) string   { return d.bindvar(n) }

var (
	mysqlDial    = tdialect{"mysql", func(s string) string { return "`" + s + "`" }, func(int) string { return "?" }}
	postgresDial = tdialect{"postgres", func(s string) string { return `"` + s + `"` }, func(n int) string { return fmt.Sprintf("$%d", n) }}
	sqliteDial   = tdialect{"sqlite", func(s string) string { return `"` + s + `"` }, func(int) string { return "?" }}
)

// ---- 测试夹具：带合法 Ptr 的字段元数据，支撑字段引用解析路径 ----

type m4RefObj struct {
	Age  int
	Name string
}

func m4Elems(u *m4RefObj) []Elem {
	ot := reflect.TypeOf(*u)
	elems := make([]Elem, 0, ot.NumField())
	for i := 0; i < ot.NumField(); i++ {
		f := ot.Field(i)
		elems = append(elems, Elem{
			Index:  i,
			Type:   f.Type,
			Tag:    strings.ToLower(f.Name),
			Offset: f.Offset,
		})
	}
	base := unsafe.Pointer(u)
	for i := range elems {
		elems[i].Ptr = unsafe.Add(base, elems[i].Offset)
	}
	return elems
}

// m4And/m4Or/m4Not 组合条件构造（包内测试用，镜像 opao 包语义）。
func m4And(children ...any) Condition { return Condition{Type: AND, Args: children} }
func m4Or(children ...any) Condition  { return Condition{Type: OR, Args: children} }
func m4Not(child any) Condition       { return Condition{Type: NOT, Left: child} }

// ---- CondHash 稳定性 ----

func TestCondHashStability(t *testing.T) {
	// 值不参与哈希：Eq("id",1) 与 Eq("id",2) 同哈希（共享同一 SQL 文本）
	h1 := CondHash(Condition{Type: EQ, Left: "id", Right: 1})
	h2 := CondHash(Condition{Type: EQ, Left: "id", Right: 2})
	if h1 != h2 {
		t.Fatalf("Eq(\"id\",1) 与 Eq(\"id\",2) 应同哈希: %d != %d", h1, h2)
	}

	// LIMIT 参数被内联进 SQL：Limit(10) 与 Limit(20) 异哈希
	if CondHash(Condition{Type: LIMIT, Args: []any{10}}) ==
		CondHash(Condition{Type: LIMIT, Args: []any{20}}) {
		t.Fatal("Limit(10) 与 Limit(20) 应异哈希")
	}

	// 结构差异必须体现：三层嵌套 And(Or(..),..) 与扁平 Or(..) 异哈希
	nested := m4And(
		m4Or(
			Condition{Type: EQ, Left: "age", Right: 18},
			Condition{Type: EQ, Left: "age", Right: 20},
		),
		Condition{Type: EQ, Left: "name", Right: "a"},
	)
	flat := m4Or(
		Condition{Type: EQ, Left: "age", Right: 18},
		Condition{Type: EQ, Left: "age", Right: 20},
	)
	if CondHash(nested) == CondHash(flat) {
		t.Fatal("And(Or(..),..) 与 Or(..) 应异哈希")
	}

	// 自定义 SQL 文本属于结构：文本差异异哈希
	c1 := Condition{Type: CUSTOM, Left: "score > ?", Args: []any{60}}
	c2 := Condition{Type: CUSTOM, Left: "score < ?", Args: []any{60}}
	if CondHash(c1) == CondHash(c2) {
		t.Fatal("Custom 不同文本应异哈希")
	}

	// 哈希确定性：同结构重复计算恒等
	first := CondHash(nested)
	for i := 0; i < 100; i++ {
		if CondHash(nested) != first {
			t.Fatal("CondHash 不具备确定性")
		}
	}

	// 切片长度参与哈希（影响占位符数）
	in2 := Condition{Type: IN, Left: "age", Right: []any{1, 2}}
	in3 := Condition{Type: IN, Left: "age", Right: []any{1, 2, 3}}
	if CondHash(in2) == CondHash(in3) {
		t.Fatal("IN 长度差异应异哈希")
	}

	// 字段名差异异哈希
	if CondHash(Condition{Type: EQ, Left: "age", Right: 1}) ==
		CondHash(Condition{Type: EQ, Left: "name", Right: 1}) {
		t.Fatal("字段名差异应异哈希")
	}
}

// ---- parity 穷举矩阵 ----

type parityCase struct {
	name  string
	cond  Condition
	table string // 用例独立表名，隔离 L2 缓存键空间
}

func buildParityCases(u *m4RefObj) []parityCase {
	eqAge25 := Condition{Type: EQ, Left: "age", Right: 25}
	gtID5 := Condition{Type: GT, Left: "id", Right: 5}
	inList := Condition{Type: IN, Left: "age", Right: []any{20, 30}}

	return []parityCase{
		// 组合类：AND / OR / NOT，含字符串字段引用与指针引用子项
		{"AND", m4And(eqAge25, gtID5), "p_and"},
		{"AND_ref", m4And("age", gtID5), "p_andref"},
		{"AND_ptr", m4And(&u.Name, eqAge25), "p_andptr"},
		{"OR", m4Or(eqAge25, gtID5), "p_or"},
		{"OR_ref", m4Or("name", inList), "p_orref"},
		{"NOT_cond", m4Not(eqAge25), "p_notc"},
		{"NOT_ref", m4Not("age"), "p_notr"},
		{"NOT_ptr", m4Not(&u.Age), "p_notp"},

		// 比较类 × 2 组不同值（值共享条目，字段名区分结构）
		{"EQ_a", Condition{Type: EQ, Left: "age", Right: 25}, "p_eq"},
		{"EQ_b", Condition{Type: EQ, Left: "name", Right: "张三"}, "p_eq2"},
		{"NE_a", Condition{Type: NE, Left: "age", Right: 0}, "p_ne"},
		{"NE_b", Condition{Type: NE, Left: "name", Right: ""}, "p_ne2"},
		{"GT_a", Condition{Type: GT, Left: "age", Right: 18}, "p_gt"},
		{"GT_b", Condition{Type: GT, Left: "age", Right: 65}, "p_gt"},
		{"LT_a", Condition{Type: LT, Left: "age", Right: 10}, "p_lt"},
		{"LT_b", Condition{Type: LT, Left: "age", Right: 99}, "p_lt"},
		{"GTE_a", Condition{Type: GTE, Left: "age", Right: 1}, "p_gte"},
		{"GTE_b", Condition{Type: GTE, Left: "age", Right: 2}, "p_gte"},
		{"LTE_a", Condition{Type: LTE, Left: "age", Right: 7}, "p_lte"},
		{"LTE_b", Condition{Type: LTE, Left: "age", Right: 8}, "p_lte"},

		// LIKE 类
		{"LIKE_a", Condition{Type: LIKE, Left: "name", Right: "%a%"}, "p_like"},
		{"LIKE_b", Condition{Type: LIKE, Left: "name", Right: "b%"}, "p_like"},
		{"NOT_LIKE_a", Condition{Type: NOT_LIKE, Left: "name", Right: "%x%"}, "p_nlike"},
		{"NOT_LIKE_b", Condition{Type: NOT_LIKE, Left: "name", Right: "%y%"}, "p_nlike"},

		// IN / NOT_IN：展开值、空集合恒真恒假、数组形态
		{"IN_a", Condition{Type: IN, Left: "age", Right: []any{20, 30}}, "p_in"},
		{"IN_b", Condition{Type: IN, Left: "age", Right: []any{40}}, "p_in2"},
		{"IN_empty", Condition{Type: IN, Left: "age", Right: []any{}}, "p_in3"},
		{"NOT_IN_a", Condition{Type: NOT_IN, Left: "age", Right: []any{1, 2, 3}}, "p_nin"},
		{"NOT_IN_arr", Condition{Type: NOT_IN, Left: "age", Right: [2]any{"a", "b"}}, "p_nin2"},
		{"NOT_IN_empty", Condition{Type: NOT_IN, Left: "age", Right: []any{}}, "p_nin3"},

		// BETWEEN 类
		{"BETWEEN_a", Condition{Type: BETWEEN, Left: "age", Args: []any{18, 30}}, "p_bw"},
		{"BETWEEN_b", Condition{Type: BETWEEN, Left: "age", Args: []any{31, 64}}, "p_bw"},
		{"NOT_BETWEEN_a", Condition{Type: NOT_BETWEEN, Left: "age", Args: []any{0, 17}}, "p_nbw"},
		{"NOT_BETWEEN_b", Condition{Type: NOT_BETWEEN, Left: "age", Args: []any{66, 99}}, "p_nbw"},

		// 存在性 / 子查询（纯文本形态，无绑定值）
		{"EXISTS", Condition{Type: EXISTS, Left: "SELECT 1 FROM users"}, "p_ex"},
		{"NOT_EXISTS", Condition{Type: NOT_EXISTS, Left: "SELECT 1 FROM orders"}, "p_nex"},
		{"IN_SUBQUERY_a", Condition{Type: IN_SUBQUERY, Left: "id", Right: "SELECT id FROM t1"}, "p_ins"},
		{"IN_SUBQUERY_b", Condition{Type: IN_SUBQUERY, Left: "uid", Right: "SELECT uid FROM t2"}, "p_ins2"},
		{"NOT_IN_SUBQUERY", Condition{Type: NOT_IN_SUBQUERY, Left: "id", Right: "SELECT x FROM t3"}, "p_nins"},

		// 值列表
		{"IN_VALUES_a", Condition{Type: IN_VALUES, Left: "age", Args: []any{1, 2, 3}}, "p_inv"},
		{"IN_VALUES_b", Condition{Type: IN_VALUES, Left: "age", Args: []any{9}}, "p_inv2"},
		{"NOT_IN_VALUES_a", Condition{Type: NOT_IN_VALUES, Left: "age", Args: []any{4, 5}}, "p_ninv"},
		{"NOT_IN_VALUES_b", Condition{Type: NOT_IN_VALUES, Left: "age", Args: []any{6}}, "p_ninv2"},

		// LIMIT：arity 1 与 2；数值被内联故必须各自成条目
		{"LIMIT_1_a", Condition{Type: LIMIT, Args: []any{10}}, "p_lim"},
		{"LIMIT_1_b", Condition{Type: LIMIT, Args: []any{20}}, "p_lim2"},
		{"LIMIT_2", Condition{Type: LIMIT, Args: []any{10, 5}}, "p_lim3"},

		// CUSTOM
		{"CUSTOM_args", Condition{Type: CUSTOM, Left: "score > ?", Args: []any{60}}, "p_cust"},
		{"CUSTOM_noargs", Condition{Type: CUSTOM, Left: "created_at IS NOT NULL"}, "p_cust2"},

		// AND/OR/NOT 三层嵌套组合（覆盖递归收集路径）
		{"NEST3", m4And(
			m4Or(eqAge25, m4Not(gtID5)),
			m4Not(m4Or(
				Condition{Type: BETWEEN, Left: "age", Args: []any{40, 50}},
				Condition{Type: LIKE, Left: "name", Right: "%z%"},
			)),
		), "p_nest"},
		{"NEST3_mixed", m4And(
			m4Or("age", m4Not(Condition{Type: IN_VALUES, Left: "age", Args: []any{1, 9}})),
			m4Not(Condition{Type: CUSTOM, Left: "flag = ?", Args: []any{true}}),
		), "p_nest2"},
	}
}

func TestCondCacheParityMatrix(t *testing.T) {
	// 指针引用子项必须指向同一实例：obj 与条件内的指针同源。
	u := &m4RefObj{Age: 33, Name: "bob"}
	objType := reflect.TypeOf(m4RefObj{})
	elems := m4Elems(u)
	cases := buildParityCases(u)

	totalAssertions := 0
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// 纯渲染路径（基准）
			wantSQL, wantArgs, err := RenderCondition(elems, u, objType, tc.cond)
			if err != nil {
				t.Fatalf("RenderCondition: %v", err)
			}

			before := testHookRenders.Load()
			sql1, args1, err := CachedRender(mysqlDial, tc.table, elems, u, objType, tc.cond)
			if err != nil {
				t.Fatalf("CachedRender#1: %v", err)
			}
			afterFirst := testHookRenders.Load()
			sql2, args2, err := CachedRender(mysqlDial, tc.table, elems, u, objType, tc.cond)
			if err != nil {
				t.Fatalf("CachedRender#2: %v", err)
			}
			afterSecond := testHookRenders.Load()

			// SQL 一致性：缓存路径两次结果均与纯渲染相同
			if sql1 != wantSQL || sql2 != wantSQL {
				t.Fatalf("SQL 不一致:\n render=%q\n cache1=%q\n cache2=%q", wantSQL, sql1, sql2)
			}
			// 第二次必命中：渲染计数不再增长
			if afterSecond != afterFirst {
				t.Fatalf("第二次调用不应触发渲染: first=%d second=%d", afterFirst, afterSecond)
			}
			if afterFirst-before > 1 {
				t.Fatalf("首次调用至多渲染一次: delta=%d", afterFirst-before)
			}

			// 绑定值逐元素相等
			if len(args1) != len(wantArgs) || len(args2) != len(wantArgs) {
				t.Fatalf("参数数量不一致: render=%d cache1=%d cache2=%d",
					len(wantArgs), len(args1), len(args2))
			}
			for i := range wantArgs {
				if !reflect.DeepEqual(args1[i], wantArgs[i]) ||
					!reflect.DeepEqual(args2[i], wantArgs[i]) {
					t.Fatalf("第 %d 个绑定值不一致: render=%v cache1=%v cache2=%v",
						i, wantArgs[i], args1[i], args2[i])
				}
			}
			// 占位符守恒：? 数量 == 绑定值数量
			if got := strings.Count(sql1, "?"); got != len(wantArgs) {
				t.Fatalf("? 数量(%d) != 绑定值数量(%d)，SQL=%q", got, len(wantArgs), sql1)
			}
			totalAssertions += 4 + 2*len(wantArgs)
		})
	}
	t.Logf("parity 矩阵完成：%d 条件用例 × (SQL×2 + 计数×2 + 参数逐项×2 + 守恒)，累计约 %d 项断言",
		len(cases), totalAssertions)
}

// TestCondCacheHitCounter 同一形状多次调用仅首渲染：
// 首次调用后记录计数，随后以不同方言/对象重复调用断言计数不再增长。
func TestCondCacheHitCounter(t *testing.T) {
	u := &m4RefObj{Age: 41, Name: "counter"}
	objType := reflect.TypeOf(m4RefObj{})
	elems := m4Elems(u)
	cond := m4And(
		Condition{Type: GT, Left: "age", Right: 10},
		Condition{Type: LIKE, Left: "name", Right: "%o%"},
	)
	const table = "p_counter"

	if _, _, err := CachedRender(mysqlDial, table, elems, nil, objType, cond); err != nil {
		t.Fatal(err)
	}
	afterFirst := testHookRenders.Load()

	// 相同形状（不同对象实例）→ 必须命中缓存且零渲染
	for i := 0; i < 8; i++ {
		if _, _, err := CachedRender(mysqlDial, table, elems, u, objType, cond); err != nil {
			t.Fatal(err)
		}
	}
	if got := testHookRenders.Load(); got != afterFirst {
		t.Fatalf("命中路径不应触发渲染: first=%d now=%d", afterFirst, got)
	}
}

// TestCondCacheEviction 容量上限的整体置换淘汰：超过 condCacheCap 后
// 新 map 生效，旧条目不可见且新表可回填命中。
func TestCondCacheEviction(t *testing.T) {
	sentinel := Condition{Type: EQ, Left: "__evict_probe__", Right: 1}
	key := condCacheKey{dname: "mysql", table: "evict_t", h: CondHash(sentinel)}

	// 局部替换全局表，避免污染其他用例
	prev := condMapPtr.Swap(new(sync.Map))
	defer func() {
		condMapPtr.Store(prev)
		condSize.Store(0)
	}()

	condSize.Store(int64(condCacheCap)) // 伪造已达上限
	old := condMapPtr.Load()
	condCachePut(key, "1 = 0")

	if cur := condMapPtr.Load(); cur == old {
		t.Fatal("超限后应整体置换为新 map")
	}
	if _, ok := condMapPtr.Load().Load(key); ok {
		t.Fatal("置换后旧 map 的写入应随置换丢弃")
	}

	condCachePut(key, "1 = 0")
	v, ok := condMapPtr.Load().Load(key)
	if !ok || v.(string) != "1 = 0" {
		t.Fatal("置换后的新表应可回填并命中")
	}
	sqlText, ok := condCacheGet(key)
	if !ok || sqlText != "1 = 0" {
		t.Fatalf("condCacheGet 应命中回填条目: %q %v", sqlText, ok)
	}
}

// TestInsertValues L1 值收集：跳过自增列且顺序与插入列一致。
func TestInsertValues(t *testing.T) {
	id := int64(7)
	name := "op"
	age := 3
	elems := []Elem{
		{Type: reflect.TypeOf(id), Tag: "id", Option: map[string]string{"autoIncrement": "-"}, AutoInc: true, Ptr: unsafe.Pointer(&id)},
		{Type: reflect.TypeOf(name), Tag: "name", Ptr: unsafe.Pointer(&name)},
		{Type: reflect.TypeOf(age), Tag: "age", Ptr: unsafe.Pointer(&age)},
	}
	vals := InsertValues(elems)
	if len(vals) != 2 || vals[0] != name || vals[1] != age {
		t.Fatalf("InsertValues 错误: %v", vals)
	}

	// 全自增：值集合为空，三方言 Insert 内联为空串
	allAI := []Elem{{Type: reflect.TypeOf(id), Tag: "id",
		Option: map[string]string{"autoIncrement": "-"}, AutoInc: true, Ptr: unsafe.Pointer(&id)}}
	if got := InsertValues(allAI); len(got) != 0 {
		t.Fatalf("全自增应为空: %v", got)
	}
	for _, d := range []Dialect{mysqlDial, postgresDial, sqliteDial} {
		ss := InternStmts(d, "empty_t", allAI)
		if ss.Insert != "" {
			t.Fatalf("%s 全自增 Insert 应为空串: %q", d.Name(), ss.Insert)
		}
	}
}

// TestInternStmts L1 内联语句集：内容与 Build* 一致、跨调用共享同一实例、并发安全。
func TestInternStmts(t *testing.T) {
	name := "n"
	age := 1
	elems := []Elem{
		{Type: reflect.TypeOf(name), Tag: "name", Ptr: unsafe.Pointer(&name)},
		{Type: reflect.TypeOf(age), Tag: "age", Ptr: unsafe.Pointer(&age)},
	}

	ss1 := InternStmts(sqliteDial, "intern_t", elems)
	wantInsert, _, err := BuildInsert(sqliteDial, "intern_t", elems)
	if err != nil {
		t.Fatal(err)
	}
	wantSelect, _, err := BuildSelect(sqliteDial, "intern_t", elems, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantCount, _, err := BuildCount(sqliteDial, "intern_t", elems, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ss1.Insert != wantInsert || ss1.SelectAll != wantSelect || ss1.CountAll != wantCount {
		t.Fatalf("内联语句与 Builder 输出不一致:\n%q|%q|%q vs %q|%q|%q",
			ss1.Insert, ss1.SelectAll, ss1.CountAll, wantInsert, wantSelect, wantCount)
	}

	if ss2 := InternStmts(sqliteDial, "intern_t", elems); ss1 != ss2 {
		t.Fatal("相同 (方言, 表) 应返回同一内联实例")
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s := InternStmts(sqliteDial, "intern_t", elems); s != ss1 {
				t.Error("并发构建返回了不同实例")
			}
		}()
	}
	wg.Wait()

	// 三方言语句形态抽查
	if !strings.HasPrefix(ss1.SelectAll, `SELECT "name","age" FROM "intern_t"`) {
		t.Fatalf("SelectAll 形态错误: %q", ss1.SelectAll)
	}
	if !strings.Contains(ss1.CountAll, "COUNT(*)") {
		t.Fatalf("CountAll 形态错误: %q", ss1.CountAll)
	}
	pgSS := InternStmts(postgresDial, "intern_pg_t", elems)
	if !strings.Contains(pgSS.Insert, "$1") {
		t.Fatalf("PG 插入语句应含 $1 占位符: %q", pgSS.Insert)
	}
	mySS := InternStmts(mysqlDial, "intern_my_t", elems)
	if !strings.Contains(mySS.Insert, "`intern_my_t`") {
		t.Fatalf("MySQL 插入语句应含反引号表名: %q", mySS.Insert)
	}
}

// TestCollectBindValuesErrorParity 收集器镜像渲染器的错误分支：
// 渲染失败的非法条件在缓存路径同样失败且错误类别一致。
func TestCollectBindValuesErrorParity(t *testing.T) {
	bad := []Condition{
		{Type: EQ, Left: 123, Right: 1},     // 比较条件缺字符串字段名
		{Type: EQ, Left: "age", Right: nil}, // 缺比较值
		{Type: IN, Left: "age", Right: 42},  // IN 值非切片
		{Type: BETWEEN, Left: "age", Args: []any{1}},
		{Type: LIMIT, Args: []any{1, 2, 3}}, // arity 非法
		{Type: CUSTOM, Left: 42},            // 自定义缺 SQL 文本
		{Type: AND, Args: []any{42}},        // 组合子项非法
		{Type: NOT, Left: 42},               // NOT 子项非法
		{Type: IN_SUBQUERY, Left: 42, Right: "SELECT 1"},
		{Type: EXISTS, Left: 42},
		{Type: IN_VALUES, Left: "age"}, // 缺值列表
		{Type: ConditionType(999)},     // 未知类型
	}
	u := &m4RefObj{}
	elems := m4Elems(u)
	objType := reflect.TypeOf(m4RefObj{})
	for i, c := range bad {
		_, rargs, rerr := RenderCondition(elems, nil, objType, c)
		_, cargs, cerr := CachedRender(mysqlDial, "p_err", elems, nil, objType, c)
		if rerr == nil || cerr == nil {
			t.Fatalf("用例 %d: 两路径均应报错 (render=%v cached=%v)", i, rerr, cerr)
		}
		if errors.Is(rerr, ErrUnsupportedCondition) != errors.Is(cerr, ErrUnsupportedCondition) {
			t.Fatalf("用例 %d: ErrUnsupportedCondition 类别不一致: %v vs %v", i, rerr, cerr)
		}
		if errors.Is(rerr, ErrInvalidCondition) != errors.Is(cerr, ErrInvalidCondition) {
			t.Fatalf("用例 %d: ErrInvalidCondition 类别不一致: %v vs %v", i, rerr, cerr)
		}
		_ = rargs
		_ = cargs
	}
}
