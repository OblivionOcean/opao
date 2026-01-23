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

package mysql

import (
	"database/sql"
	"database/sql/driver"
	"reflect"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/utils"
)

// MySQL MySQL 数据库 ORM 实现
type MySQL struct {
	Table   string         // 表名
	err     error          // 错误信息
	Elems   []support.Elem // 字段元素列表
	conn    *sql.DB        // 数据库连接
	obj     any            // 关联的对象
	objType reflect.Type   // 对象类型
}

// NewMySQL 创建 MySQL ORM 实例
// 参数:
//   - conn: 数据库连接
//   - obj: 关联的对象
//   - objType: 对象的类型信息
//   - table: 表名
//   - Elems: 字段元素列表
//   - err: 初始化错误
func NewMySQL(conn *sql.DB, obj any, objType reflect.Type, table string, Elems []support.Elem, err error) support.ObjectORM {
	if err != nil {
		return &MySQL{err: err}
	}
	return &MySQL{Table: table, Elems: Elems, err: err, conn: conn, obj: obj, objType: objType}
}

// Error 返回当前 ORM 实例的错误信息
func (qt *MySQL) Error() error {
	return qt.err
}

// Update 更新数据库记录
// 根据提供的查询条件和值更新记录,仅更新非零值且非自增字段的字段
// 使用 MySQL 的 `table` 引用表名和 ? 占位符
// 参数:
//   - queryParts: 查询条件部分,可以是条件字符串和参数
//
// 返回:
//   - error: 执行错误
func (qt *MySQL) Update(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)

	// 计算 SQL 语句所需的缓冲区大小
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Zero() {
			continue
		}
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		// 字段名(2个反引号) + `=?`(3) + 逗号(1)
		elemsNameLength += len(qt.Elems[i].Tag) + 4 + 1
	}

	// 如果没有字段需要更新,直接返回
	if elemsNameLength == 0 {
		return nil
	}

	// 创建缓冲区并构建 UPDATE 语句
	var buf utils.Buffer
	if query == "-" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength) // UPDATE `table` SET
	} else {
		buf = utils.NewBuffer(21 + tabNameLen + queryStringLen + elemsNameLength) // UPDATE `table` SET WHERE
	}
	buf.WriteString("UPDATE `")
	buf.WriteString(qt.Table)
	buf.WriteString("` SET ")

	// 收集需要更新的字段值并构建 SET 子句
	values := make([]any, 0, elemsLeng+len(args))
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Zero() {
			continue
		}
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		buf.WriteByte('`')
		buf.WriteString(qt.Elems[i].Tag)
		buf.WriteString("`=?")
		buf.WriteByte(',')
		values = append(values, qt.Elems[i].Get())
	}
	buf.TruncateLast(1) // 移除末尾的逗号

	// 构建 WHERE 子句
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
		values = append(values, args...)
	}

	// 执行 UPDATE 语句
	r, err := qt.conn.Exec(buf.String(), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// Save 保存或更新数据库记录
// 与 Update 类似,但包含所有非自增字段(包括零值)
// 参数:
//   - queryParts: 查询条件部分,可以是条件字符串和参数
//
// 返回:
//   - error: 执行错误
func (qt *MySQL) Save(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)

	// 计算 SQL 语句所需的缓冲区大小
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		// 字段名(2个反引号) + `=?`(3) + 逗号(1)
		elemsNameLength += len(qt.Elems[i].Tag) + 4 + 1
	}

	// 创建缓冲区并构建 UPDATE 语句
	var buf utils.Buffer
	if query == "-" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength)
	} else {
		buf = utils.NewBuffer(21 + tabNameLen + queryStringLen + elemsNameLength)
	}
	buf.WriteString("UPDATE `")
	buf.WriteString(qt.Table)
	buf.WriteString("` SET ")

	// 收集所有字段值并构建 SET 子句
	values := make([]any, 0, elemsLeng+len(args))
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		buf.WriteByte('`')
		buf.WriteString(qt.Elems[i].Tag)
		buf.WriteString("`=?")
		buf.WriteByte(',')
		values = append(values, qt.Elems[i].Get())
	}
	buf.TruncateLast(1) // 移除末尾的逗号

	// 构建 WHERE 子句
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
		values = append(values, args...)
	}

	// 执行 UPDATE 语句
	r, err := qt.conn.Exec(buf.String(), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// Delete 删除数据库记录
// 根据提供的查询条件删除匹配的记录
// 使用 MySQL 的 `table` 引用表名和 ? 占位符
// 参数:
//   - queryParts: 查询条件部分,可以是条件字符串和参数
//
// 返回:
//   - error: 执行错误
func (qt *MySQL) Delete(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)

	// 计算 SQL 语句所需的缓冲区大小
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(14 + tabNameLen) // DELETE FROM `table`
	} else {
		buf = utils.NewBuffer(20 + tabNameLen + queryStringLen) // DELETE FROM `table` WHERE
	}
	buf.WriteString("DELETE FROM `")
	buf.WriteString(qt.Table)
	buf.WriteByte('`')

	// 构建 WHERE 子句
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
	}

	// 执行 DELETE 语句
	_, err := qt.conn.Exec(buf.String(), args...)
	return err
}

// Create 插入新记录到数据库
// 使用 INSERT 语句将数据插入到表中,跳过自增字段
// 使用 MySQL 的 `table` 引用表名和 ? 占位符
// 返回:
//   - error: 执行错误
func (qt *MySQL) Create() error {
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)

	// 计算 SQL 语句所需的缓冲区大小
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		// 字段名(2个反引号) + 逗号(2)
		elemsNameLength += len(qt.Elems[i].Tag) + 3 + 2
	}

	// 创建缓冲区并构建 INSERT 语句
	buf := utils.NewBuffer(29 + tabNameLen + elemsNameLength) // INSERT INTO `table` (...) VALUES (...);
	buf.WriteString("INSERT INTO `")
	buf.WriteString(qt.Table)
	buf.WriteString("` (")

	// 构建字段列表
	values := make([]any, 0, elemsLeng)
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		buf.WriteByte('`')
		buf.WriteString(qt.Elems[i].Tag)
		buf.WriteByte('`')
		values = append(values, qt.Elems[i].Get())
		buf.WriteByte(',')
	}
	buf.TruncateLast(1) // 移除末尾的逗号
	buf.WriteString(") VALUES (")

	// 构建 VALUES 子句,使用 MySQL 的 ? 占位符
	for i := 0; i < elemsLeng; i++ {
		buf.WriteByte('?')
		buf.WriteByte(',')
	}
	buf.TruncateLast(1) // 移除末尾的逗号
	buf.WriteString(");")

	// 执行 INSERT 语句
	r, err := qt.conn.Exec(buf.String(), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// FindAll 查询多条记录
// 根据提供的查询条件查询所有匹配的记录
// 使用 MySQL 的 `table` 引用表名和 ? 占位符
// 参数:
//   - queryParts: 查询条件部分,可以是条件字符串和参数
//
// 返回:
//   - []any: 查询结果对象列表
//   - error: 执行错误
func (qt *MySQL) FindAll(queryParts ...any) ([]any, error) {
	query, args := qt.buildQuery(queryParts...)

	// 执行查询
	rows, err := qt.conn.Query(qt.getSelectSQL(query), args...)
	elemsLen := len(qt.Elems)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	// 遍历查询结果
	var objs []any
	for rows.Next() {
		obj := reflect.New(qt.objType).Elem()
		Scans := make([]any, elemsLen)
		for i := 0; i < elemsLen; i++ {
			// 使用反射获取字段的地址用于扫描
			Scans[i] = reflect.NewAt(qt.Elems[i].Type, obj.Field(qt.Elems[i].Index).Addr().UnsafePointer()).Interface()
		}
		err := rows.Scan(Scans...)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		objs = append(objs, obj.Interface())
	}
	return objs, nil
}

// Find 查询单条记录
// 根据提供的查询条件查询第一条匹配的记录
// 使用 MySQL 的 `table` 引用表名和 ? 占位符
// 参数:
//   - queryParts: 查询条件部分,可以是条件字符串和参数
//
// 返回:
//   - any: 查询结果对象
//   - error: 执行错误
func (qt *MySQL) Find(queryParts ...any) (any, error) {
	query, args := qt.buildQuery(queryParts...)

	// 执行查询
	row := qt.conn.QueryRow(qt.getSelectSQL(query), args...)

	elemsLen := len(qt.Elems)
	Scans := make([]any, elemsLen)
	for i := 0; i < elemsLen; i++ {
		Scans[i] = qt.Elems[i].GetInterface()
	}

	// 扫描结果
	err := row.Scan(Scans...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return qt.obj, nil
}

// Count 统计记录数量
// 根据提供的查询条件统计匹配的记录数
// 使用 MySQL 的 `table` 引用表名和 ? 占位符
// 参数:
//   - queryParts: 查询条件部分,可以是条件字符串和参数
//
// 返回:
//   - int: 记录数量
//   - error: 执行错误
func (qt *MySQL) Count(queryParts ...any) (int, error) {
	query, args := qt.buildQuery(queryParts...)

	// 计算 SQL 语句所需的缓冲区大小
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(23 + tabNameLen) // SELECT COUNT(*) FROM `table`
	} else {
		buf = utils.NewBuffer(30 + tabNameLen + queryStringLen) // SELECT COUNT(*) FROM `table` WHERE
	}
	buf.WriteString("SELECT COUNT(*) FROM `")
	buf.WriteString(qt.Table)
	buf.WriteString("`")

	// 构建 WHERE 子句
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
	}

	// 执行 COUNT 查询
	var counter int
	err := qt.conn.QueryRow(buf.String(), args).Scan(&counter)
	return counter, err
}

// autotType 根据目标类型和源值执行类型转换
// 参数:
//   - destType: 目标类型的反射类型
//   - src: 源值
//
// 返回:
//   - any: 转换后的值
func autotType(destType reflect.Kind, src any) any {
	switch destType {
	case reflect.String:
		return utils.ToString(src)
	case reflect.Slice:
		return utils.ToBytes(src)
	case reflect.Bool:
		bv, err := driver.Bool.ConvertValue(src)
		if err == nil {
			return bv.(bool)
		}
		return nil
	case reflect.Int64, reflect.Uint64:
		return utils.AutotAnyI64tAnyI(src)
	}

	switch src.(type) {
	case *any:
		return reflect.ValueOf(src).Elem().Interface()
	}

	if src == nil {
		return nil
	}
	return src.(*any)
}

// IsEmpty 检查错误是否由于空结果导致
// 参数:
//   - err: 错误对象
//
// 返回:
//   - bool: 如果是空结果错误返回 true,否则返回 false
func IsEmpty(err error) bool {
	if err != nil && err.Error() == "Empty result" {
		return true
	}
	return false
}

// getSelectSQL 生成 SELECT 查询语句
// 参数:
//   - queryString: WHERE 子句的条件字符串
//
// 返回:
//   - string: 完整的 SELECT SQL 语句
func (qt *MySQL) getSelectSQL(queryString string) string {
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	queryStringLen := len(queryString)

	// 计算 SQL 语句所需的缓冲区大小
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		// 字段名(2个反引号) + 逗号(1)
		elemsNameLength += len(qt.Elems[i].Tag) + 2 + 1
	}

	// 创建缓冲区并构建 SELECT 语句
	var buf utils.Buffer
	if queryString == "" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength) // SELECT ... FROM `table`
	} else {
		buf = utils.NewBuffer(22 + tabNameLen + queryStringLen + elemsNameLength) // SELECT ... FROM `table` WHERE
	}
	buf.WriteString("SELECT ")

	// 构建字段列表
	for i := 0; i < elemsLeng; i++ {
		buf.WriteByte('`')
		buf.WriteString(qt.Elems[i].Tag)
		buf.WriteByte('`')
		buf.WriteByte(',')
	}
	buf.TruncateLast(1) // 移除末尾的逗号

	// 构建 FROM 和 WHERE 子句
	buf.WriteString(" FROM `")
	buf.WriteString(qt.Table)
	buf.WriteString("`")
	if queryString != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(queryString)
	}
	return buf.String()
}
