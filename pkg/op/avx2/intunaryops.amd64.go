//go:build amd64

package op

func absI64(src, dst []int64) {
	for i := 0; i < len(src); i++ {
		m := src[i] >> 63
		dst[i] = (src[i] + m) ^ m
	}
}

//go:noescape
func absI32(src, dst []int32)

//go:noescape
func negI64(src, dst []int64)

//go:noescape
func negI32(src, dst []int32)

//go:noescape
func sqI32(src, dst []int32)

func sqI64(src, dst []int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * src[i]
	}
}

//go:noescape
func sqrtI64(src []int64, dst []float64)

//go:noescape
func sqrtI32(src []int32, dst []float32)

//go:noescape
func recipI64(src []int64, dst []float64)

//go:noescape
func recipI32(src []int32, dst []float32)
