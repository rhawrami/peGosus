//go:build amd64

package op

//go:noescape
func addI64Lit(src, dst []int64, lit int64)

//go:noescape
func subI64Lit(src, dst []int64, lit int64)

func mulI64Lit(src, dst []int64, lit int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * lit
	}
}

//go:noescape
func divI64Lit(src []int64, dst []float64, lit float64)

//go:noescape
func addI32Lit(src, dst []int32, lit int32)

//go:noescape
func subI32Lit(src, dst []int32, lit int32)

//go:noescape
func mulI32Lit(src, dst []int32, lit int32)

//go:noescape
func divI32Lit(src []int32, dst []float32, lit float32)

//go:noescape
func addI64Vec(src1, src2, dst []int64)

//go:noescape
func subI64Vec(src1, src2, dst []int64)

func mulI64Vec(src1, src2, dst []int64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] * src2[i]
	}
}

//go:noescape
func divI64Vec(src1, src2 []int64, dst []float64)

//go:noescape
func addI32Vec(src1, src2, dst []int32)

//go:noescape
func subI32Vec(src1, src2, dst []int32)

//go:noescape
func mulI32Vec(src1, src2, dst []int32)

//go:noescape
func divI32Vec(src1, src2 []int32, dst []float32)
