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

package utils

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/OblivionOcean/opao/support"
)

const (
	wordSize = unsafe.Sizeof(uintptr(0)) // 获取指针大小
)

// ParseConditionArg 解析条件参数
func ParseConditionArg(obj any, objType reflect.Type, Elems []support.Elem, arg any) (string, any) {
	if arg == nil {
		panic("ParseConditionArg: argument cannot be nil")
	}

	switch v := arg.(type) {
	case support.Condition:
		return "", arg // 返回Condition本身
	case string:
		for i := 0; i < len(Elems); i++ {
			if Elems[i].Tag == v {
				return v, Elems[i].Get()
			}
		}
	default:
		argType := reflect.TypeOf(arg)
		if argType.Kind() == reflect.Ptr {
			objPtr := *(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(&obj)) + wordSize))
			argPtr := *(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(&arg)) + wordSize))

			for i := 0; i < len(Elems); i++ {
				elem := Elems[i]
				if elem.Offset == argPtr-objPtr {
					return elem.Tag, Elems[i].Get()
				}
			}
		} else if argType.Kind() == reflect.Struct && argType == objType {
			return "", arg
		} else {
			fmt.Println("ParseConditionArg: Input Type:", reflect.TypeOf(arg))
			panic("Invalid argument type, please use pointer or struct")
		}
	}
	return "", nil
}

func GetValueNum(cond support.Condition) (size int) {
	if cond.Left != nil {
		size++
	}
	if cond.Right != nil {
		size++
	}
	for i := 0; i < len(cond.Args); i++ {
		arg := cond.Args[i]
		if arg == nil {
			continue
		}
		if _, ok := arg.(support.Condition); ok {
			size += GetValueNum(arg.(support.Condition))
		} else {
			size++
		}
	}
	return size
}
