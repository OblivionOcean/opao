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
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"
	"unsafe"
)

//go:inline
func String2Slice(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

//go:inline
func Bytes2String(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

//go:inline
func Byte2String(b byte) string {
	return unsafe.String(&b, 1)
}

//go:inline
func BytesCombine2(pBytes ...[]byte) []byte {
	return bytes.Join(pBytes, String2Slice(""))
}

func ContainsInSlice(items []string, item string) bool {
	itemsLength := len(items)
	for i := 0; i < itemsLength; i++ {
		if items[i] == item {
			return true
		}
	}
	return false
}

//go:inline
func Strings2Ints(strs []string) []int {
	ints := []int{}
	for i := 0; i < len(strs); i++ {
		t, _ := strconv.Atoi(strs[i])
		ints = append(ints, t)
	}
	return ints
}

func Ints2Strings(ints []int) []string {
	strs := []string{}
	for i := 0; i < len(ints); i++ {
		strs = append(strs, strconv.Itoa(ints[i]))
	}
	return strs
}

func ToString(src any) string {
	switch s := src.(type) {
	case nil:
		return ""
	case string:
		return s
	case byte:
		bs := []byte{s}
		return *(*string)(unsafe.Pointer(&bs))
	case int, int8, int16, int32, int64, uint, uint16, uint32, uint64:
		switch i := src.(type) {
		case int:
			return strconv.FormatInt(int64(i), 10)
		case int8:
			return strconv.FormatInt(int64(i), 10)
		case int16:
			return strconv.FormatInt(int64(i), 10)
		case int32:
			return strconv.FormatInt(int64(i), 10)
		case int64:
			return strconv.FormatInt(i, 10)
		case uint:
			return strconv.FormatUint(uint64(i), 10)
		case uint8:
			return strconv.FormatUint(uint64(i), 10)
		case uint16:
			return strconv.FormatUint(uint64(i), 10)
		case uint32:
			return strconv.FormatUint(uint64(i), 10)
		case uint64:
			return strconv.FormatUint(i, 10)
		}
	case float32, float64:
		switch f := src.(type) {
		case float32:
			return strconv.FormatFloat(float64(f), 'f', -1, 32)
		case float64:
			return strconv.FormatFloat(float64(f), 'f', -1, 64)
		}

	case bool:
		return strconv.FormatBool(s)
	case error:
		if s != nil {
			return s.Error()
		} else {
			return ""
		}
	case reflect.Value:
		src = s.Interface()
		return ToString(src)
	case time.Time:
		return s.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return s.String()
	case io.Reader:
		byt, e := io.ReadAll(s)
		if e != nil {
			panic(e)
		} else {
			return ToString(byt)
		}
	case []byte:
		byts := s
		return *(*string)(unsafe.Pointer(&byts))
	case []any:
		str := ""
		ls := s
		for k := 0; k < len(ls); k++ {
			str += ", " + ToString(ls[k])
		}
		if len(str) > 2 {
			return str[2:]
		}
		return str
	case any, *any:
		sv := reflect.ValueOf(src)
		if sv.Kind() == reflect.Ptr {
			return ToString(sv.Elem().Interface())
		} else if sv.Kind() == reflect.Slice {
			return ToString(src.([]byte))
		} else if sv.Kind() == reflect.Map {
			mapKeys := sv.MapKeys()
			mapKeysLength := len(mapKeys)
			tmp := "{"
			for i := 0; i < mapKeysLength; i++ {
				key := ToString(mapKeys[i])
				tmp += key + ": " + ToString(sv.MapIndex(mapKeys[i]).Interface()) + ", "
			}
			return tmp + "}"
		} else {
			return "<Type " + sv.Type().String() + ">"
		}
	}
	return ""
}

func ToBytes(src any) []byte {
	switch s := src.(type) {
	case nil:
		return nil
	case string:
		// Zero-copy read-only conversion; do NOT mutate the returned slice.
		return String2Slice(s)
	case byte:
		return []byte{s}
	case int, int8, int16, int32, int64, uint, uint16, uint32, uint64:
		byts := []byte{}
		switch i := src.(type) {
		case int:
			return strconv.AppendInt(byts, int64(i), 10)
		case int8:
			return strconv.AppendInt(byts, int64(i), 10)
		case int16:
			return strconv.AppendInt(byts, int64(i), 10)
		case int32:
			return strconv.AppendInt(byts, int64(i), 10)
		case int64:
			return strconv.AppendInt(byts, i, 10)
		case uint:
			return strconv.AppendUint(byts, uint64(i), 10)
		case uint8:
			return strconv.AppendUint(byts, uint64(i), 10)
		case uint16:
			return strconv.AppendUint(byts, uint64(i), 10)
		case uint32:
			return strconv.AppendUint(byts, uint64(i), 10)
		case uint64:
			return strconv.AppendUint(byts, uint64(i), 10)
		}
	case float32, float64:
		byts := []byte{}
		switch f := src.(type) {
		case float32:
			return strconv.AppendFloat(byts, float64(f), 'f', -1, 32)
		case float64:
			return strconv.AppendFloat(byts, float64(f), 'f', -1, 64)
		}
	case bool:
		byts := []byte{}
		return strconv.AppendBool(byts, s)
	case error:
		if s != nil {
			return ToBytes(s.Error())
		} else {
			return nil
		}
	case reflect.Value:
		src = s.Interface()
		return ToBytes(src)
	case fmt.Stringer:
		return ToBytes(s.String())
	case io.Reader:
		byt, e := io.ReadAll(s)
		if e != nil {
			panic(e)
		} else {
			return byt
		}
	case time.Time:
		return ToBytes(s.Format(time.RFC3339Nano))
	case sql.RawBytes:
		return s
	case []byte:
		return s
	case []any:
		byts := []byte{}
		ls := s
		for k := 0; k < len(ls); k++ {
			byts = append(byts, ToBytes(ls[k])...)
		}
		return byts
	case any, *any:
		sv := reflect.ValueOf(src)
		if sv.Kind() == reflect.Ptr {
			return ToBytes(sv.Elem().Interface())
		} else if sv.Kind() == reflect.Slice {
			return s.([]byte)
		} else if sv.Kind() == reflect.Map {
			return ToBytes(ToString(s))
		} else {
			return ToBytes(s)
		}
	}
	return nil
}

func AutotAnyI64tAnyI(Val any) any {
	switch Val.(type) {
	case int64:
		return int(Val.(int64))
	case uint64:
		return uint(Val.(uint64))
	}
	// 默认情况下返回零值
	return Val
}

func Used(...any) {}

func GetSlicesSize(slice [][]byte) int {
	sliceLength := len(slice)
	size := 0
	if sliceLength == 0 {
		return 0
	}
	for i := 0; i < sliceLength; i++ {
		size += len(slice[i])
	}
	return size
}

func SplitStringByByte(s string, sep byte) []string {
	bs := String2Slice(s)
	result := make([]string, 0, CountByte(bs, sep)+1)
	start := 0
	for i := 0; i < len(bs); i++ {
		if bs[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	// Append the final segment
	result = append(result, s[start:])
	return result
}

func CountByte(s []byte, sep byte) int {
	n := 0
	for {
		i := bytes.IndexByte(s, sep)
		if i == -1 {
			return n
		}
		n++
		s = s[i+1:]
	}
}
