package opao

import (
	"database/sql"
	"database/sql/driver"
	_ "github.com/go-sql-driver/mysql"
	//_ "github.com/mattn/go-sqlite3"  // SQLite3 database driver
	"github.com/OblivionOcean/opao/utils"
	"reflect"
	"strings"
	"unsafe"
)

// Action represents a set of database operations
type Action struct {
	Table string           // Name of the table
	Db    *Database        // Reference to the Database instance
	Elems map[string]*Elem // Mapping of field names to Elem objects
	Cache *Cache           // Cache object for the table
	Obj   any              // Object
}

// Update updates the database record based on the provided query string and values
func (qt *Action) Update(queryString string, queryValue ...any) error {
	// Fetch tag and corresponding stored data
	tagNames, values := []string{}, []any{}
	for _, val := range qt.Cache.Elems {
		if val.Get() == nil {
			continue
		}
		tagNames = append(tagNames, "`"+val.Tag+"`")
		values = append(values, val.Get())
	}
	values = append(values, queryValue...)
	_, err := qt.Db.Db.Exec("UPDATE `"+qt.Table+"` SET "+strings.Join(tagNames, "=?,")+"=? WHERE "+queryString, values...)
	return err
}

// Delete deletes database records based on the provided query string and values
func (qt *Action) Delete(queryString string, queryValue ...any) error {
	_, err := qt.Db.Db.Exec("DELETE FROM `"+qt.Table+"` WHERE "+queryString, queryValue...)
	return err
}

// Insert inserts data into the database using INSERT operation
func (qt *Action) Insert() error {
	tagNames, values := []string{}, []any{}
	for _, val := range qt.Elems {
		if val.Get() == nil {
			continue
		}
		tagNames = append(tagNames, "`"+val.Tag+"`")
		values = append(values, val.Get())
	}
	_, err := qt.Db.Db.Exec("INSERT INTO `"+qt.Table+"` ("+strings.Join(tagNames, ",")+") VALUES ("+strings.Repeat("?,", len(tagNames)-1)+"?);", values...)
	return err
}

// Select retrieves data from the database based on the provided query string and values
func (qt *Action) Select(queryString string, queryValue ...any) ([]any, error) {
	// Retrieve objtype type through reflection, then query the data and convert it to the type of objType
	tagNames := []string{}
	for _, val := range qt.Elems {
		tagNames = append(tagNames, "`"+val.Tag+"`")
	}
	rows, err := qt.Db.Db.Query("SELECT * FROM `"+qt.Table+"` WHERE "+queryString, queryValue...)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)
	var objs []any
	for rows.Next() {
		obj := reflect.New(qt.Cache.ObjType).Elem()
		columns, _ := rows.Columns()
		Scans := make([]any, len(columns))
		values := make([]any, len(columns))
		row := map[string]any{}
		for e := 0; e < len(columns); e++ {
			Scans[e] = &values[e]
			row[columns[e]] = &values[e]
		}
		err := rows.Scan(Scans...)
		if err != nil {
			return nil, err
		}
		for k, v := range row {
			field := obj.FieldByName(qt.Elems[k].Name)
			fieldPtr := unsafe.Pointer(field.UnsafeAddr())
			newFieldValue := reflect.NewAt(field.Type(), fieldPtr).Elem()
			val := autotType(qt.Elems[k].Type.Type.Kind(), v)
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
func (qt *Action) SelectOne(queryString string, queryValue ...any) error {
	objs, err := qt.Select(queryString, queryValue...)
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return utils.NewError("Empty result")
	}
	objV := reflect.ValueOf(qt.Obj).Elem()
	newObjV := reflect.New(qt.Cache.ObjType).Elem()
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
	return nil
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
