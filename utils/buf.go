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
