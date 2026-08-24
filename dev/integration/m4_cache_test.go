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
	"testing"

	_ "modernc.org/sqlite"

	"github.com/OblivionOcean/opao"
)

// newM4DB 创建 M4 缓存场景专用的 sqlite 内存库（独立 DSN 隔离数据）
func newM4DB(t *testing.T, name string) *opao.Database {
	t.Helper()
	db, err := opao.NewDatabase("sqlite", "file:"+name+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.Conn.SetMaxOpenConns(1)
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

// TestM4CreateInternedInsert L1 生效：Create 走内联 INSERT 语句，
// 连续两次插入均成功且自增主键逐次回填（1、2），值列与占位符对齐。
func TestM4CreateInternedInsert(t *testing.T) {
	db := newM4DB(t, "opao_dev_m4_create")

	a := &User{Name: "张三", Age: 25}
	if err := db.Load(a).Create(); err != nil {
		t.Fatalf("第一次 Create: %v", err)
	}
	if a.Id != 1 {
		t.Fatalf("首次自增回填应为 1，得到 %d", a.Id)
	}

	b := &User{Name: "李四", Age: 30}
	if err := db.Load(b).Create(); err != nil {
		t.Fatalf("第二次 Create（复用内联语句）: %v", err)
	}
	if b.Id != 2 {
		t.Fatalf("第二次自增回填应为 2，得到 %d", b.Id)
	}

	n, err := db.Load(&User{}).Count()
	if err != nil || n != 2 {
		t.Fatalf("计数应为 2: %v n=%d", err, n)
	}
}

// TestM4FindAllDifferentValues L2 生效：Eq("age",25) 与 Eq("age",30)
// 共享同一 SQL 结构（值走绑定参数），各自返回正确的行集。
func TestM4FindAllDifferentValues(t *testing.T) {
	db := newM4DB(t, "opao_dev_m4_findall")

	fixtures := []*User{
		{Name: "a", Age: 25},
		{Name: "b", Age: 30},
		{Name: "c", Age: 25},
		{Name: "d", Age: 40},
	}
	for _, f := range fixtures {
		if err := db.Load(f).Create(); err != nil {
			t.Fatal(err)
		}
	}

	assertAge := func(age int, want int) {
		t.Helper()
		rows, err := db.Load(&User{}).FindAll(opao.Eq("age", age))
		if err != nil {
			t.Fatalf("FindAll(Eq(\"age\",%d)): %v", age, err)
		}
		if len(rows) != want {
			t.Fatalf("age=%d 应命中 %d 行，得到 %d", age, want, len(rows))
		}
		for _, r := range rows {
			if got := r.(User).Age; got != age {
				t.Fatalf("结果行 age=%d 与条件不符", got)
			}
		}
	}
	assertAge(25, 2) // 首次渲染并缓存
	assertAge(30, 1) // 共享同一 SQL 文本，仅参数不同
	assertAge(25, 2) // 再次命中缓存路径

	if n, err := db.Load(&User{}).Count(opao.Eq("age", 25)); err != nil || n != 2 {
		t.Fatalf("Count(Eq) 缓存路径: %v n=%d", err, n)
	}
}

// TestM4UnconditionedFastPath 无条件 FindAll/Count 走 L1 内联快路径：
// 结果与常规路径一致。
func TestM4UnconditionedFastPath(t *testing.T) {
	db := newM4DB(t, "opao_dev_m4_fast")

	for _, name := range []string{"x", "y", "z"} {
		if err := db.Load(&User{Name: name}).Create(); err != nil {
			t.Fatal(err)
		}
	}

	all, err := db.Load(&User{}).FindAll()
	if err != nil {
		t.Fatalf("无条件 FindAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("全表应返回 3 行，得到 %d", len(all))
	}
	names := map[string]bool{}
	for _, r := range all {
		names[r.(User).Name] = true
	}
	if !names["x"] || !names["y"] || !names["z"] {
		t.Fatalf("全表行集缺失: %v", names)
	}

	n, err := db.Load(&User{}).Count()
	if err != nil || n != 3 {
		t.Fatalf("无条件 Count: %v n=%d", err, n)
	}

	// 快路径与带条件路径交叉执行互不干扰
	if n2, err := db.Load(&User{}).Count(opao.Eq("name", "y")); err != nil || n2 != 1 {
		t.Fatalf("L2 条件路径: %v n=%d", err, n2)
	}
	if n3, err := db.Load(&User{}).Count(); err != nil || n3 != 3 {
		t.Fatalf("回到 L1 快路径: %v n=%d", err, n3)
	}
}
