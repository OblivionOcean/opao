package mysql

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"unsafe"

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
func (qt *MySQL) Update(queryString string, queryValue ...any) error {
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
		tmp = make([]byte, 15+tabNameLen+elemsNameLength)
	} else {
		tmp = make([]byte, 21+tabNameLen+queryStringLen+elemsNameLength)
	}
	copy(tmp[:8], "UPDATE `")
	copy(tmp[8:8+tabNameLen], qt.Table)
	copy(tmp[8+tabNameLen:15+tabNameLen], "` SET ")
	count := 15 + tabNameLen
	values := make([]any, elemsLeng+len(queryValue))
	for i := 0; i < elemsLeng; i++ {
		tmp[count] = '`'
		count++
		copy(tmp[count:count+len(qt.Elems[i].Tag)], qt.Elems[i].Tag)
		count += len(qt.Elems[i].Tag)
		copy(tmp[count:count+3], "`=?")
		count += 3
		if i != elemsLeng-1 {
			tmp[count] = ','
			count++
		}
		values[i] = qt.Elems[i].Get()
	}
	if queryString != "" {
		copy(tmp[count:count+6], " WHERE ")
		copy(tmp[count+6:], queryString)
		copy(values[elemsLeng:], queryValue)

	}
	//_, err := qt.conn.Exec("DELETE FROM `"+qt.Table+"` WHERE "+queryString, queryValue...)
	var err error
	return err
}

// Delete deletes database records based on the provided query string and values
func (qt *MySQL) Delete(queryString string, queryValue ...any) error {
	tabNameLen := len(qt.Table)
	queryStringLen := len(queryString)
	var err error
	var tmp []byte
	if queryString == "" {
		tmp = make([]byte, 14+tabNameLen)
	} else {
		tmp = make([]byte, 20+tabNameLen+queryStringLen)
	}
	copy(tmp[:13], "DELETE FROM `")
	copy(tmp[13:13+tabNameLen], qt.Table)
	tmp[13+tabNameLen] = '`'
	if queryString != "" {
		copy(tmp[14+tabNameLen:20+tabNameLen], " WHERE ")
		copy(tmp[20+tabNameLen:], queryString)
	}
	//_, err := qt.conn.Exec("DELETE FROM `"+qt.Table+"` WHERE "+queryString, queryValue...)
	return err
}

// Insert inserts data into the database using INSERT operation
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
	tmp := make([]byte, 29+tabNameLen+elemsNameLength)
	copy(tmp[:13], "INSERT INTO `")
	copy(tmp[13:13+tabNameLen], qt.Table)
	copy(tmp[13+tabNameLen:16+tabNameLen], "` (")
	count := 16 + tabNameLen
	values := make([]any, elemsLeng)
	for i := 0; i < elemsLeng; i++ {
		tmp[count] = '`'
		count++
		copy(tmp[count:count+len(qt.Elems[i].Tag)], qt.Elems[i].Tag)
		count += len(qt.Elems[i].Tag)
		tmp[count] = '`'
		count++
		values[i] = qt.Elems[i].Get()
		if i != elemsLeng-1 {
			tmp[count] = ','
			count++
		}
	}
	copy(tmp[count:count+10], ") VALUES (")
	count += 10
	for i := 0; i < elemsLeng; i++ {
		tmp[count] = '?'
		count++
		if i != elemsLeng-1 {
			tmp[count] = ','
			count++
		}
	}
	copy(tmp[count:count+2], ");")
	utils.Used(tmp)
	//fmt.Println(utils.Bytes2String(tmp))
	var err error
	return err
}

// Select retrieves data from the database based on the provided query string and values
func (qt *MySQL) FindAll(queryString string, queryValue ...any) ([]any, error) {
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
func (qt *MySQL) Find(queryString string, queryValue ...any) (any, error) {
	rows, err := qt.conn.Query(qt.getSelectSql(queryString), queryValue...)
	elemsLen := len(qt.Elems)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)
	if err != nil {
		return nil, err
	}
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

func (qt *MySQL) getSelectSql(queryString string) string {
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
		tmp = make([]byte, 13+tabNameLen+elemsNameLength)
	} else {
		tmp = make([]byte, 20+tabNameLen+queryStringLen+elemsNameLength)
	}
	copy(tmp[:6], "SELECT ")
	count := 6
	for i := 0; i < elemsLeng; i++ {
		tmp[count] = '`'
		count++
		copy(tmp[count:count+len(qt.Elems[i].Tag)], qt.Elems[i].Tag)
		count += len(qt.Elems[i].Tag)
		tmp[count] = '`'
		count++
		if i != elemsLeng-1 {
			tmp[count] = ','
			count++
		}
	}
	copy(tmp[count:count+7], " FROM `")
	count += 7
	copy(tmp[count:count+tabNameLen], qt.Table)
	count += tabNameLen
	tmp[count] = '`'
	count++
	if queryString != "" {
		copy(tmp[count:count+20], "` WHERE ")
		count += 20
		copy(tmp[count:count+queryStringLen], queryString)
	}
	return utils.Bytes2String(tmp)
}
