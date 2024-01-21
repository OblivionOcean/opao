package opao

import (
	"reflect"
	"unsafe"
)

func (db *Database) Use(obj any) *Action {
	objType := reflect.TypeOf(obj)
	if objType.Kind() == reflect.Ptr {
		ptr := reflect.ValueOf(obj).Elem()
		objType = ptr.Type()
	} else {
		panic("Not a struct or struct pointer")
	}
	db.RWLock.RLock()
	cache := db.Caches[objType]
	db.RWLock.RUnlock()
	if cache == nil {
		return nil
	}
	return cache.Use(obj)
}

type Elem struct {
	Type  reflect.StructField
	Value reflect.Value
	Tag   string
	Name  string
}

type Cache struct {
	Table   string
	ObjType reflect.Type
	Elems   map[string]*Elem
	Db      *Database
}

func (elem *Elem) Get() any {
	return elem.Value.Interface()
}

func (elem *Elem) Set(val any) {
	elem.Value.Set(reflect.ValueOf(val))
}

func (db *Database) Load(obj any, table string) *Cache {
	objType := reflect.TypeOf(obj)
	if objType.Kind() == reflect.Ptr {
		ptr := reflect.ValueOf(obj).Elem()
		objType = ptr.Type()
	} else if objType.Kind() == reflect.Struct {
	} else {
		panic("Not a struct or struct pointer")
	}
	elems := map[string]*Elem{}
	for i := 0; i < objType.NumField(); i++ {
		fieldType := objType.Field(i)
		tagName := fieldType.Tag.Get("db")
		if tagName == "" {
			continue
		}
		elems[tagName] = &Elem{
			Type: fieldType,
			Tag:  tagName,
			Name: fieldType.Name,
		}
	}
	cache := &Cache{
		Db:      db,
		Elems:   elems,
		Table:   table,
		ObjType: objType,
	}
	db.RWLock.Lock()
	db.Caches[objType] = cache
	db.RWLock.Unlock()
	return cache
}

func (cache *Cache) Use(obj any) *Action {
	objType := reflect.TypeOf(obj)
	objValue := reflect.ValueOf(obj)
	if objType.Kind() == reflect.Ptr {
		ptr := reflect.ValueOf(obj).Elem()
		objType = ptr.Type()
		objValue = ptr
	} else {
		panic("Not a struct pointer")
	}
	if objType != cache.ObjType {
		panic("Incorrect type")
	}

	for _, val := range cache.Elems {
		val.Value = reflect.NewAt(val.Type.Type, unsafe.Pointer(objValue.FieldByName(val.Name).UnsafeAddr())).Elem()
	}
	return &Action{
		Cache: cache,
		Elems: cache.Elems,
		Table: cache.Table,
		Db:    cache.Db,
		Obj:   obj,
	}
}
