//go:build arm64

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
func addF64Vec(src1, src2, dst []float64)

//go:noescape
func addF32Vec(src1, src2, dst []float32)

//go:noescape
func subF64Vec(src1, src2, dst []float64)

//go:noescape
func subF32Vec(src1, src2, dst []float32)

//go:noescape
func mulF64Vec(src1, src2, dst []float64)

//go:noescape
func mulF32Vec(src1, src2, dst []float32)

//go:noescape
func divF64Vec(src1, src2, dst []float64)

//go:noescape
func divF32Vec(src1, src2, dst []float32)
