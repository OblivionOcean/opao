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
	if elemsNameLength == 0 {
		return nil
	}
	var buf utils.Buffer
	if query == "-" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength)
	} else {
		buf = utils.NewBuffer(21 + tabNameLen + queryStringLen + elemsNameLength)
	}
	buf.WriteString("UPDATE `")
	buf.WriteString(qt.Table)
	buf.WriteString("` SET ")

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
		if i != elemsLeng-1 {
			buf.WriteByte(',')
		}
		values = append(values, qt.Elems[i].Get())
	}
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
		values = append(values, args)
	}
	r, err := qt.conn.Exec(buf.String(), values...)
	if err != nil {
		return err
	}
	support.WriteLii(qt.Elems, r)
	return err
}

// Update updates the database record based on the provided query string and values
func (qt *MySQL) Save(queryParts ...any) error {
	query, args := qt.buildQuery(queryParts...)
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
	if query == "-" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength)
	} else {
		buf = utils.NewBuffer(21 + tabNameLen + queryStringLen + elemsNameLength)
	}
	buf.WriteString("UPDATE `")
	buf.WriteString(qt.Table)
	buf.WriteString("` SET ")

	values := make([]any, 0, elemsLeng+len(args))
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		buf.WriteByte('`')
		buf.WriteString(qt.Elems[i].Tag)
		buf.WriteString("`=?")
		if i != elemsLeng-1 {
			buf.WriteByte(',')
		}
		values = append(values, qt.Elems[i].Get())
	}
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
		values = append(values, args)
	}
	r, err := qt.conn.Exec(buf.String(), values...)
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
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(14 + tabNameLen)
	} else {
		buf = utils.NewBuffer(20 + tabNameLen + queryStringLen)
	}
	buf.WriteString("DELETE FROM `")
	buf.WriteString(qt.Table)
	buf.WriteByte('`')
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
	}
	_, err := qt.conn.Exec(buf.String(), args...)
	return err
}

// Create inserts data into the database using INSERT operation
func (qt *MySQL) Create() error {
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
	buf.WriteString("INSERT INTO `")
	buf.WriteString(qt.Table)
	buf.WriteString("` (")

	values := make([]any, 0, elemsLeng)
	for i := 0; i < elemsLeng; i++ {
		if qt.Elems[i].Option["autoIncrement"] == "-" {
			continue
		}
		buf.WriteByte('`')

		buf.WriteString(qt.Elems[i].Tag)

		buf.WriteByte('`')

		values = append(values, qt.Elems[i].Get())
		if i != elemsLeng-1 {
			buf.WriteByte(',')

		}
	}
	buf.WriteString(") VALUES (")

	for i := 0; i < elemsLeng; i++ {
		buf.WriteByte('?')

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
func (qt *MySQL) FindAll(queryParts ...any) ([]any, error) {
	query, args := qt.buildQuery(queryParts...)
	// Retrieve objtype type through reflection, then query the data and convert it to the type of objType
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
func (qt *MySQL) Find(queryParts ...any) (any, error) {
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

func (qt *MySQL) Count(queryParts ...any) (int, error) {
	query, args := qt.buildQuery(queryParts...)
	tabNameLen := len(qt.Table)
	queryStringLen := len(query)
	var buf utils.Buffer
	if query == "" {
		buf = utils.NewBuffer(23 + tabNameLen)
	} else {
		buf = utils.NewBuffer(30 + tabNameLen + queryStringLen)
	}
	buf.WriteString("SELECT COUNT(*) FROM `")

	buf.WriteString(qt.Table)

	buf.WriteString("`")
	if query != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(query)
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
	var buf utils.Buffer
	if queryString == "" {
		buf = utils.NewBuffer(15 + tabNameLen + elemsNameLength)
	} else {
		buf = utils.NewBuffer(22 + tabNameLen + queryStringLen + elemsNameLength)
	}
	buf.WriteString("SELECT ")
	for i := 0; i < elemsLeng; i++ {
		buf.WriteByte('`')

		buf.WriteString(qt.Elems[i].Tag)

		buf.WriteByte('`')

		if i != elemsLeng-1 {
			buf.WriteByte(',')

		}
	}
	buf.WriteString(" FROM `")

	buf.WriteString(qt.Table)

	buf.WriteString("`")
	if queryString != "" {
		buf.WriteString(" WHERE ")

		buf.WriteString(queryString)
	}
	return buf.String()
}
