package pg

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/utils"
)

type PgSQL struct {
	Table   string
	err     error
	Elems   []support.Elem
	conn    *sql.DB
	obj     any
	objType reflect.Type
}

func NewPg(conn *sql.DB, obj any, objType reflect.Type, table string, Elems []support.Elem, err error) support.ObjectORM {
	if err != nil {
		return &PgSQL{err: err}
	}
	return &PgSQL{Table: table, Elems: Elems, err: err, conn: conn, obj: obj, objType: objType}
}

func (qt *PgSQL) Error() error {
	return qt.err
}

// Update updates the database record based on the provided query string and values
func (qt *PgSQL) Update(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)
	// 修改参数拼接逻辑，将字段值和查询参数合并
	values := make([]any, 0, len(qt.Elems))
	for i := 0; i < len(qt.Elems); i++ {
		if qt.Elems[i].Zero() {
			continue
		}
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		values = append(values, qt.Elems[i].Get())
	}
	values = append(values, args...)
	// Fetch tag and corresponding stored data
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
		elemsNameLength += len(qt.Elems[i].Tag) + 4
		if i != elemsLeng-1 {
			elemsNameLength += 1
		}
	}
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength)
	} else {
		buf = utils.NewBuffer(21 + tabNameLen + queryStringLen + elemsNameLength)
	}
	buf.WriteString("UPDATE \"")
	buf.WriteString(qt.Table)
	buf.WriteString("\" SET ")

	whereCounter := 1
	// 修正占位符生成逻辑
	for i := 0; i < len(qt.Elems); i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		whereCounter++
		buf.WriteString(fmt.Sprintf("\"=$%d", whereCounter))
	}
	if query != "" {
		buf.WriteString(" WHERE ")
		// 替换问号为 pgsql 占位符格式
		var processedQuery strings.Builder
		for _, c := range query {
			if c == '?' {
				processedQuery.WriteString(fmt.Sprintf("$%d", whereCounter))
				whereCounter++
			} else {
				processedQuery.WriteRune(c)
			}
		}
		buf.WriteString(processedQuery.String())
	}
	r, err := qt.conn.Exec(buf.String(), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

func (qt *PgSQL) Save(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)
	// 修改参数拼接逻辑，将字段值和查询参数合并
	values := make([]any, 0, len(qt.Elems))
	for i := 0; i < len(qt.Elems); i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		values = append(values, qt.Elems[i].Get())
	}
	values = append(values, args...)
	// Fetch tag and corresponding stored data
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		elemsNameLength += len(qt.Elems[i].Tag) + 4
		if i != elemsLeng-1 {
			elemsNameLength += 1
		}
	}
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength)
	} else {
		buf = utils.NewBuffer(21 + tabNameLen + queryStringLen + elemsNameLength)
	}
	buf.WriteString("UPDATE \"")
	buf.WriteString(qt.Table)
	buf.WriteString("\" SET ")

	whereCounter := 1
	// 修正占位符生成逻辑
	for i := 0; i < len(qt.Elems); i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		whereCounter++
		buf.WriteString(fmt.Sprintf("\"=$%d", whereCounter))
	}
	if query != "" {
		buf.WriteString(" WHERE ")
		// 替换问号为 pgsql 占位符格式
		var processedQuery strings.Builder
		for _, c := range query {
			if c == '?' {
				processedQuery.WriteString(fmt.Sprintf("$%d", whereCounter))
				whereCounter++
			} else {
				processedQuery.WriteRune(c)
			}
		}
		buf.WriteString(processedQuery.String())
	}
	r, err := qt.conn.Exec(buf.String(), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// Delete deletes database records based on the provided query string and values
func (qt *PgSQL) Delete(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(14 + tabNameLen)
	} else {
		buf = utils.NewBuffer(20 + tabNameLen + queryStringLen)
	}
	buf.WriteString("DELETE FROM \"")
	buf.WriteString(qt.Table)
	buf.WriteByte('"')
	if query != "" {
		buf.WriteString(" WHERE ")
		// 处理 WHERE 子句中的占位符
		whereCounter := 1
		var processedQuery strings.Builder
		for _, c := range query {
			if c == '?' {
				processedQuery.WriteString(fmt.Sprintf("$%d", whereCounter))
				whereCounter++
			} else {
				processedQuery.WriteRune(c)
			}
		}
		buf.WriteString(processedQuery.String())
	}
	_, err := qt.conn.Exec(buf.String(), args...)
	return err
}

// Create inserts data into the database using INSERT operation
func (qt *PgSQL) Create() error {
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		elemsNameLength += len(qt.Elems[i].Tag) + 3
		if i != elemsLeng-1 {
			elemsNameLength += 2
		}
	}
	buf := utils.NewBuffer(29 + tabNameLen + elemsNameLength)
	buf.WriteString("INSERT INTO \"")
	buf.WriteString(qt.Table)
	buf.WriteString("\" (")

	values := make([]any, 0, elemsLeng)
	for i := 0; i < elemsLeng; i++ {
		buf.WriteByte('"')
		buf.WriteString(qt.Elems[i].Tag)
		buf.WriteByte('"')
		values = append(values, qt.Elems[i].Get())
		if i != elemsLeng-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString(") VALUES (")
	// 修改 VALUES 占位符为 $n 格式
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		buf.WriteString(fmt.Sprintf("$%d", i+1))
		if i != elemsLeng-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString(");")
	r, err := qt.conn.Exec(buf.String(), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// FindAll retrieves data from the database based on the provided query string and values
func (qt *PgSQL) FindAll(queryParts ...any) ([]any, error) {
	query, args := qt.buildQuery(queryParts...)
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
	var objs []any
	for rows.Next() {
		obj := reflect.New(qt.objType).Elem()
		Scans := make([]any, elemsLen)
		for i := 0; i < elemsLen; i++ {
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

// Find retrieves a single record from the database based on the provided query string and values
func (qt *PgSQL) Find(queryParts ...any) (any, error) {
	query, args := qt.buildQuery(queryParts...)

	row := qt.conn.QueryRow(qt.getSelectSQL(query), args...)

	elemsLen := len(qt.Elems)
	Scans := make([]any, elemsLen)
	for i := 0; i < elemsLen; i++ {
		Scans[i] = qt.Elems[i].GetInterface()
	}

	err := row.Scan(Scans...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return qt.obj, nil
}

func (qt *PgSQL) Count(queryParts ...any) (int, error) {
	query, args := qt.buildQuery(queryParts...)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(23 + tabNameLen)
	} else {
		buf = utils.NewBuffer(30 + tabNameLen + queryStringLen)
	}
	buf.WriteString("SELECT COUNT(*) FROM \"")

	buf.WriteString(qt.Table)

	buf.WriteString("\"")
	if query != "" {
		buf.WriteString(" WHERE ")

		// 使用计数器处理占位符
		whereCounter := 1
		var processedQuery strings.Builder
		for _, c := range query {
			if c == '?' {
				processedQuery.WriteString(fmt.Sprintf("$%d", whereCounter))
				whereCounter++
			} else {
				processedQuery.WriteRune(c)
			}
		}
		buf.WriteString(processedQuery.String())
	}
	var counter int
	err := qt.conn.QueryRow(buf.String(), args).Scan(&counter)
	return counter, err
}

// autotType performs type conversion based on the destination type and source value
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

// IsEmpty checks if the error is due to an empty result
func IsEmpty(err error) bool {
	if err != nil && err.Error() == "Empty result" {
		return true
	}
	return false
}
func (qt *PgSQL) getSelectSQL(queryString string) string {
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	queryStringLen := len(queryString)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		elemsNameLength += len(qt.Elems[i].Tag) + 2
		if i != elemsLeng-1 {
			elemsNameLength += 1
		}
	}
	var buf utils.Buffer
	if queryString == "" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength)
	} else {
		buf = utils.NewBuffer(22 + tabNameLen + queryStringLen + elemsNameLength)
	}
	buf.WriteString("SELECT ")

	for i := 0; i < elemsLeng; i++ {
		buf.WriteByte('"')
		buf.WriteString(qt.Elems[i].Tag)
		buf.WriteByte('"')
		if i != elemsLeng-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString(" FROM \"")
	buf.WriteString(qt.Table)
	buf.WriteByte('"')
	if queryString != "" {
		buf.WriteString(" WHERE ")

		// 使用计数器处理占位符
		whereCounter := 1
		var processedQuery strings.Builder
		for _, c := range queryString {
			if c == '?' {
				processedQuery.WriteString(fmt.Sprintf("$%d", whereCounter))
				whereCounter++
			} else {
				processedQuery.WriteRune(c)
			}
		}
		buf.WriteString(processedQuery.String())
	}
	return buf.String()
}
