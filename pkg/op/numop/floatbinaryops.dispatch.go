//go:build !arm64

package numop

// AddF64Lit adds `lit` to elements in `src`, placing the result in `dst`.
func AddF64Lit(src, dst []float64, lit float64) { addF64LitImpl(src, dst, lit) }

// SubF64Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
func SubF64Lit(src, dst []float64, lit float64) { subF64LitImpl(src, dst, lit) }

// MulF64Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
func MulF64Lit(src, dst []float64, lit float64) { mulF64LitImpl(src, dst, lit) }

// DivF64Lit divides elements in `src` by `lit` , placing the result in `dst`.
func DivF64Lit(src, dst []float64, lit float64) { divF64LitImpl(src, dst, lit) }

// AddF32Lit adds `lit` to elements in `src`, placing the result in `dst`.
func AddF32Lit(src, dst []float32, lit float32) { addF32LitImpl(src, dst, lit) }

// SubF32Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
func SubF32Lit(src, dst []float32, lit float32) { subF32LitImpl(src, dst, lit) }

// MulF32Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
func MulF32Lit(src, dst []float32, lit float32) { mulF32LitImpl(src, dst, lit) }

// DivF32Lit divides elements in `src` by `lit` , placing the result in `dst`.
func DivF32Lit(src, dst []float32, lit float32) { divF32LitImpl(src, dst, lit) }

// AddF64Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
func AddF64Vec(src1, src2, dst []float64) { addF64VecImpl(src1, src2, dst) }

// SubF64Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
func SubF64Vec(src1, src2, dst []float64) { subF64VecImpl(src1, src2, dst) }

// MulF64Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
func MulF64Vec(src1, src2, dst []float64) { mulF64VecImpl(src1, src2, dst) }

// DivF64Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
func DivF64Vec(src1, src2, dst []float64) { divF64VecImpl(src1, src2, dst) }

// AddF32Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
func AddF32Vec(src1, src2, dst []float32) { addF32VecImpl(src1, src2, dst) }

// SubF32Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
func SubF32Vec(src1, src2, dst []float32) { subF32VecImpl(src1, src2, dst) }

// MulF32Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
func MulF32Vec(src1, src2, dst []float32) { mulF32VecImpl(src1, src2, dst) }

// DivF32Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
func DivF32Vec(src1, src2, dst []float32) { divF32VecImpl(src1, src2, dst) }
