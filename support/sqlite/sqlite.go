package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"unsafe"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/utils"
)

type Sqlite struct {
	Table   string
	err     error
	Elems   []support.Elem
	conn    *sql.DB
	obj     any
	objType reflect.Type
}

func NewSqlite(conn *sql.DB, obj any, objType reflect.Type, table string, Elems []support.Elem, err error) support.ObjectORM {
	if err != nil {
		return &Sqlite{err: err}
	}
	return &Sqlite{Table: table, Elems: Elems, err: err, conn: conn, obj: obj, objType: objType}
}

func (qt *Sqlite) Error() error {
	return qt.err
}

// Update updates the database record based on the provided query string and values
func (qt *Sqlite) Update(queryString string, queryValue ...any) error {
	// Fetch tag and corresponding stored data
	elemsLeng := len(qt.Elems)
	tabNameLen := len(qt.Table)
	queryStringLen := len(queryString)
	elemsNameLength := 0
	for i := 0; i < elemsLeng; i++ {
		elemsNameLength += len(qt.Elems[i].Tag) + 4
		if i != elemsLeng-1 {
			elemsNameLength += 1
		}
	}
	var tmp []byte
	if queryString == "" {
		tmp = make([]byte, 0, 15+tabNameLen+elemsNameLength)
	} else {
		tmp = make([]byte, 0, 21+tabNameLen+queryStringLen+elemsNameLength)
	}
	tmp = append(tmp, "UPDATE \""...)
	tmp = append(tmp, qt.Table...)
	tmp = append(tmp, "\" SET "...)

	values := make([]any, elemsLeng+len(queryValue))
	for i := 0; i < elemsLeng; i++ {
		tmp = append(tmp, '"')

		tmp = append(tmp, qt.Elems[i].Tag...)

		tmp = append(tmp, "\"=?"...)

		if i != elemsLeng-1 {
			tmp = append(tmp, ',')

		}
		values[i] = qt.Elems[i].Get()
	}
	if queryString != "" {
		tmp = append(tmp, " WHERE "...)
		tmp = append(tmp, queryString...)
		copy(values[elemsLeng:], queryValue)

	}
	_, err := qt.conn.Exec(utils.Bytes2String(tmp), values...)
	return err
}

// Delete deletes database records based on the provided query string and values
func (qt *Sqlite) Delete(queryString string, queryValue ...any) error {
	tabNameLen := len(qt.Table)
	queryStringLen := len(queryString)
	var tmp []byte
	if queryString == "" {
		tmp = make([]byte, 0, 14+tabNameLen)
	} else {
		tmp = make([]byte, 0, 20+tabNameLen+queryStringLen)
	}
	tmp = append(tmp, "DELETE FROM \""...)
	tmp = append(tmp, qt.Table...)
	tmp[13+tabNameLen] = '"'
	if queryString != "" {
		tmp = append(tmp, " WHERE "...)
		tmp = append(tmp, queryString...)
	}
	_, err := qt.conn.Exec(utils.Bytes2String(tmp), queryValue...)
	return err
}

// Insert inserts data into the database using INSERT operation
func (qt *Sqlite) Create() error {
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
	tmp = append(tmp, "INSERT INTO \""...)
	tmp = append(tmp, qt.Table...)
	tmp = append(tmp, "\" ("...)

	values := make([]any, elemsLeng)
	for i := 0; i < elemsLeng; i++ {
		tmp = append(tmp, '"')

		tmp = append(tmp, qt.Elems[i].Tag...)

		tmp = append(tmp, '"')

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
	_, err := qt.conn.Exec(utils.Bytes2String(tmp), values...)
	return err
}

// Select retrieves data from the database based on the provided query string and values
func (qt *Sqlite) FindAll(queryString string, queryValue ...any) ([]any, error) {
	// Retrieve objtype type through reflection, then query the data and convert it to the type of objType
	rows, err := qt.conn.Query(qt.getSelectSql(queryString), queryValue...)
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
		err := rows.Scan(Scans...)
		if err != nil {
			return nil, err
		}
		for i := 0; i < elemsLen; i++ {
			field := obj.Field(qt.Elems[i].Index)
			newFieldValue := reflect.NewAt(qt.Elems[i].Type, unsafe.Pointer(field.UnsafeAddr())).Elem()
			val := autotType(qt.Elems[i].Type.Kind(), Scans[i].(*any))
			if val == nil {
				continue
			}
			newFieldValue.Set(reflect.ValueOf(utils.AutotAnyI64tAnyI(val)))
		}
		objs = append(objs, obj.Interface())
	}
	return objs, nil
}

// SelectOne selects a single record from the database based on the provided query string and values
func (qt *Sqlite) Find(queryString string, queryValue ...any) (any, error) {
	rows, err := qt.conn.Query(qt.getSelectSql(queryString), queryValue...)
	elemsLen := len(qt.Elems)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)
	Scans := make([]any, elemsLen)
	err = rows.Scan(Scans...)
	if err != nil {
		return nil, err
	}
	for i := 0; i < elemsLen; i++ {
		val := autotType(qt.Elems[i].Type.Kind(), Scans[i].(*any))
		if val == nil {
			continue
		}
		qt.Elems[i].Set(reflect.ValueOf(utils.AutotAnyI64tAnyI(val)))
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

func (qt *Sqlite) getSelectSql(queryString string) string {
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
		tmp = append(tmp, '"')

		tmp = append(tmp, qt.Elems[i].Tag...)

		tmp = append(tmp, '"')

		if i != elemsLeng-1 {
			tmp = append(tmp, ',')

		}
	}
	tmp = append(tmp, " FROM \""...)

	tmp = append(tmp, qt.Table...)

	if queryString != "" {
		tmp = append(tmp, "\" WHERE "...)

		tmp = append(tmp, queryString...)
	}
	return utils.Bytes2String(tmp)
}
