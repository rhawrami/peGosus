//go:build arm64

package asimd

//go:noescape
func addI64VecI64Lit(src, dst []int64, lit int64)

//go:noescape
func addI32VecI32Lit(src, dst []int32, lit int32)

//go:noescape
func subI64VecI64Lit(src, dst []int64, lit int64)

//go:noescape
func subI32VecI32Lit(src, dst []int32, lit int32)

//go:noescape
func mulI32VecI32Lit(src, dst []int32, lit int32)

//go:noescape
func mulI64VecI64Lit(src, dst []int64, lit int64)

//go:noescape
func addI64VecI64Vec(src1, src2, dst []float64)

//go:noescape
func addI32VecI32Vec(src1, src2, dst []float32)

//go:noescape
func subI64VecI64Vec(src1, src2, dst []float64)

//go:noescape
func subI32VecI32Vec(src1, src2, dst []float32)

//go:noescape
func mulI32VecI32Vec(src1, src2, dst []float32)

//go:noescape
func mulI64VecI64Vec(src1, src2, dst []int64)

//go:noescape
func divI64VecI64Lit(src []int64, dst []float64, lit float64)

//go:noescape
func divI32VecI32Lit(src []int32, dst []float32, lit float32)

//go:noescape
func divI64VecI64Vec(src1, src2 []int64, dst []float64)

//go:noescape
func divI32VecI32Vec(src1, src2 []int32, dst []float32)
