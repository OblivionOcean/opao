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

package opao

import (
	"github.com/OblivionOcean/opao/support"
)

// In 创建IN条件
func In(field string, values []any) support.Condition {
	return support.Condition{
		Type:  support.IN,
		Left:  field,
		Right: values,
	}
}

// Eq 创建等于条件
func Eq(field string, value any) support.Condition {
	return support.Condition{
		Type:  support.EQ,
		Left:  field,
		Right: value,
	}
}

// Gt 创建大于条件
func Gt(field string, value any) support.Condition {
	return support.Condition{
		Type:  support.GT,
		Left:  field,
		Right: value,
	}
}

// Lt 创建小于条件
func Lt(field string, value any) support.Condition {
	return support.Condition{
		Type:  support.LT,
		Left:  field,
		Right: value,
	}
}

// Gte 创建大于等于条件
func Gte(field string, value any) support.Condition {
	return support.Condition{
		Type:  support.GTE,
		Left:  field,
		Right: value,
	}
}

// Lte 创建小于等于条件
func Lte(field string, value any) support.Condition {
	return support.Condition{
		Type:  support.LTE,
		Left:  field,
		Right: value,
	}
}

func Or(conditions ...any) support.Condition {
	return support.Condition{
		Type: support.OR,
		Args: conditions,
	}
}

func And(conditions ...any) support.Condition {
	return support.Condition{
		Type: support.AND,
		Args: conditions,
	}
}

func Not(condition any) support.Condition {
	return support.Condition{
		Type: support.NOT,
		Left: condition,
	}
}

func Like(field string, value any) support.Condition {
	return support.Condition{
		Type:  support.LIKE,
		Left:  field,
		Right: value,
	}
}
func NotLike(field string, value any) support.Condition {
	return support.Condition{
		Type:  support.NOT_LIKE,
		Left:  field,
		Right: value,
	}
}
func Between(field string, start, end any) support.Condition {
	return support.Condition{
		Type: support.BETWEEN,
		Left: field,
		Args: []any{start, end},
	}
}
func NotBetween(field string, start, end any) support.Condition {
	return support.Condition{
		Type: support.NOT_BETWEEN,
		Left: field,
		Args: []any{start, end},
	}
}
func Exists(condition any) support.Condition {
	return support.Condition{
		Type: support.EXISTS,
		Left: condition,
	}
}
func NotExists(condition any) support.Condition {
	return support.Condition{
		Type: support.NOT_EXISTS,
		Left: condition,
	}
}
func InSubquery(field string, subquery string) support.Condition {
	return support.Condition{
		Type:  support.IN_SUBQUERY,
		Left:  field,
		Right: subquery,
	}
}
func NotInSubquery(field string, subquery string) support.Condition {
	return support.Condition{
		Type:  support.NOT_IN_SUBQUERY,
		Left:  field,
		Right: subquery,
	}
}
func InValues(field string, values []any) support.Condition {
	return support.Condition{
		Type: support.IN_VALUES,
		Left: field,
		Args: values,
	}
}
func NotInValues(field string, values []any) support.Condition {
	return support.Condition{
		Type: support.NOT_IN_VALUES,
		Left: field,
		Args: values,
	}
}
func Limit(limit int) support.Condition {
	return support.Condition{
		Type:  support.LIMIT,
		Right: limit,
	}
}
func LimitOffset(limit, offset int) support.Condition {
	return support.Condition{
		Type:  support.LIMIT,
		Left:  limit,
		Right: offset,
	}
}
func Custom(condition string, args ...any) support.Condition {
	return support.Condition{
		Type: support.CUSTOM,
		Left: condition,
		Args: args,
	}
}
