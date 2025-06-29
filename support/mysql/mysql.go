package mysql

import (
	"database/sql"
	"database/sql/driver"
	"reflect"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/utils"
)

type MySQL struct {
	Table   string
	err     error
	Elems   []support.Elem
	conn    *sql.DB
	obj     any
	objType reflect.Type
}

func NewMySQL(conn *sql.DB, obj any, objType reflect.Type, table string, Elems []support.Elem, err error) support.ObjectORM {
	if err != nil {
		return &MySQL{err: err}
	}
	return &MySQL{Table: table, Elems: Elems, err: err, conn: conn, obj: obj, objType: objType}
}

func (qt *MySQL) Error() error {
	return qt.err
}

// Update updates the database record based on the provided query string and values
func (qt *MySQL) Update(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)
	// Fetch tag and corresponding stored data
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		elemsNameLength += len(qt.Elems[i].Tag) + 4
		if i != elemsLeng-1 {
			elemsNameLength += 1
		}
	}
	var tmp []byte
	if query == "" {
		tmp = make([]byte, 0, 15+tabNameLen+elemsNameLength)
	} else {
		tmp = make([]byte, 0, 21+tabNameLen+queryStringLen+elemsNameLength)
	}
	tmp = append(tmp, "UPDATE `"...)
	tmp = append(tmp, qt.Table...)
	tmp = append(tmp, "` SET "...)

	values := make([]any, elemsLeng+len(args))
	for i := 0; i < elemsLeng; i++ {
		tmp = append(tmp, '`')

		tmp = append(tmp, qt.Elems[i].Tag...)

		tmp = append(tmp, "`=?"...)

		if i != elemsLeng-1 {
			tmp = append(tmp, ',')

		}
		values[i] = qt.Elems[i].Get()
	}
	if query != "" {
		tmp = append(tmp, " WHERE "...)
		tmp = append(tmp, query...)
		copy(values[elemsLeng:], args)

	}
	r, err := qt.conn.Exec(utils.Bytes2String(tmp), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// Delete deletes database records based on the provided query string and values
func (qt *MySQL) Delete(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	var tmp []byte
	if query == "" {
		tmp = make([]byte, 0, 14+tabNameLen)
	} else {
		tmp = make([]byte, 0, 20+tabNameLen+queryStringLen)
	}
	tmp = append(tmp, "DELETE FROM `"...)
	tmp = append(tmp, qt.Table...)
	tmp[13+tabNameLen] = '`'
	if query != "" {
		tmp = append(tmp, " WHERE "...)
		tmp = append(tmp, query...)
	}
	_, err := qt.conn.Exec(utils.Bytes2String(tmp), args...)
	return err
}

// Create inserts data into the database using INSERT operation
func (qt *MySQL) Create() error {
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		elemsNameLength += len(qt.Elems[i].Tag) + 3
		if i != elemsLeng-1 {
			elemsNameLength += 2
		}
	}
	tmp := make([]byte, 0, 29+tabNameLen+elemsNameLength)
	tmp = append(tmp, "INSERT INTO `"...)
	tmp = append(tmp, qt.Table...)
	tmp = append(tmp, "` ("...)

	values := make([]any, elemsLeng)
	for i := 0; i < elemsLeng; i++ {
		tmp = append(tmp, '`')

		tmp = append(tmp, qt.Elems[i].Tag...)

		tmp = append(tmp, '`')

		values[i] = qt.Elems[i].Get()
		if i != elemsLeng-1 {
			tmp = append(tmp, ',')

		}
	}
	tmp = append(tmp, ") VALUES ("...)

	for i := 0; i < elemsLeng; i++ {
		tmp = append(tmp, '?')

		if i != elemsLeng-1 {
			tmp = append(tmp, ',')

		}
	}
	tmp = append(tmp, ");"...)
	r, err := qt.conn.Exec(utils.Bytes2String(tmp), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// FindAll retrieves data from the database based on the provided query string and values
func (qt *MySQL) FindAll(queryParts ...any) ([]any, error) {
	query, args := qt.buildQuery(queryParts...)
	// Retrieve objtype type through reflection, then query the data and convert it to the type of objType
	rows, err := qt.conn.Query(qt.getSelectSQL(query), args...)
	elemsLen := len(qt.Elems)
	if err != nil {
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
			Scans[i] = reflect.NewAt(qt.Elems[i].Type, obj.Index(qt.Elems[i].Index).Addr().UnsafePointer()).Interface()
		}
		err := rows.Scan(Scans...)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj.Interface())
	}
	return objs, nil
}

// Find retrieves a single record from the database based on the provided query string and values
func (qt *MySQL) Find(queryParts ...any) (any, error) {
	query, args := qt.buildQuery(queryParts...)
	rows, err := qt.conn.Query(qt.getSelectSQL(query), args...)
	elemsLen := len(qt.Elems)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return nil, err
		}
		// 如果没有数据，返回空对象
		if IsEmpty(err) {
			return nil, nil
		}
	}
	Scans := make([]any, elemsLen)
	for i := 0; i < elemsLen; i++ {
		Scans[i] = qt.Elems[i].GetPtr()
	}
	err = rows.Scan(Scans...)
	if err != nil {
		return nil, err
	}
	return qt.obj, nil
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

func (qt *MySQL) getSelectSQL(queryString string) string {
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
	var tmp []byte
	if queryString == "" {
		tmp = make([]byte, 0, 15+tabNameLen+elemsNameLength)
	} else {
		tmp = make([]byte, 0, 22+tabNameLen+queryStringLen+elemsNameLength)
	}
	tmp = append(tmp, "SELECT "...)
	for i := 0; i < elemsLeng; i++ {
		tmp = append(tmp, '`')

		tmp = append(tmp, qt.Elems[i].Tag...)

		tmp = append(tmp, '`')

		if i != elemsLeng-1 {
			tmp = append(tmp, ',')

		}
	}
	tmp = append(tmp, " FROM `"...)

	tmp = append(tmp, qt.Table...)

	tmp = append(tmp, "`"...)
	if queryString != "" {
		tmp = append(tmp, " WHERE "...)

		tmp = append(tmp, queryString...)
	}
	return utils.Bytes2String(tmp)
}
