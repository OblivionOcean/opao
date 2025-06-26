package mysql

import (
	"bytes"
	"unsafe"

	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/utils"
)

// go:inline
func (qt *MySQL) buildQuery(queryParts ...any) (string, []any) {
	if len(queryParts) == 0 {
		return "", nil
	}
	if len(queryParts) == 1 && queryParts[0] != nil {
		if cond, ok := queryParts[0].(support.Condition); ok {
			buf := &bytes.Buffer{}
			buf.Grow(128)
			args := make([]any, 0, utils.GetValueNum(cond))
			args = qt.parseQuery(buf, args, cond)
			bufByte := buf.Bytes()
			return unsafe.String(&bufByte[0], len(bufByte)), args
		} else {
			return queryParts[0].(string), nil
		}
	}
	return queryParts[0].(string), queryParts[1:]
}

func (qt *MySQL) parseQuery(buf *bytes.Buffer, args []any, cond support.Condition) []any {
	// 常量大驼峰
	switch cond.Type {
	case support.AND:
		for i := 0; i < len(cond.Args); i++ {
			arg := cond.Args[i]
			tmp, arg := utils.ParseConditionArg(qt.obj, qt.objType, qt.Elems, arg)
			if tmp != "" {
				buf.WriteString(tmp)
				buf.WriteString(" = ?")
				args = append(args, arg)
			} else if condition, ok := arg.(support.Condition); ok {
				args = qt.parseQuery(buf, args, condition)

			}
			if i < len(cond.Args)-1 {
				buf.WriteString(" AND ")
			}
		}
	case support.OR:
		for i := 0; i < len(cond.Args); i++ {
			arg := cond.Args[i]
			tmp, arg := utils.ParseConditionArg(qt.obj, qt.objType, qt.Elems, arg)
			if tmp != "" {
				buf.WriteString(tmp)
				buf.WriteString(" = ?")
				args = append(args, arg)
			} else if condition, ok := arg.(support.Condition); ok {
				args = qt.parseQuery(buf, args, condition)
			}
			if i < len(cond.Args)-1 {
				buf.WriteString(" OR ")
			}
		}
	case support.NOT:
		buf.WriteString("NOT (")
		tmp, arg := utils.ParseConditionArg(qt.obj, qt.objType, qt.Elems, cond.Left)
		if tmp != "" {
			buf.WriteString(tmp)
			buf.WriteString(" = ?")
			args = append(args, arg)
		} else if condition, ok := arg.(support.Condition); ok {
			args = qt.parseQuery(buf, args, condition)

		}
		buf.WriteString(")")
	case support.EQ, support.NE, support.LT, support.LTE, support.GT, support.GTE:
		if cond.Left == nil || cond.Right == nil {
			panic("Comparison condition must have exactly 2 arguments")
		}
		left := cond.Left.(string)
		right := cond.Right
		switch cond.Type {
		case support.EQ:
			buf.WriteString(left)
			buf.WriteString(" = ?")
		case support.NE:
			buf.WriteString(left)
			buf.WriteString(" <> ?")
		case support.LT:
			buf.WriteString(left)
			buf.WriteString(" < ?")
		case support.LTE:
			buf.WriteString(left)
			buf.WriteString(" <= ?")
		case support.GT:
			buf.WriteString(left)
			buf.WriteString(" > ?")
		case support.GTE:
			buf.WriteString(left)
			buf.WriteString(" >= ?")
		default:
			panic("UNKNOWN COMPARISON CONDITION TYPE")
		}
		args = append(args, right)
	case support.IN, support.NOT_IN, support.LIKE, support.NOT_LIKE:
		if cond.Left == nil || cond.Right == nil {
			panic("Comparison condition must have exactly 2 arguments")
		}
		left := cond.Left.(string)
		right := cond.Right
		switch cond.Type {
		case support.IN:
			buf.WriteString(left)
			buf.WriteString(" IN (?)")
		case support.NOT_IN:
			buf.WriteString(left)
			buf.WriteString(" NOT IN (?)")
		case support.LIKE:
			buf.WriteString(left)
			buf.WriteString(" LIKE ?")
		case support.NOT_LIKE:
			buf.WriteString(left)
			buf.WriteString(" NOT LIKE ?")
		default:
			panic("UNKNOWN COMPARISON CONDITION TYPE")
		}
		args = append(args, right)
	case support.BETWEEN, support.NOT_BETWEEN:
		if len(cond.Args) != 3 {
			panic("BETWEEN condition must have exactly 3 arguments")
		}
		left := cond.Left.(string)
		start := cond.Args[0]
		end := cond.Args[1]
		switch cond.Type {
		case support.BETWEEN:
			buf.WriteString(left)
			buf.WriteString(" BETWEEN ? AND ?")
		case support.NOT_BETWEEN:
			buf.WriteString(left)
			buf.WriteString(" NOT BETWEEN ? AND ?")
		default:
			panic("UNKNOWN BETWEEN CONDITION TYPE")
		}
		args = append(args, start, end)
	case support.EXISTS, support.NOT_EXISTS:
		if cond.Left == nil {
			panic("EXISTS condition must have exactly 1 argument")
		}
		switch cond.Type {
		case support.EXISTS:
			buf.WriteString("EXISTS (" + cond.Left.(string))
			buf.WriteString(")")
		case support.NOT_EXISTS:
			buf.WriteString("NOT EXISTS (" + cond.Left.(string))
			buf.WriteString(")")
		default:
			panic("UNKNOWN EXISTS CONDITION TYPE")
		}
	case support.IN_SUBQUERY, support.NOT_IN_SUBQUERY:
		if cond.Left == nil || cond.Right == nil {
			panic("IN_SUBQUERY condition must have exactly 2 arguments")
		}
		left := cond.Left.(string)
		subquery := cond.Right.(string)
		switch cond.Type {
		case support.IN_SUBQUERY:
			buf.WriteString(left + " IN (" + subquery)
			buf.WriteString(")")
		case support.NOT_IN_SUBQUERY:
			buf.WriteString(left + " NOT IN (" + subquery)
			buf.WriteString(")")
		default:
			panic("UNKNOWN IN_SUBQUERY CONDITION TYPE")
		}
	case support.IN_VALUES, support.NOT_IN_VALUES:
		if cond.Left == nil || len(cond.Args) == 0 {
			panic("IN_VALUES condition must have at least 2 arguments")
		}
		left := cond.Left.(string)
		values := cond.Args
		switch cond.Type {
		case support.IN_VALUES:
			buf.WriteString(left)
			buf.WriteString(" IN (")
		case support.NOT_IN_VALUES:
			buf.WriteString(left)
			buf.WriteString(" NOT IN (")
		default:
			panic("UNKNOWN IN_VALUES CONDITION TYPE")
		}
		for i, value := range values {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("?")
			args = append(args, value)
		}
		buf.WriteString(")")
	case support.LIMIT:
		if cond.Left == nil {
			return args
		}
		if cond.Right == nil && cond.Left != nil {
			buf.WriteString(" LIMIT ?")
			args = append(args, cond.Left)
		} else if cond.Left != nil && cond.Right != nil {
			buf.WriteString(" LIMIT ?, ?")
			args = append(args, cond.Left, cond.Right)
		} else {
			panic("LIMIT condition must have 1 or 2 arguments")
		}
	case support.CUSTOM:
		buf.WriteString(cond.Args[0].(string))
		if len(cond.Args) > 1 {
			args = append(args, cond.Args[1:]...)
		}
	default:
		panic("UNKNOWN CONDITION TYPE")
	}
	return args
}
