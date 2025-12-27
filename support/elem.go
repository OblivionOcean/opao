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

type Elem struct {
	Index  int
	Type   reflect.Type
	Tag    string
	Ptr    unsafe.Pointer
	Offset uintptr
	Option map[string]string
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
	if elem.Type != reflect.TypeOf(val) {
		return errors.New("type mismatch")
	}
	ptr := elem.Ptr
	switch elem.Type.Kind() {
	case reflect.Bool:
		*(*bool)(ptr) = val.(bool)
	case reflect.Int:
		*(*int)(ptr) = val.(int)
	case reflect.String:
		*(*string)(ptr) = val.(string)
	case reflect.Uint:
		*(*uint)(ptr) = val.(uint)
	case reflect.Float64:
		*(*float64)(ptr) = val.(float64)
	case reflect.Float32:
		*(*float32)(ptr) = val.(float32)
	case reflect.Uint64:
		*(*uint64)(ptr) = val.(uint64)
	case reflect.Uint32:
		*(*uint32)(ptr) = val.(uint32)
	case reflect.Uint16:
		*(*uint16)(ptr) = val.(uint16)
	case reflect.Uint8:
		*(*uint8)(ptr) = val.(uint8)
	case reflect.Int64:
		*(*int64)(ptr) = val.(int64)
	case reflect.Int32:
		*(*int32)(ptr) = val.(int32)
	case reflect.Int16:
		*(*int16)(ptr) = val.(int16)
	case reflect.Int8:
		*(*int8)(ptr) = val.(int8)
	case reflect.Complex64:
		*(*complex64)(ptr) = val.(complex64)
	case reflect.Complex128:
		*(*complex128)(ptr) = val.(complex128)
	case reflect.Uintptr:
		*(*uintptr)(ptr) = val.(uintptr)
	case reflect.Slice:
		*(*[]byte)(ptr) = val.([]byte)
	}

	if reflect.TypeOf(*(*time.Time)(nil)) == elem.Type || reflect.TypeOf((*time.Time)(nil)) == elem.Type && (*time.Time)(ptr) != nil {
		*(*time.Time)(ptr) = val.(time.Time)
	}

	reflect.NewAt(elem.Type, ptr).Elem().Set(reflect.ValueOf(val))
	return nil
}

func (elem *Elem) GetInterface() any {
	if elem.Type.Kind()&flagAddr != 0 {
		return reflect.NewAt(elem.Type, elem.Ptr).Interface()
	}
	return nil
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
