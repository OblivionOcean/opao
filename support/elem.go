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
	"reflect"
	"time"
	"unsafe"
)

const (
	flagAddr = 1 << 8
)

// OptionFlag 位标志类型，用于高效存储字段选项
type OptionFlag uint64

const (
	// 字段选项位标志
	OptAutoInc    OptionFlag = 1 << 0 // 自增
	OptPrimaryKey OptionFlag = 1 << 1 // 主键
	OptNotNull    OptionFlag = 1 << 2 // 非空
	OptUnique     OptionFlag = 1 << 3 // 唯一
	OptIndexed    OptionFlag = 1 << 4 // 索引
	OptHasDefault OptionFlag = 1 << 5 // 有默认值
)

type Elem struct {
	Index  int
	Type   reflect.Type
	Tag    string
	Ptr    unsafe.Pointer
	Offset uintptr
	Flags  OptionFlag  // 位标志：存储字段选项
	DefaultValue string // 默认值字符串，空字符串表示无默认值

	// AutoInc 预计算的自增标记：热路径避免反复读取 Flags。
	// 由 Register 写入；与 Flags&OptAutoInc != 0 语义一致。
	AutoInc bool

	// PrimaryKey 预计算的主键标记：由 Register 写入。
	PrimaryKey bool

	// NotNull 预计算的非空标记：由 Register 写入。
	NotNull bool
}

// HasFlag 检查是否设置了指定的选项标志
func (e *Elem) HasFlag(flag OptionFlag) bool {
	return e.Flags&flag != 0
}

// SetFlag 设置选项标志
func (e *Elem) SetFlag(flag OptionFlag) {
	e.Flags |= flag
}

// ClearFlag 清除选项标志
func (e *Elem) ClearFlag(flag OptionFlag) {
	e.Flags &^= flag
}

type Cache struct {
	Elems   []Elem
	Table   string
	ObjType reflect.Type
}

func (elem *Elem) Get() any {
	// fastest way to get unexp value from reflect.Value
	ptr := elem.Ptr
	switch elem.Type.Kind() {
	case reflect.Bool:
		return *(*bool)(ptr)
	case reflect.Int:
		return *(*int)(ptr)
	case reflect.String:
		return *(*string)(ptr)
	case reflect.Uint:
		return *(*uint)(ptr)
	case reflect.Float64:
		return *(*float64)(ptr)
	case reflect.Float32:
		return *(*float32)(ptr)
	case reflect.Uint64:
		return *(*uint64)(ptr)
	case reflect.Uint32:
		return *(*uint32)(ptr)
	case reflect.Uint16:
		return *(*uint16)(ptr)
	case reflect.Uint8:
		return *(*uint8)(ptr)
	case reflect.Int64:
		return *(*int64)(ptr)
	case reflect.Int32:
		return *(*int32)(ptr)
	case reflect.Int16:
		return *(*int16)(ptr)
	case reflect.Int8:
		return *(*int8)(ptr)
	case reflect.Complex64:
		return *(*complex64)(ptr)
	case reflect.Complex128:
		return *(*complex128)(ptr)
	case reflect.Uintptr:
		return *(*uintptr)(ptr)
	case reflect.Slice:
		return *(*[]byte)(ptr)
	}

	if reflect.TypeOf((*time.Time)(nil)).Elem() == elem.Type || reflect.TypeOf((*time.Time)(nil)) == elem.Type && (*time.Time)(ptr) != nil {
		return *(*time.Time)(ptr)
	}

	return reflect.NewAt(elem.Type, ptr).Elem().Interface()
}

func (elem *Elem) Set(val any) error {
	if val == nil || elem.Type != reflect.TypeOf(val) {
		return errors.New("type mismatch")
	}
	ptr := elem.Ptr
	switch elem.Type.Kind() {
	case reflect.Bool:
		*(*bool)(ptr) = val.(bool)
	case reflect.Int:
		*(*int)(ptr) = val.(int)
	case reflect.Int8:
		*(*int8)(ptr) = val.(int8)
	case reflect.Int16:
		*(*int16)(ptr) = val.(int16)
	case reflect.Int32:
		*(*int32)(ptr) = val.(int32)
	case reflect.Int64:
		*(*int64)(ptr) = val.(int64)
	case reflect.Uint:
		*(*uint)(ptr) = val.(uint)
	case reflect.Uint8:
		*(*uint8)(ptr) = val.(uint8)
	case reflect.Uint16:
		*(*uint16)(ptr) = val.(uint16)
	case reflect.Uint32:
		*(*uint32)(ptr) = val.(uint32)
	case reflect.Uint64:
		*(*uint64)(ptr) = val.(uint64)
	case reflect.Float32:
		*(*float32)(ptr) = val.(float32)
	case reflect.Float64:
		*(*float64)(ptr) = val.(float64)
	case reflect.String:
		*(*string)(ptr) = val.(string)
	case reflect.Slice:
		*(*[]byte)(ptr) = val.([]byte)
	default:
		// time.Time 及其他未覆盖类型走反射兜底（仅写一次）
		reflect.NewAt(elem.Type, ptr).Elem().Set(reflect.ValueOf(val))
	}
	return nil
}

func (elem *Elem) GetInterface() any {
	return reflect.NewAt(elem.Type, elem.Ptr).Interface()
}

func (elem *Elem) Zero() bool {
	// fastest way to get unexp value from reflect.Value
	ptr := elem.Ptr
	switch elem.Type.Kind() {
	case reflect.Bool:
		return *(*bool)(ptr) == false
	case reflect.Int:
		return *(*int)(ptr) == 0
	case reflect.String:
		return *(*string)(ptr) == ""
	case reflect.Uint:
		return *(*uint)(ptr) == 0
	case reflect.Float64:
		return *(*float64)(ptr) == 0
	case reflect.Float32:
		return *(*float32)(ptr) == 0
	case reflect.Uint64:
		return *(*uint64)(ptr) == 0
	case reflect.Uint32:
		return *(*uint32)(ptr) == 0
	case reflect.Uint16:
		return *(*uint16)(ptr) == 0
	case reflect.Uint8:
		return *(*uint8)(ptr) == 0
	case reflect.Int64:
		return *(*int64)(ptr) == 0
	case reflect.Int32:
		return *(*int32)(ptr) == 0
	case reflect.Int16:
		return *(*int16)(ptr) == 0
	case reflect.Int8:
		return *(*int8)(ptr) == 0
	case reflect.Complex64:
		return *(*complex64)(ptr) == 0
	case reflect.Complex128:
		return *(*complex128)(ptr) == 0
	case reflect.Uintptr:
		return *(*uintptr)(ptr) == 0
	case reflect.Slice:
		return len(*(*[]byte)(ptr)) == 0
	}

	return reflect.NewAt(elem.Type, ptr).Elem().IsZero()
}
