package numop

const (
	minI32Float = -0x1p31
	maxI32Float = 0x1p31
	minI64Float = -0x1p63
	maxI64Float = 0x1p63
)

// CastI64ToI32 casts int64 elements in src to int32 with wrapping narrowing,
// placing the result in dst.
func CastI64ToI32(src []int64, dst []int32) {
	for i, value := range src {
		dst[i] = int32(value)
	}
}

// CastF64ToF32 casts float64 elements in src to float32, placing the result in
// dst.
func CastF64ToF32(src []float64, dst []float32) {
	for i, value := range src {
		dst[i] = float32(value)
	}
}

// CastF32ToI32Checked casts float32 elements in the half-open int32 source
// range. Invalid payload elements are zero and validity is an LSB-first bitmap.
func CastF32ToI32Checked(src []float32, dst []int32, validity []byte) {
	clear(validity[:(len(src)+7)/8])
	for i, value := range src {
		if value >= minI32Float && value < maxI32Float {
			dst[i] = int32(value)
			validity[i/8] |= 1 << (i % 8)
		} else {
			dst[i] = 0
		}
	}
}

// CastF32ToI64Checked casts float32 elements in the half-open int64 source
// range. Invalid payload elements are zero and validity is an LSB-first bitmap.
func CastF32ToI64Checked(src []float32, dst []int64, validity []byte) {
	clear(validity[:(len(src)+7)/8])
	for i, value := range src {
		if value >= minI64Float && value < maxI64Float {
			dst[i] = int64(value)
			validity[i/8] |= 1 << (i % 8)
		} else {
			dst[i] = 0
		}
	}
}

// CastF64ToI32Checked casts float64 elements in the half-open int32 source
// range. Invalid payload elements are zero and validity is an LSB-first bitmap.
func CastF64ToI32Checked(src []float64, dst []int32, validity []byte) {
	clear(validity[:(len(src)+7)/8])
	for i, value := range src {
		if value >= minI32Float && value < maxI32Float {
			dst[i] = int32(value)
			validity[i/8] |= 1 << (i % 8)
		} else {
			dst[i] = 0
		}
	}
}

// CastF64ToI64Checked casts float64 elements in the half-open int64 source
// range. Invalid payload elements are zero and validity is an LSB-first bitmap.
func CastF64ToI64Checked(src []float64, dst []int64, validity []byte) {
	clear(validity[:(len(src)+7)/8])
	for i, value := range src {
		if value >= minI64Float && value < maxI64Float {
			dst[i] = int64(value)
			validity[i/8] |= 1 << (i % 8)
		} else {
			dst[i] = 0
		}
	}
}
