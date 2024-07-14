package support

import (
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"unsafe"

	"github.com/OblivionOcean/opao/internal/runtime"
)

type ORM struct {
	objectORM func(*sql.DB, any, reflect.Type, string, []Elem, error) ObjectORM
	caches    *SafeCache // map[reflect.Type]Cache
	conn      *sql.DB
}

type ObjectORM interface {
	Error() error
	Create() error
	Update(queryString string, queryValue ...any) error
	Delete(queryString string, queryValue ...any) error
	Find(queryString string, queryValue ...any) (any, error)
	FindAll(queryString string, queryValue ...any) ([]any, error)
}

type Elem struct {
	Index int
	Type  reflect.Type
	Tag   string
	Value uintptr
}

type Cache struct {
	Elems   []Elem
	Table   string
	ObjType reflect.Type
}

func (elem *Elem) Get() any {
	if elem.Value == 0 {
		return nil
	}
	return *(*any)(unsafe.Pointer(elem.Value))
}

func (elem *Elem) Set(val any) error {
	if elem.Value == 0 {
		elem.Value = uintptr(unsafe.Pointer(&val))
	}
	if reflect.TypeOf(elem.Get()) != reflect.TypeOf(val) {
		return errors.New("type mismatch")
	}
	// 向指针写入值
	*(*any)(unsafe.Pointer(elem.Value)) = val
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
		runtime.GetFieldAndReused(field, objTypePtr, i)
		tagName, ok := runtime.GetTag(field.Tag, "db")
		if !ok {
			continue
		}
		elems[i].Index = i
		elems[i].Type = field.Type
		elems[i].Tag = tagName
	}
	o.caches.Store(objType, Cache{
		Elems:   elems,
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
			cache.Elems[i].Value = uintptr(objValue.Field(cache.Elems[i].Index).UnsafePointer())
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

func NewSafeCache() *SafeCache {
	return &SafeCache{
		cache: make(map[reflect.Type]Cache),
	}
}

func (sc *SafeCache) Load(key reflect.Type) (value Cache, ok bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	value, ok = sc.cache[key]
	return
}

func (sc *SafeCache) Store(key reflect.Type, value Cache) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[key] = value
}
