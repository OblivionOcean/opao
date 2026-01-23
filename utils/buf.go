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

package utils

type Buffer []byte

func NewBuffer(cap int) Buffer {
	return make(Buffer, 0, cap)
}

func (b *Buffer) Write(n ...byte) {
	*b = append(*b, n...)
}

func (b *Buffer) WriteString(s string) {
	*b = append(*b, s...)
}

func (b *Buffer) WriteByte(c byte) {
	*b = append(*b, c)
}

func (b Buffer) String() string {
	return Bytes2String(b)
}

func (b *Buffer) TruncateLast(n int) {
	if n <= 0 || len(*b) < n {
		return
	}
	*b = (*b)[:len(*b)-n]
}
