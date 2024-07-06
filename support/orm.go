package support

import (
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"unsafe"
)

type ORM struct {
	objectORM  func(*sql.DB, any, reflect.Type, string, []*Elem, error) ObjectORM
	caches     map[reflect.Type]*Cache
	cachesLock sync.RWMutex
	conn       *sql.DB
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
	Value reflect.Value
}

type Cache struct {
	Elems   []*Elem
	Table   string
	ObjType reflect.Type
}

func (elem *Elem) Get() any {
	return elem.Value.Interface()
}

func (elem *Elem) Set(val any) {
	elem.Value.Set(reflect.ValueOf(val))
}

func (orm *ORM) Init(conn *sql.DB, driver func(*sql.DB, any, reflect.Type, string, []*Elem, error) ObjectORM) {
	orm.objectORM = driver
	orm.conn = conn
}

func (o *ORM) Register(tableName string, object any) error {
	objType := reflect.TypeOf(object)
	if objType.Kind() == reflect.Ptr {
		ptr := reflect.ValueOf(object).Elem()
		objType = ptr.Type()
	} else if objType.Kind() != reflect.Struct {
		return errors.New("object must be a struct or a pointer to a struct")
	}
	numIndex := objType.NumField()
	elems := make([]*Elem, numIndex)
	for i := 0; i < numIndex; i++ {
		fieldType := objType.Field(i)
		tagName := fieldType.Tag.Get("db")
		if tagName == "" {
			continue
		}
		elems = append(elems, &Elem{
			Index: i,
			Type:  fieldType.Type,
			Tag:   tagName,
		})
	}
	cache := &Cache{
		Elems:   elems,
		Table:   tableName,
		ObjType: objType,
	}
	o.cachesLock.Lock()
	o.caches[objType] = cache
	o.cachesLock.Unlock()
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
	o.cachesLock.RLock()
	if cache, ok := o.caches[objType]; ok {
		Elems := []*Elem{}
		copy(Elems, cache.Elems)
		ElemsLength := len(Elems)
		for i := 0; i < ElemsLength; i++ {
			Elems[i].Value = reflect.NewAt(cache.Elems[i].Type, unsafe.Pointer(objValue.Field(cache.Elems[i].Index).UnsafeAddr())).Elem()
		}
		orm = o.objectORM(o.conn, object, cache.ObjType, cache.Table, Elems, nil)
		o.cachesLock.RUnlock()
		return
	} else {
		orm = o.objectORM(nil, nil, nil, "", nil, errors.New("object not registered"))
		o.cachesLock.RUnlock()
		return
	}
	return
}
