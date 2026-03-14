//go:build amd64

package op

func absI64Vec(src, dst []int64) {
	for i := 0; i < len(src); i++ {
		m := src[i] >> 63
		dst[i] = (src[i] + m) ^ m
	}
}

//go:noescape
func absI32Vec(src, dst []int32)

//go:noescape
func negI64Vec(src, dst []int64)

//go:noescape
func negI32Vec(src, dst []int32)

//go:noescape
func sqI32Vec(src, dst []int32)

func sqI64Vec(src, dst []int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * src[i]
	}
}

//go:noescape
func sqrtI64Vec(src []int64, dst []float64)

//go:noescape
func sqrtI32Vec(src []int32, dst []float32)

//go:noescape
func recipI64Vec(src []int64, dst []float64)

//go:noescape
func recipI32Vec(src []int32, dst []float32)
