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

package opao_test

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/OblivionOcean/opao/internal/runtime"
	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/utils"
)

type regUser struct {
	ID   int64  `db:"id" option:"autoIncrement"`
	Name string `db:"name"`
	Age  int    `db:"age"`
	hide string `db:"-"`
}

// captureDriver 记录 Load 时驱动收到的元数据，用于无数据库验证注册/加载链路
type captureDriver struct {
	elems   []support.Elem
	table   string
	objType reflect.Type
}

type stubORM struct{ err error }

func (s *stubORM) Error() error                       { return s.err }
func (s *stubORM) Create() error                      { return nil }
func (s *stubORM) Update(args ...any) error           { return nil }
func (s *stubORM) Save(args ...any) error             { return nil }
func (s *stubORM) Delete(args ...any) error           { return nil }
func (s *stubORM) Find(args ...any) (any, error)      { return nil, nil }
func (s *stubORM) FindAll(args ...any) ([]any, error) { return nil, nil }
func (s *stubORM) Count(args ...any) (int, error)     { return 0, nil }

func newCaptureORM() (*support.ORM, *captureDriver) {
	captured := &captureDriver{}
	orm := &support.ORM{}
	orm.Init(nil, func(_ *sql.DB, obj any, objType reflect.Type, table string, elems []support.Elem, err error) support.ObjectORM {
		captured.elems, captured.table, captured.objType = elems, table, objType
		return &stubORM{err: err}
	})
	return orm, captured
}

func TestRegisterAndLoadMetadata(t *testing.T) {
	orm, captured := newCaptureORM()
	if err := orm.Register("user", &regUser{}); err != nil {
		t.Fatalf("Register 失败: %v", err)
	}
	u := &regUser{ID: 1, Name: "张三", Age: 25}
	o := orm.Load(u)
	if o.Error() != nil {
		t.Fatalf("Load 失败: %v", o.Error())
	}

	if captured.table != "user" || captured.objType != reflect.TypeOf(regUser{}) {
		t.Fatalf("表名或类型不符: table=%s type=%v", captured.table, captured.objType)
	}
	want := map[string]bool{"id": true, "name": true, "age": true}
	if len(captured.elems) != len(want) {
		t.Fatalf("字段数不符: got %d (%v)", len(captured.elems), captured.elems)
	}
	for _, e := range captured.elems {
		if !want[e.Tag] {
			t.Fatalf("意外字段: %s", e.Tag)
		}
		if e.Option["autoIncrement"] == "-" && e.Tag != "id" {
			t.Fatalf("autoIncrement 标记错位: %s", e.Tag)
		}
	}
}

func TestLoadUnregisteredObject(t *testing.T) {
	orm, _ := newCaptureORM()
	type Other struct {
		X int `db:"x"`
	}
	o := orm.Load(&Other{})
	if o.Error() == nil {
		t.Fatal("未注册对象应当返回错误")
	}
}

func TestLoadNonPointerRejected(t *testing.T) {
	orm, _ := newCaptureORM()
	if err := orm.Register("user", &regUser{}); err != nil {
		t.Fatal(err)
	}
	o := orm.Load(regUser{})
	if o.Error() == nil {
		t.Fatal("非指针对象应当返回错误")
	}
}

func TestRuntimeGetTag(t *testing.T) {
	type probe struct {
		Name string `db:"user_name" option:"autoIncrement"`
		Age  int
	}
	st := runtime.Type2StructType(reflect.TypeOf(probe{}))
	if st == nil {
		t.Fatal("Type2StructType 返回 nil")
	}
	var f reflect.StructField
	if !runtime.GetField(&f, st, 0) {
		t.Fatal("GetField 失败")
	}
	if f.Name != "Name" {
		t.Fatalf("字段名不符: %s", f.Name)
	}
	tag, ok := runtime.GetTag(f.Tag, "db")
	if !ok || tag != "user_name" {
		t.Fatalf("db 标签解析错误: %q %v", tag, ok)
	}
	opt, ok := runtime.GetTag(f.Tag, "option")
	if !ok || opt != "autoIncrement" {
		t.Fatalf("option 标签解析错误: %q %v", opt, ok)
	}
	if _, ok := runtime.GetTag(f.Tag, "json"); ok {
		t.Fatal("不存在的标签不应返回 ok")
	}
}

func TestBufferTruncateLastAndString(t *testing.T) {
	b := utils.NewBuffer(16)
	b.WriteString("a,b,")
	b.TruncateLast(1)
	if got := b.String(); got != "a,b" {
		t.Fatalf("TruncateLast 结果错误: %q", got)
	}
	b.TruncateLast(99) // 超长截断应为安全 no-op
	if b.String() != "a,b" {
		t.Fatalf("超长截断不应破坏内容: %q", b.String())
	}
}
