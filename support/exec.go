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
func ScanAllCtx(ctx context.Context, e Execer, query string, args []any, objType reflect.Type, elems []Elem) ([]any, error) {
	rows, err := e.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	objs := make([]any, 0, 16)
	targets := make([]any, len(elems))
	for rows.Next() {
		obj := reflect.New(objType).Elem()
		base := obj.Addr().UnsafePointer()
		for i := range elems {
			targets[i] = reflect.NewAt(elems[i].Type, unsafe.Add(base, elems[i].Offset)).Interface()
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		objs = append(objs, obj.Interface())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return objs, nil
}

// ScanOneCtx 执行单行查询并扫描进 elem 指针指向的目标对象。
// 未命中时返回 sql.ErrNoRows；其余原始错误由调用方判断。
func ScanOneCtx(ctx context.Context, e Execer, query string, args []any, elems []Elem) error {
	row := e.QueryRowContext(ctx, query, args...)
	targets := make([]any, len(elems))
	for i := range elems {
		targets[i] = elems[i].GetInterface()
	}
	err := row.Scan(targets...)
	if errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return err
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
