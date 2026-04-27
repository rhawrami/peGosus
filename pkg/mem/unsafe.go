package mem

import "unsafe"

const alignSize int = 64

func makeAlignedSlice(l int) []byte {
	if l <= 0 || l%alignSize != 0 {
		return nil
	}

	s := make([]byte, l+alignSize)
	offBy := int(uintptr(unsafe.Pointer(&s[0])) & uintptr(alignSize-1))
	start := 0
	if offBy != 0 {
		start = 64 - offBy
	}

	s = s[start : start+l]
	return s
}

func isAligned(addr *byte, a int) bool {
	return uintptr(unsafe.Pointer(addr))&uintptr(a-1) == 0
}

func asBT(ptr *byte, length int) []byte {
	return unsafe.Slice(ptr, length)
}

func asI64T(ptr *byte, length int) []int64 {
	return unsafe.Slice((*int64)(unsafe.Pointer(ptr)), length)
}

func asI32T(ptr *byte, length int) []int32 {
	return unsafe.Slice((*int32)(unsafe.Pointer(ptr)), length)
}

func asF64T(ptr *byte, length int) []float64 {
	return unsafe.Slice((*float64)(unsafe.Pointer(ptr)), length)
}

func asF32T(ptr *byte, length int) []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(ptr)), length)
}

func asU32T(ptr *byte, length int) []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(ptr)), length)
}
