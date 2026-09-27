package cmpop

// EqI32Vec compares corresponding elements in src1 and src2 for equality and
// places the bit-packed results in dst.
func EqI32Vec(src1, src2 []int32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] == src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// NeqI32Vec compares corresponding elements in src1 and src2 for inequality
// and places the bit-packed results in dst.
func NeqI32Vec(src1, src2 []int32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] != src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LtI32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than results in dst.
func LtI32Vec(src1, src2 []int32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] < src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LeI32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than-or-equal results in dst.
func LeI32Vec(src1, src2 []int32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] <= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GtI32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than results in dst.
func GtI32Vec(src1, src2 []int32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] > src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GeI32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than-or-equal results in dst.
func GeI32Vec(src1, src2 []int32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] >= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// EqI64Vec compares corresponding elements in src1 and src2 for equality and
// places the bit-packed results in dst.
func EqI64Vec(src1, src2 []int64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] == src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// NeqI64Vec compares corresponding elements in src1 and src2 for inequality
// and places the bit-packed results in dst.
func NeqI64Vec(src1, src2 []int64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] != src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LtI64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than results in dst.
func LtI64Vec(src1, src2 []int64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] < src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LeI64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than-or-equal results in dst.
func LeI64Vec(src1, src2 []int64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] <= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GtI64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than results in dst.
func GtI64Vec(src1, src2 []int64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] > src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GeI64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than-or-equal results in dst.
func GeI64Vec(src1, src2 []int64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] >= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// EqF32Vec compares corresponding elements in src1 and src2 for equality and
// places the bit-packed results in dst.
func EqF32Vec(src1, src2 []float32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] == src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// NeqF32Vec compares corresponding elements in src1 and src2 for inequality
// and places the bit-packed results in dst.
func NeqF32Vec(src1, src2 []float32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] != src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LtF32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than results in dst.
func LtF32Vec(src1, src2 []float32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] < src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LeF32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than-or-equal results in dst.
func LeF32Vec(src1, src2 []float32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] <= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GtF32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than results in dst.
func GtF32Vec(src1, src2 []float32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] > src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GeF32Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than-or-equal results in dst.
func GeF32Vec(src1, src2 []float32, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] >= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// EqF64Vec compares corresponding elements in src1 and src2 for equality and
// places the bit-packed results in dst.
func EqF64Vec(src1, src2 []float64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] == src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// NeqF64Vec compares corresponding elements in src1 and src2 for inequality
// and places the bit-packed results in dst.
func NeqF64Vec(src1, src2 []float64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] != src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LtF64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than results in dst.
func LtF64Vec(src1, src2 []float64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] < src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// LeF64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed less-than-or-equal results in dst.
func LeF64Vec(src1, src2 []float64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] <= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GtF64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than results in dst.
func GtF64Vec(src1, src2 []float64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] > src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

// GeF64Vec compares corresponding elements in src1 and src2 and places the
// bit-packed greater-than-or-equal results in dst.
func GeF64Vec(src1, src2 []float64, dst []byte) {
	clear(dst[:(len(src1)+7)/8])
	for i := range src1 {
		if src1[i] >= src2[i] {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}
