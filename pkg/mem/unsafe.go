package mem

import "unsafe"

func bToI64(b []byte, l int) []int64 {
	if l == 0 {
		return nil
	}
	return unsafe.Slice((*int64)(unsafe.Pointer(&b[0])), l)
}

func bToI32(b []byte, l int) []int32 {
	if l == 0 {
		return nil
	}
	return unsafe.Slice((*int32)(unsafe.Pointer(&b[0])), l)
}

func bToF64(b []byte, l int) []float64 {
	if l == 0 {
		return nil
	}
	return unsafe.Slice((*float64)(unsafe.Pointer(&b[0])), l)
}

func bToF32(b []byte, l int) []float32 {
	if l == 0 {
		return nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(&b[0])), l)
}

func bToB(b []byte, l int) []byte {
	if l == 0 {
		return nil
	}
	return b[:l]
}
