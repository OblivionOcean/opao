package support

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"unsafe"

	"github.com/OblivionOcean/opao/internal/runtime"
	"github.com/OblivionOcean/opao/utils"
)

type ORM struct {
	objectORM func(*sql.DB, any, reflect.Type, string, []Elem, error) ObjectORM
	caches    *SafeCache // map[reflect.Type]Cache
	conn      *sql.DB
}

type ObjectORM interface {
	Error() error
	Create() error
	Update(args ...any) error
	Delete(args ...any) error
	Find(args ...any) (any, error)
	FindAll(args ...any) ([]any, error)
}

type Elem struct {
	Index  int
	Type   reflect.Type
	Tag    string
	Value  reflect.Value
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
	ptr := unsafe.Pointer(elem.Value.UnsafeAddr())
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
	case reflect.Invalid:
		return nil
	case reflect.Ptr, reflect.Chan:
		return elem.Value.Pointer()
	}
	if elem.Value.CanInterface() {
		return elem.Value.Interface()
	}
	return reflect.NewAt(elem.Type, ptr).Elem().Interface()
}

func (elem *Elem) Set(val any) error {
	if elem.Type != reflect.TypeOf(val) {
		return errors.New("type mismatch")
	}
	ptr := unsafe.Pointer(elem.Value.UnsafeAddr())
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
		return nil
	case reflect.Uint32:
		*(*uint32)(ptr) = val.(uint32)
		return nil
	case reflect.Uint16:
		*(*uint16)(ptr) = val.(uint16)
		return nil
	case reflect.Uint8:
		*(*uint8)(ptr) = val.(uint8)
		return nil
	case reflect.Int64:
		*(*int64)(ptr) = val.(int64)
		return nil
	case reflect.Int32:
		*(*int32)(ptr) = val.(int32)
		return nil
	case reflect.Int16:
		*(*int16)(ptr) = val.(int16)
		return nil
	case reflect.Int8:
		*(*int8)(ptr) = val.(int8)
		return nil
	case reflect.Complex64:
		*(*complex64)(ptr) = val.(complex64)
		return nil
	case reflect.Complex128:
		*(*complex128)(ptr) = val.(complex128)
		return nil
	case reflect.Uintptr:
		*(*uintptr)(ptr) = val.(uintptr)
	case reflect.Invalid, reflect.Ptr, reflect.Chan:
		return nil
	}
	if elem.Value.CanInterface() {
		elem.Value.Set(reflect.ValueOf(val))
	}
	reflect.NewAt(elem.Type, ptr).Elem().Set(reflect.ValueOf(val))
	return nil
}

func (orm *ORM) Init(conn *sql.DB, driver func(*sql.DB, any, reflect.Type, string, []Elem, error) ObjectORM) {
	orm.objectORM = driver
	orm.conn = conn
	orm.caches = &SafeCache{cache: map[reflect.Type]Cache{}}
}

func (o *ORM) Register(tableName string, object any) error {
	objType := reflect.TypeOf(object)
	if objType.Kind() == reflect.Ptr {
		objType = reflect.TypeOf(object).Elem()
	}
	objTypePtr := runtime.Type2StructType(objType)
	if objTypePtr == nil {
		return errors.New("object must be a struct or a pointer to a struct")
	}
	numIndex := runtime.TypeFieldLen(objTypePtr)
	elems := make([]Elem, numIndex)
	field := &reflect.StructField{}
	for i := 0; i < numIndex; i++ {
		runtime.GetField(field, objTypePtr, i)
		tagName, ok := runtime.GetTag(field.Tag, "db")
		option, okOption := runtime.GetTag(field.Tag, "option")
		if !ok && (okOption && option != "-" && option != "") {
			ok = true
			tagName = field.Name
		}
		if !ok || tagName == "-" || tagName == "" || field.Type.Kind() == reflect.Invalid || field.Type.Kind() == reflect.Func || field.Type.Kind() == reflect.Chan || field.Type.Kind() == reflect.UnsafePointer || field.Type.Kind() == reflect.Map || field.Type.Kind() == reflect.Interface || field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array || field.Type.Kind() == reflect.Ptr || field.Type.Kind() == reflect.Struct {
			numIndex--
			continue
		}
		if okOption && option != "" && option != "-" {
			optMap := make(map[string]string, utils.CountByte(utils.String2Slice(option), ';')+1)
			tmp := utils.SplitStringByByte(option, ';')
			for i := 0; i < len(tmp); i++ {
				opt := tmp[i][:strings.IndexByte(tmp[i], '=')]
				val := tmp[i][strings.IndexByte(tmp[i], '=')+1:]
				if opt != "" {
					optMap[opt] = val
				}
			}
		}
		elems[i].Index = i
		elems[i].Type = field.Type
		elems[i].Tag = tagName
		elems[i].Offset = field.Offset
	}
	o.caches.Store(objType, Cache{
		Elems:   elems[:numIndex],
		Table:   tableName,
		ObjType: objType,
	})
	return nil
}

func (o *ORM) Load(object any) (orm ObjectORM) {
	objType := reflect.TypeOf(object)
	objValue := reflect.ValueOf(object)
	if objType.Kind() == reflect.Ptr {
		ptr := objValue.Elem()
		objType = ptr.Type()
		objValue = ptr
	} else {
		orm = o.objectORM(nil, nil, nil, "", nil, errors.New("object must be a pointer to a struct"))
		return
	}
	if rawCache, ok := o.caches.Load(objType); ok {
		cache := rawCache
		ElemsLength := len(cache.Elems)
		for i := 0; i < ElemsLength; i++ {
			cache.Elems[i].Value = objValue.Field(cache.Elems[i].Index)
		}
		orm = o.objectORM(o.conn, object, cache.ObjType, cache.Table, cache.Elems, nil)
		return
	} else {
		orm = o.objectORM(nil, nil, nil, "", nil, errors.New("object not registered"))
		return
	}
}

type SafeCache struct {
	mu    sync.RWMutex
	cache map[reflect.Type]Cache
}

//go:inline
func NewSafeCache() *SafeCache {
	return &SafeCache{
		cache: make(map[reflect.Type]Cache),
	}
}

//go:inline
func (sc *SafeCache) Load(key reflect.Type) (value Cache, ok bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	value, ok = sc.cache[key]
	return
}

// go::inline
func (sc *SafeCache) Store(key reflect.Type, value Cache) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[key] = value
}

func WriteLii(elems []Elem, r sql.Result) {
	elemsLeng := len(elems)
	lii, liie := r.LastInsertId()
	if liie == nil {
		for i := 0; i < elemsLeng; i++ {
			elem := elems[i]
			if elem.Option["autoIncrement"] == "-" {
				elems[i].Value.Set(reflect.ValueOf(utils.AutotAnyI64tAnyI(lii)))
			}
		}
	}
}
