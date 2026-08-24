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
	"database/sql"
	"errors"
)

// 包级哨兵错误。调用方可用 errors.Is 判别。
var (
	// ErrUnsupportedCondition 查询参数类型不受支持（例如首参既非字符串也非 Condition）
	ErrUnsupportedCondition = errors.New("opao: 不支持的查询条件参数类型")
	// ErrInvalidCondition 条件构造不完整（缺少字段名或比较值等）
	ErrInvalidCondition = errors.New("opao: 查询条件不完整")
	// ErrNoFieldsToUpdate 未找到任何需要写入的列
	ErrNoFieldsToUpdate = errors.New("opao: 没有可写入的字段")
	// ErrMissingWhereClause 缺少 WHERE 条件，拒绝执行可能影响全表的写操作
	ErrMissingWhereClause = errors.New("opao: 缺少 WHERE 条件")
	// ErrRecordNotFound 统一的未命中语义（sql.ErrNoRows 的别名导出）
	ErrRecordNotFound = sql.ErrNoRows
)

// allowAll 全表写操作放行哨兵（不可比较的私有类型，防止伪造）
type allowAll struct{}

// AllowAll 显式放行无 WHERE 条件的全表写操作：
// Delete(AllowAll()) 表示调用方已知晓并接受影响全表。
func AllowAll() any { return allowAll{} }

// IsAllowAll 报告 v 是否为 AllowAll() 生成的放行哨兵。
func IsAllowAll(v any) bool {
	_, ok := v.(allowAll)
	return ok
}

// StripAllowAll 移除哨兵放行参数：RequireWhere 放行后，共享构建器不识别该类型，
// 组装 SQL 前需剔除。仅处理单参形式；其余原样返回。
func StripAllowAll(queryParts []any) []any {
	if len(queryParts) == 1 {
		if _, ok := queryParts[0].(allowAll); ok {
			return nil
		}
	}
	return queryParts
}

// RequireWhere 无条件的写操作直接拒绝，防止全表误伤；
// 显式传入 AllowAll() 哨兵时放行。三驱动共享同一实现。
func RequireWhere(queryParts ...any) error {
	if len(queryParts) == 0 || (len(queryParts) == 1 && queryParts[0] == nil) {
		return ErrMissingWhereClause
	}
	return nil
}
