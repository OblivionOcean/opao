// Package runtime 包含与 Go 运行时相关的类型和函数
package runtime

import (
	"reflect"
	"strconv"
	"unsafe"
)

// 定义一个与 reflect.rtype 结构体相似的结构体，以便我们可以访问非导出的字段
type rtype struct{}

type structType struct {
	PkgPath Name
	Fields  []structField
}

type structField struct {
	Name   Name
	Typ    *rtype
	Offset uintptr
}

// GetField 获取反射类型的结构体字段
//
//go:inline
func GetField(sf *reflect.StructField, st *structType, i int) bool {
	if st == nil || i < 0 || i >= len(st.Fields) {
		return false
	}
	stf := &st.Fields[i]
	sf.Name = stf.Name.Name()
	sf.Type = toType(stf.Typ)
	sf.Offset = stf.Offset
	sf.Anonymous = stf.Name.IsEmbedded()
	if tag := stf.Name.Tag(); tag != "" {
		sf.Tag = reflect.StructTag(tag)
	}
	if !stf.Name.IsExported() {
		sf.PkgPath = st.PkgPath.Name()
	}
	return true
}

//go:linkname toType reflect.toType
//go:noescape
func toType(t *rtype) reflect.Type

// RType2Type 将 *rtype 转换为 reflect.Type
//
//go:inline
func RType2Type(t *rtype) reflect.Type {
	return toType(t)
}

//go:inline
func TypeFieldLen(st *structType) int {
	return len(st.Fields)
}

func Type2StructType(t reflect.Type) *structType {
	if t.Kind() != reflect.Struct {
		return nil
	}
	return (*structType)(unsafe.Pointer((*[2]uintptr)(unsafe.Pointer(&t))[1] + abiTypeSize))
}

func GetTag(tag reflect.StructTag, key string) (value string, ok bool) {
	// When modifying this code, also update the validateStructTag code
	// in cmd/vet/structtag.go.

	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// Scan quoted string to find value.
		needUnquote := false
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				needUnquote = true
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		tmp := tag[:i+1]
		qvalue := string(tmp)
		tag = tag[i+1:]

		if key == name {
			if needUnquote {
				value, err := strconv.Unquote(qvalue)
				if err != nil {
					break
				}
				return value, true
			}
			return qvalue[1 : len(qvalue)-1], true
		}
	}
	return "", false
}
