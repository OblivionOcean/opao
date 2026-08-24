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
)

// Driver 各方言驱动的共享实现：行为完全一致，仅注入的 Dialect 不同。
// SQL 构建委托给本包 Builder，语句文本复用 L1 内联缓存。
// 方言包装类型位于子包 support/mysql、support/pg、support/sqlite。
type Driver struct {
	Dialect     Dialect         // 方言单例
	Table       string          // 表名
	quotedTable string          // 预计算的 Quote(Table)，热路径零拼接
	dialectName string          // 预计算的方言注册名（缓存键分量）
	fp          uint64          // 预计算的列元数据指纹（缓存键分量）
	err         error           // 错误信息（Load 阶段注入，操作前应检查）
	Elems       []Elem          // 字段元素列表
	Exec        Execer          // 执行器：*sql.DB 或 *sql.Tx（WithTx/Session 注入）
	Ctx         context.Context // 上下文：默认 context.Background()，WithCtx 替换
	obj         any             // 关联的对象
	objType     reflect.Type    // 对象类型
}

// NewDriver 创建绑定方言的共享驱动实例（方言子包的构造函数内部调用）。
func NewDriver(dialect Dialect, conn *sql.DB, obj any, objType reflect.Type, table string, elems []Elem, err error) *Driver {
	if err != nil {
		return &Driver{err: err}
	}
	return &Driver{Dialect: dialect, Table: table, quotedTable: dialect.Quote(table),
		dialectName: dialect.Name(), fp: elemsFingerprint(elems),
		Elems: elems, Exec: conn, Ctx: context.Background(), obj: obj, objType: objType}
}

func (b *Driver) Error() error { return b.err }

// BindExec 实现会话注入接口：ORM.Load 后按视图绑定的 tx/ctx 调整执行器。
func (b *Driver) BindExec(e Execer, ctx context.Context) {
	if e != nil {
		b.Exec = e
	}
	if ctx != nil {
		b.Ctx = ctx
	}
}

// Create 插入新记录，自增字段跳过并在成功后回填 LastInsertId。
// 语句文本走 L1 内联缓存；绑定值切片池化复用。
func (b *Driver) Create() error {
	ss := internStmtsAt(b.Dialect, b.dialectName, b.Table, b.fp, b.Elems)
	pa := getArgs()
	defer putArgs(pa)
	for i := range b.Elems {
		if !b.Elems[i].AutoInc {
			*pa = append(*pa, b.Elems[i].Get())
		}
	}
	if len(*pa) == 0 {
		return fmt.Errorf("%w", ErrNoFieldsToUpdate)
	}
	err := ExecCtx(b.Ctx, b.Exec, ss.Insert, *pa, b.Elems, true)
	return err
}

// execWriteUpdate Update/Save 的共享实现：SET 子句按位模式缓存，
// WHERE 片段走 L2 条件哈希缓存；缓冲与值切片均池化。
func (b *Driver) execWriteUpdate(includeZero bool, queryParts ...any) error {
	if err := RequireWhere(queryParts...); err != nil {
		return err
	}
	// 快路径分类：避免把 variadic 切片传入会使其逃逸的辅助函数
	var (
		whereCond Condition
		hasCond   bool
		whereRaw  string
		rawArgs   []any
		fullTable bool // AllowAll 显式放行
	)
	switch n := len(queryParts); {
	case n == 0 || (n == 1 && queryParts[0] == nil):
		return ErrMissingWhereClause
	case n == 1:
		switch v := queryParts[0].(type) {
		case Condition:
			whereCond = v
			hasCond = true
		case allowAll:
			fullTable = true
		case string:
			whereRaw = v
		default:
			return fmt.Errorf("%w: %T", ErrUnsupportedCondition, v)
		}
	default:
		if _, ok := queryParts[0].(allowAll); ok {
			fullTable = true
			break
		}
		s, ok := queryParts[0].(string)
		if !ok {
			return fmt.Errorf("%w: 多参数形式首参必须为字符串", ErrUnsupportedCondition)
		}
		whereRaw = s
		rawArgs = queryParts[1:]
	}

	pa := getArgs()
	buf := getBuf()
	defer func() { putBuf(buf); putArgs(pa) }()

	setSQL, err := cachedSetClauseAt(b.Dialect, b.dialectName, b.Table, b.fp, b.Elems, includeZero, pa)
	if err != nil {
		if errors.Is(err, ErrNoFieldsToUpdate) {
			return nil
		}
		return err
	}
	var where string
	switch {
	case hasCond:
		w, aerr := cachedWhereAppendAt(b.dialectName, b.Table, b.Elems, b.obj, b.objType, whereCond, pa)
		if aerr != nil {
			return aerr
		}
		where = w
	case whereRaw != "":
		where = whereRaw
		*pa = append(*pa, rawArgs...)
	case fullTable:
		// 显式放行：无 WHERE
	}
	buf.WriteString("UPDATE ")
	buf.WriteString(b.quotedTable)
	buf.WriteString(" SET ")
	buf.WriteString(setSQL)
	if where != "" {
		appendClause(buf, where)
	}
	return ExecCtx(b.Ctx, b.Exec, Rebind(b.Dialect, buf.String()), *pa, b.Elems, false)
}

// Update 更新非零值且非自增字段；必须携带 WHERE 条件（AllowAll 放行全表）
func (b *Driver) Update(queryParts ...any) error { return b.execWriteUpdate(false, queryParts...) }

// Save 更新全部非自增字段(含零值)；必须携带 WHERE 条件（AllowAll 放行全表）
func (b *Driver) Save(queryParts ...any) error { return b.execWriteUpdate(true, queryParts...) }

// Delete 按条件删除记录；必须携带 WHERE 条件（AllowAll 放行全表）
func (b *Driver) Delete(queryParts ...any) error {
	if err := RequireWhere(queryParts...); err != nil {
		return err
	}
	query, args, err := BuildDelete(b.Dialect, b.Table, b.Elems, b.obj, b.objType, StripAllowAll(queryParts)...)
	if err != nil {
		return err
	}
	return ExecCtx(b.Ctx, b.Exec, query, args, b.Elems, false)
}

// Find 查询单条记录并扫描进关联对象
func (b *Driver) Find(queryParts ...any) (any, error) {
	query, args, err := BuildSelect(b.Dialect, b.Table, b.Elems, b.obj, b.objType, queryParts...)
	if err != nil {
		return nil, err
	}
	if err := ScanOneCtx(b.Ctx, b.Exec, query, args, b.Elems); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return b.obj, nil
}

// FindAll 查询多条记录；无条件时走 L1 内联的全表语句快路径
func (b *Driver) FindAll(queryParts ...any) ([]any, error) {
	if len(queryParts) == 0 {
		ss := internStmtsAt(b.Dialect, b.dialectName, b.Table, b.fp, b.Elems)
		return ScanAllCtx(b.Ctx, b.Exec, ss.SelectAll, nil, b.objType, b.Elems)
	}
	query, args, err := BuildSelect(b.Dialect, b.Table, b.Elems, b.obj, b.objType, queryParts...)
	if err != nil {
		return nil, err
	}
	return ScanAllCtx(b.Ctx, b.Exec, query, args, b.objType, b.Elems)
}

// Count 统计记录数量；无条件时走 L1 内联的计数语句快路径
func (b *Driver) Count(queryParts ...any) (int, error) {
	if len(queryParts) == 0 {
		ss := internStmtsAt(b.Dialect, b.dialectName, b.Table, b.fp, b.Elems)
		return ScanCountCtx(b.Ctx, b.Exec, ss.CountAll, nil)
	}
	query, args, err := BuildCount(b.Dialect, b.Table, b.Elems, b.obj, b.objType, queryParts...)
	if err != nil {
		return 0, err
	}
	return ScanCountCtx(b.Ctx, b.Exec, query, args)
}
