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
	Save(args ...any) error
	Delete(args ...any) error
	Find(args ...any) (any, error)
	FindAll(args ...any) ([]any, error)
	Count(args ...any) (int, error)
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
	ei := 0
	for i := 0; i < numIndex; i++ {
		runtime.GetField(field, objTypePtr, i)
		tagName, ok := runtime.GetTag(field.Tag, "db")
		option, okOption := runtime.GetTag(field.Tag, "option")
		if !ok && (okOption && option != "-" && option != "") {
			ok = true
			tagName = field.Name
		}
		if !ok || tagName == "-" || tagName == "" || field.Type.Kind() == reflect.Invalid || field.Type.Kind() == reflect.Func || field.Type.Kind() == reflect.Chan || field.Type.Kind() == reflect.UnsafePointer || field.Type.Kind() == reflect.Map || field.Type.Kind() == reflect.Interface || field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array || field.Type.Kind() == reflect.Ptr || field.Type.Kind() == reflect.Struct {
			continue
		}
		if okOption && option != "" && option != "-" {
			optMap := make(map[string]string, utils.CountByte(utils.String2Slice(option), ';')+1)
			tmp := utils.SplitStringByByte(option, ';')
			for i := 0; i < len(tmp); i++ {
				sepIndex := strings.IndexByte(tmp[i], '=')
				if sepIndex == -1 {
					optMap[tmp[i]] = "-"
					continue
				}
				opt := tmp[i][:sepIndex]
				val := tmp[i][sepIndex+1:]
				if opt != "" {
					optMap[opt] = val
				}
			}
			elems[ei].Option = optMap
		} else {
			elems[ei].Option = make(map[string]string, 0)
		}
		elems[ei].Index = i
		elems[ei].Type = field.Type
		elems[ei].Tag = tagName
		elems[ei].Offset = field.Offset
		ei++
	}
	o.caches.Store(objType, Cache{
		Elems:   elems[:ei],
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
	objPtr := objValue.UnsafeAddr()
	if rawCache, ok := o.caches.Load(objType); ok {
		cache := rawCache
		ElemsLength := len(cache.Elems)
		for i := 0; i < ElemsLength; i++ {
			cache.Elems[i].Ptr = unsafe.Pointer(objPtr + cache.Elems[i].Offset)
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
				elems[i].Set(utils.AutotAnyI64tAnyI(lii))
			}
		}
	}
}
