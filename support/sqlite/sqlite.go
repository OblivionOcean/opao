package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"unsafe"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/utils"
)

type Sqlite struct {
	Table   string
	err     error
	Elems   []*support.Elem
	conn    *sql.DB
	obj     any
	objType reflect.Type
}

func NewSqlite(conn *sql.DB, obj any, objType reflect.Type, table string, Elems []*support.Elem, err error) support.ObjectORM {
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
	tagNames, values := []string{}, []any{}
	for _, val := range qt.Elems {
		if val.Get() == nil {
			continue
		}
		tagNames = append(tagNames, "\""+val.Tag+"\"")
		values = append(values, val.Get())
	}
	values = append(values, queryValue...)
	_, err := qt.conn.Exec("UPDATE \""+qt.Table+"\" SET "+strings.Join(tagNames, "=?,")+"=? WHERE "+queryString, values...)
	return err
}

// Delete deletes database records based on the provided query string and values
func (qt *Sqlite) Delete(queryString string, queryValue ...any) error {
	_, err := qt.conn.Exec("DELETE FROM \""+qt.Table+"\" WHERE "+queryString, queryValue...)
	return err
}

// Insert inserts data into the database using INSERT operation
func (qt *Sqlite) Create() error {
	tagNames, values := []string{}, []any{}
	for _, val := range qt.Elems {
		if val.Get() == nil {
			continue
		}
		tagNames = append(tagNames, "\""+val.Tag+"\"")
		values = append(values, val.Get())
	}
	_, err := qt.conn.Exec("INSERT INTO \""+qt.Table+"\" ("+strings.Join(tagNames, ",")+") VALUES ("+strings.Repeat("?,", len(tagNames)-1)+"?);", values...)
	return err
}

// Select retrieves data from the database based on the provided query string and values
func (qt *Sqlite) FindAll(queryString string, queryValue ...any) ([]any, error) {
	// Retrieve objtype type through reflection, then query the data and convert it to the type of objType
	tagNames := []string{}
	for _, val := range qt.Elems {
		tagNames = append(tagNames, "\""+val.Tag+"\"")
	}
	rows, err := qt.conn.Query("SELECT * FROM \""+qt.Table+"\" WHERE "+queryString, queryValue...)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)
	var objs []any
	for rows.Next() {
		obj := reflect.New(qt.objType).Elem()
		columns, _ := rows.Columns()
		Scans := make([]any, len(columns))
		values := make([]any, len(columns))
		row := make(map[string]any, len(columns))
		for e := 0; e < len(columns); e++ {
			Scans[e] = &values[e]
			row[columns[e]] = &values[e]
		}
		err := rows.Scan(Scans...)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(qt.Elems); i++ {
			field := obj.Field(qt.Elems[i].Index)
			fieldPtr := unsafe.Pointer(field.UnsafeAddr())
			newFieldValue := reflect.NewAt(field.Type(), fieldPtr).Elem()
			val := autotType(qt.Elems[i].Type.Kind(), row[qt.Elems[i].Tag])
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
	objs, err := qt.FindAll(queryString, queryValue...)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, errors.New("empty result")
	}
	objV := reflect.ValueOf(qt.obj).Elem()
	newObjV := reflect.New(qt.objType).Elem()
	newObjV.Set(reflect.ValueOf(objs[0]))
	for i := 0; i < objV.NumField(); i++ {
		newVal, val := newObjV.Field(i), objV.Field(i)
		if !utils.ValIsNil(val) || !newVal.CanAddr() || !val.CanAddr() || utils.ValIsNil(newObjV.Field(i)) {
			continue
		}
		newVal = reflect.NewAt(newVal.Type(), unsafe.Pointer(newVal.UnsafeAddr())).Elem()
		val = reflect.NewAt(val.Type(), unsafe.Pointer(val.UnsafeAddr())).Elem()
		val.Set(newVal)
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
