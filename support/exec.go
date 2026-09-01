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
	"reflect"
	"unsafe"
)

// ErrNoRows 统一的空结果语义（sql.ErrNoRows 的别名导出）
var ErrNoRows = sql.ErrNoRows

// Execer 同时被 *sql.DB 与 *sql.Tx 满足的最小接口，
// 是事务注入（Session/WithTx）与上下文透传的统一分派点。
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ScanAllCtx 执行查询并把每一行扫描到新分配的对象，返回对象切片。
// 迭代结束后检查 rows.Err()，中途 I/O 错误不会被静默吞掉。
// 对 string/[]byte/指针等可空类型使用 **T 中间目标以兼容 NULL 值。
func ScanAllCtx(ctx context.Context, e Execer, query string, args []any, objType reflect.Type, elems []Elem) ([]any, error) {
	rows, err := e.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	objs := make([]any, 0, 16)
	targets := make([]any, len(elems))
	ptrs := make([]reflect.Value, len(elems)) // 存储 *T 指针的 reflect.Value
	for rows.Next() {
		obj := reflect.New(objType).Elem()
		base := obj.Addr().UnsafePointer()
		for i := range elems {
			ft := elems[i].Type
			ptr := unsafe.Add(base, elems[i].Offset)
			// 对 string、[]byte、slice、指针等可空类型使用 **T
			if isNullable(ft) {
				// ptrToField: *T 指向字段
				ptrToField := reflect.NewAt(ft, ptr)
				// ptrToPtrToField: **T，用于 Scan
				ptrToPtrToField := reflect.New(reflect.PtrTo(ft))
				ptrToPtrToField.Elem().Set(ptrToField)
				targets[i] = ptrToPtrToField.Interface() // **T
				ptrs[i] = ptrToField                    // 用于最终赋值
			} else {
				targets[i] = reflect.NewAt(ft, ptr).Interface() // *T
			}
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		// NULL -> 指针为 nil -> 零值；非 NULL -> 指针有值 -> 解引用
		for i := range elems {
			if isNullable(elems[i].Type) {
				// 从 **T 中取出 *T
				pp := reflect.ValueOf(targets[i]).Elem() // *T (可能是 nil)
				if !pp.IsNil() {
					// 非 NULL：将值复制到目标字段
					ptrs[i].Elem().Set(pp.Elem())
				}
				// NULL：目标字段保持零值
			}
		}
		objs = append(objs, obj.Interface())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return objs, nil
}

// isNullable 返回类型是否可能在数据库中为 NULL
func isNullable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String, reflect.Slice, reflect.Ptr, reflect.Interface:
		return true
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Uintptr:
		return false
	}
	// time.Time 等结构体类型
	return false
}

// ScanOneCtx 执行单行查询并扫描进 elem 指针指向的目标对象。
// 未命中时返回 sql.ErrNoRows；其余原始错误由调用方判断。
// 对 string 等可空类型使用 **T 中间目标以兼容 NULL 值。
func ScanOneCtx(ctx context.Context, e Execer, query string, args []any, elems []Elem) error {
	row := e.QueryRowContext(ctx, query, args...)
	targets := make([]any, len(elems))
	for i := range elems {
		ft := elems[i].Type
		if isNullable(ft) {
			// ptrToField: *T 指向字段
			ptrToField := elems[i].GetInterface() // *T
			// ptrToPtrToField: **T，用于 Scan
			ptrToPtrToField := reflect.New(reflect.PtrTo(ft))
			ptrToPtrToField.Elem().Set(reflect.ValueOf(ptrToField))
			targets[i] = ptrToPtrToField.Interface() // **T
		} else {
			targets[i] = elems[i].GetInterface() // *T
		}
	}
	err := row.Scan(targets...)
	if err != nil {
		return err
	}
	// NULL -> 指针为 nil -> 零值；非 NULL -> 指针有值 -> 解引用
	for i := range elems {
		if isNullable(elems[i].Type) {
			// 从 **T 中取出 *T
			pp := reflect.ValueOf(targets[i]).Elem() // *T (可能是 nil)
			if !pp.IsNil() {
				// 非 NULL：将值复制到目标字段
				val := pp.Elem()                    // T
				// 通过 elems[i].Ptr 直接设置值
				reflect.NewAt(elems[i].Type, elems[i].Ptr).Elem().Set(val)
			}
			// NULL：目标字段保持零值
		}
	}
	return nil
}

// ScanCountCtx 执行 COUNT 查询
func ScanCountCtx(ctx context.Context, e Execer, query string, args []any) (int, error) {
	var counter int
	err := e.QueryRowContext(ctx, query, args...).Scan(&counter)
	return counter, err
}

// ExecCtx 执行写语句并回填自增主键（仅 INSERT 场景调用 WriteLii）。
// update/save 场景传 backfill=false 以避免 LastInsertId 清零内存中的主键值。
func ExecCtx(ctx context.Context, e Execer, query string, args []any, elems []Elem, backfill bool) error {
	r, err := e.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if backfill {
		WriteLii(elems, r)
	}
	return nil
}

// ScanAll 执行查询并把每一行扫描到新分配的对象（context.Background 包装）。
func ScanAll(conn *sql.DB, query string, args []any, objType reflect.Type, elems []Elem) ([]any, error) {
	return ScanAllCtx(context.Background(), conn, query, args, objType, elems)
}

// ScanOne 执行单行查询并扫描进 elem 指针指向的目标对象（context.Background 包装）。
func ScanOne(conn *sql.DB, query string, args []any, elems []Elem) error {
	return ScanOneCtx(context.Background(), conn, query, args, elems)
}

// ScanCount 执行 COUNT 查询（context.Background 包装）
func ScanCount(conn *sql.DB, query string, args []any) (int, error) {
	return ScanCountCtx(context.Background(), conn, query, args)
}

// Exec 执行写语句并回填自增主键（context.Background 包装）
func Exec(conn *sql.DB, query string, args []any, elems []Elem, backfill bool) error {
	return ExecCtx(context.Background(), conn, query, args, elems, backfill)
}
