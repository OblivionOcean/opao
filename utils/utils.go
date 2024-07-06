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

func String2Slice(s string) (b []byte) {
	type StringHeader struct {
		Data uintptr
		Len  int
	}
	type SliceHeader struct {
		Data uintptr
		Len  int
		Cap  int
	}
	bh := (*SliceHeader)(unsafe.Pointer(&b))
	sh := (*StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return
}

func Bytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func Byte2String(b byte) string {
	bs := []byte{b}
	return *(*string)(unsafe.Pointer(&bs))
}

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
		type StringHeader struct {
			Data uintptr
			Len  int
		}
		type SliceHeader struct {
			Data uintptr
			Len  int
			Cap  int
		}
		str := s
		byts := []byte{}
		bh := (*SliceHeader)(unsafe.Pointer(&str))
		sh := (*StringHeader)(unsafe.Pointer(&byts))
		bh.Data = sh.Data
		bh.Len = sh.Len
		bh.Cap = sh.Len
		return byts
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

func ValIsNil(val reflect.Value) bool {
	if !val.IsValid() {
		return true
	} else if val.CanFloat() || val.CanInt() || val.CanUint() || val.CanComplex() {
		if val.IsZero() {
			return true
		}
	} else if val.Kind() == reflect.String {
		if val.String() == "" {
			return true
		}
	} else if val.IsNil() {
		return true
	}
	return false
}
