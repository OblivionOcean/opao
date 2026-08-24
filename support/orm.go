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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync"
	"unsafe"

	opaort "github.com/OblivionOcean/opao/internal/runtime"
	"github.com/OblivionOcean/opao/utils"
)

type ORM struct {
	objectORM func(*sql.DB, any, reflect.Type, string, []Elem, error) ObjectORM
	caches    *SafeCache // map[reflect.Type]Cache
	conn      *sql.DB
	snake     bool            // 蛇形命名回退开关（EnableSnakeCase 置位）
	tx        Execer          // Session/BindTx 绑定的事务执行器；nil 表示走连接池
	ctx       context.Context // WithContext/BindCtx 绑定的上下文；nil 视为 context.Background()
}

// execBinder 由各驱动实现：Load 组装实例后按视图绑定的 tx/ctx 注入执行器。
type execBinder interface {
	BindExec(e Execer, ctx context.Context)
}

// bindSession 把视图级事务/上下文注入驱动实例（两者皆空时为 no-op）。
func bindSession(orm ObjectORM, e Execer, ctx context.Context) {
	if e == nil && ctx == nil {
		return
	}
	if b, ok := orm.(execBinder); ok {
		b.BindExec(e, ctx)
	}
}

// BindTx 将事务绑定到该 ORM 视图：此后 Load 组装的驱动实例改为在 tx 上执行。
// 仅影响当前视图（浅拷贝后调用），不污染共享缓存与原视图；提交/回滚由调用方持有的 tx 控制。
func (o *ORM) BindTx(tx *sql.Tx) { o.tx = tx }

// BindCtx 为该 ORM 视图绑定上下文：此后 Load 组装的驱动实例透传该 ctx。
func (o *ORM) BindCtx(ctx context.Context) { o.ctx = ctx }

// EnableSnakeCase 启用蛇形命名回退：无显式列名（db 标签 / gorm column）的
// 字段按 SnakeCase(字段名) 推断列名。须在首次 Register 前调用。
func (o *ORM) EnableSnakeCase() { o.snake = true }

var (
	layoutOnce     sync.Once
	layoutCheckErr error
)

// verifyRuntimeLayout 启动自检（canary）：用探针结构体校验 unsafe 反射路径
// 与当前 Go 版本的运行时内部布局兼容。把「升级 Go 后静默生成错误 SQL」
// 转化为「启动即报错并指明版本」。
func verifyRuntimeLayout() error {
	var f reflect.StructField
	type probe struct {
		Name string `db:"name"`
	}
	st := opaort.Type2StructType(reflect.TypeOf(probe{}))
	if st == nil || !opaort.GetField(&f, st, 0) ||
		f.Name != "Name" || f.Tag.Get("db") != "name" {
		return fmt.Errorf("opao: 运行时布局自检失败(%s)：unsafe 反射路径与当前 Go 版本不兼容，请反馈 issue", goruntime.Version())
	}
	return nil
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
	layoutOnce.Do(func() { layoutCheckErr = verifyRuntimeLayout() })
	if layoutCheckErr != nil {
		return layoutCheckErr
	}
	objType := reflect.TypeOf(object)
	if objType.Kind() == reflect.Ptr {
		objType = reflect.TypeOf(object).Elem()
	}
	objTypePtr := opaort.Type2StructType(objType)
	if objTypePtr == nil {
		return errors.New("object must be a struct or a pointer to a struct")
	}
	numIndex := opaort.TypeFieldLen(objTypePtr)
	elems := make([]Elem, numIndex)
	field := &reflect.StructField{}
	ei := 0
	for i := 0; i < numIndex; i++ {
		opaort.GetField(field, objTypePtr, i)
		tagName, hasDB := opaort.GetTag(field.Tag, "db")
		option, okOption := opaort.GetTag(field.Tag, "option")

		// gorm 标签迁移映射（fail-open）：db 标签 > gorm column > SnakeCase 回退
		gormBody, _ := opaort.GetTag(field.Tag, "gorm")
		m := ResolveColumn(tagName, hasDB, ParseGormTag(gormBody), field.Name, o.snake)

		// 兼容历史语义：无 db 标签但有合法 option 标签时以字段名作列名
		if m.Column == "" && !m.Skip && !hasDB && okOption && option != "-" && option != "" {
			m.Column = field.Name
		}
		if m.Skip || m.Column == "" || m.Column == "-" {
			continue
		}
		if field.Type.Kind() == reflect.Invalid || field.Type.Kind() == reflect.Func || field.Type.Kind() == reflect.Chan || field.Type.Kind() == reflect.UnsafePointer || field.Type.Kind() == reflect.Map || field.Type.Kind() == reflect.Interface || field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array || field.Type.Kind() == reflect.Ptr || field.Type.Kind() == reflect.Struct {
			continue
		}
		optMap := make(map[string]string, 2)
		if okOption && option != "" && option != "-" {
			tmp := utils.SplitStringByByte(option, ';')
			for j := 0; j < len(tmp); j++ {
				sepIndex := strings.IndexByte(tmp[j], '=')
				if sepIndex == -1 {
					optMap[tmp[j]] = "-"
					continue
				}
				opt := tmp[j][:sepIndex]
				val := tmp[j][sepIndex+1:]
				if opt != "" {
					optMap[opt] = val
				}
			}
		}
		if optMap["autoIncrement"] == "-" || m.AutoIncrement {
			optMap["autoIncrement"] = "-"
			elems[ei].AutoInc = true
		}
		if m.PrimaryKey {
			optMap["primaryKey"] = "-"
		}
		elems[ei].Option = optMap
		elems[ei].Index = i
		elems[ei].Type = field.Type
		elems[ei].Tag = m.Column
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
	if rawCache, ok := o.caches.Load(objType); ok {
		// 每次Load复制一份元素并独立计算字段指针：
		// 缓存中的 Elem 是不可变元数据，避免同类型多个实例共享可变状态
		elems := make([]Elem, len(rawCache.Elems))
		copy(elems, rawCache.Elems)
		base := objValue.Addr().UnsafePointer()
		for i := range elems {
			elems[i].Ptr = unsafe.Add(base, elems[i].Offset)
		}
		orm = o.objectORM(o.conn, object, rawCache.ObjType, rawCache.Table, elems, nil)
		bindSession(orm, o.tx, o.ctx)
		return
	}
	orm = o.objectORM(nil, nil, nil, "", nil, errors.New("object not registered"))
	return
}

// Snapshot 返回已注册对象的表名、字段元数据副本与类型，供上层查询构建器使用。
func (o *ORM) Snapshot(object any) (table string, elems []Elem, objType reflect.Type, err error) {
	objType = reflect.TypeOf(object)
	if objType.Kind() == reflect.Ptr {
		objType = objType.Elem()
	}
	rawCache, ok := o.caches.Load(objType)
	if !ok {
		return "", nil, nil, errors.New("opao: 对象未注册")
	}
	elems = make([]Elem, len(rawCache.Elems))
	copy(elems, rawCache.Elems)
	return rawCache.Table, elems, objType, nil
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

// WriteLii 将 INSERT 结果的自增主键回填到对象字段。
// 按目标字段的实际类型写入（int/int8..int64/uint 系列），不再依赖
// 通用类型转换，避免 int64 主键因类型不匹配被静默跳过。
func WriteLii(elems []Elem, r sql.Result) {
	lii, err := r.LastInsertId()
	if err != nil {
		return
	}
	for i := range elems {
		elem := &elems[i]
		if !elem.AutoInc {
			continue
		}
		ptr := elem.Ptr
		switch elem.Type.Kind() {
		case reflect.Int:
			*(*int)(ptr) = int(lii)
		case reflect.Int8:
			*(*int8)(ptr) = int8(lii)
		case reflect.Int16:
			*(*int16)(ptr) = int16(lii)
		case reflect.Int32:
			*(*int32)(ptr) = int32(lii)
		case reflect.Int64:
			*(*int64)(ptr) = lii
		case reflect.Uint:
			*(*uint)(ptr) = uint(lii)
		case reflect.Uint8:
			*(*uint8)(ptr) = uint8(lii)
		case reflect.Uint16:
			*(*uint16)(ptr) = uint16(lii)
		case reflect.Uint32:
			*(*uint32)(ptr) = uint32(lii)
		case reflect.Uint64:
			*(*uint64)(ptr) = uint64(lii)
		}
	}
}
