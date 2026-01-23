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

package runtime

import "unsafe"

func addChecked(p unsafe.Pointer, x uintptr, whySafe string) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

// Name is an encoded type Name with optional extra data.
//
// The first byte is a bit field containing:
//
//	1<<0 the name is exported
//	1<<1 tag data follows the name
//	1<<2 pkgPath nameOff follows the name and tag
//	1<<3 the name is of an embedded (a.k.a. anonymous) field
//
// Following that, there is a varint-encoded length of the name,
// followed by the name itself.
//
// If tag data is present, it also has a varint-encoded length
// followed by the tag itself.
//
// If the import path follows, then 4 bytes at the end of
// the data form a nameOff. The import path is only set for concrete
// methods that are defined in a different package than their type.
//
// If a name starts with "*", then the exported bit represents
// whether the pointed to type is exported.
//
// Note: this encoding must match here and in:
//   cmd/compile/internal/reflectdata/reflect.go
//   cmd/link/internal/ld/decodesym.go

type Name struct {
	Bytes *byte
}

// DataChecked does pointer arithmetic on n's Bytes, and that arithmetic is asserted to
// be safe for the reason in whySafe (which can appear in a backtrace, etc.)
func (n Name) DataChecked(off int, whySafe string) *byte {
	return (*byte)(addChecked(unsafe.Pointer(n.Bytes), uintptr(off), whySafe))
}

// Data does pointer arithmetic on n's Bytes, and that arithmetic is asserted to
// be safe because the runtime made the call (other packages use DataChecked)
func (n Name) Data(off int) *byte {
	return (*byte)(addChecked(unsafe.Pointer(n.Bytes), uintptr(off), "the runtime doesn't need to give you a reason"))
}

// IsExported returns "is n exported?"
func (n Name) IsExported() bool {
	return (*n.Bytes)&(1<<0) != 0
}

// HasTag returns true iff there is tag data following this name
func (n Name) HasTag() bool {
	return (*n.Bytes)&(1<<1) != 0
}

// IsEmbedded returns true iff n is embedded (an anonymous field).
func (n Name) IsEmbedded() bool {
	return (*n.Bytes)&(1<<3) != 0
}

// ReadVarint parses a varint as encoded by encoding/binary.
// It returns the number of encoded bytes and the encoded value.
func (n Name) ReadVarint(off int) (int, int) {
	v := 0
	for i := 0; ; i++ {
		x := *n.DataChecked(off+i, "read varint")
		v += int(x&0x7f) << (7 * i)
		if x&0x80 == 0 {
			return i + 1, v
		}
	}
}

// IsBlank indicates whether n is "_".
func (n Name) IsBlank() bool {
	if n.Bytes == nil {
		return false
	}
	_, l := n.ReadVarint(1)
	return l == 1 && *n.Data(2) == '_'
}

// writeVarint writes n to buf in varint form. Returns the
// number of bytes written. n must be nonnegative.
// Writes at most 10 bytes.
func writeVarint(buf []byte, n int) int {
	for i := 0; ; i++ {
		b := byte(n & 0x7f)
		n >>= 7
		if n == 0 {
			buf[i] = b
			return i + 1
		}
		buf[i] = b | 0x80
	}
}

// Name returns the tag string for n, or empty if there is none.
func (n Name) Name() string {
	if n.Bytes == nil {
		return ""
	}
	i, l := n.ReadVarint(1)
	return unsafe.String(n.DataChecked(1+i, "non-empty string"), l)
}

// Tag returns the tag string for n, or empty if there is none.
func (n Name) Tag() string {
	if !n.HasTag() {
		return ""
	}
	i, l := n.ReadVarint(1)
	i2, l2 := n.ReadVarint(1 + i + l)
	return unsafe.String(n.DataChecked(1+i+l+i2, "non-empty string"), l2)
}

func NewName(n, tag string, exported, embedded bool) Name {
	if len(n) >= 1<<29 {
		panic("abi.NewName: name too long: " + n[:1024] + "...")
	}
	if len(tag) >= 1<<29 {
		panic("abi.NewName: tag too long: " + tag[:1024] + "...")
	}
	var nameLen [10]byte
	var tagLen [10]byte
	nameLenLen := writeVarint(nameLen[:], len(n))
	tagLenLen := writeVarint(tagLen[:], len(tag))

	var bits byte
	l := 1 + nameLenLen + len(n)
	if exported {
		bits |= 1 << 0
	}
	if len(tag) > 0 {
		l += tagLenLen + len(tag)
		bits |= 1 << 1
	}
	if embedded {
		bits |= 1 << 3
	}

	b := make([]byte, l)
	b[0] = bits
	copy(b[1:], nameLen[:nameLenLen])
	copy(b[1+nameLenLen:], n)
	if len(tag) > 0 {
		tb := b[1+nameLenLen+len(n):]
		copy(tb, tagLen[:tagLenLen])
		copy(tb[tagLenLen:], tag)
	}

	return Name{Bytes: &b[0]}
}
