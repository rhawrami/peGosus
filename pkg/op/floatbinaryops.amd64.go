//go:build amd64

package op

//go:noescape
func addF64Lit(src, dst []float64, lit float64)

//go:noescape
func addF32Lit(src, dst []float32, lit float32)

//go:noescape
func subF64Lit(src, dst []float64, lit float64)

//go:noescape
func subF32Lit(src, dst []float32, lit float32)

//go:noescape
func mulF64Lit(src, dst []float64, lit float64)

//go:noescape
func mulF32Lit(src, dst []float32, lit float32)

//go:noescape
func divF64Lit(src, dst []float64, lit float64)

//go:noescape
func divF32Lit(src, dst []float32, lit float32)

//go:noescape
func addF64VecF64Vec(src1, src2, dst []float64)

//go:noescape
func addF32VecF32Vec(src1, src2, dst []float32)

//go:noescape
func subF64VecF64Vec(src1, src2, dst []float64)

//go:noescape
func subF32VecF32Vec(src1, src2, dst []float32)

//go:noescape
func mulF64VecF64Vec(src1, src2, dst []float64)

//go:noescape
func mulF32VecF32Vec(src1, src2, dst []float32)

//go:noescape
func divF64VecF64Vec(src1, src2, dst []float64)

//go:noescape
func divF32VecF32Vec(src1, src2, dst []float32)
