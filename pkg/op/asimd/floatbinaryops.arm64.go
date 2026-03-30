//go:build arm64

package op

// AddF64Lit adds `lit` to elements in `src`, placing the result in `dst`.
//
//go:noescape
func AddF64Lit(src, dst []float64, lit float64)

// SubF64Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
//
//go:noescape
func SubF64Lit(src, dst []float64, lit float64)

// MulF64Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
//
//go:noescape
func MulF64Lit(src, dst []float64, lit float64)

// DivF64Lit divides elements in `src` by `lit` , placing the result in `dst`.
//
//go:noescape
func DivF64Lit(src, dst []float64, lit float64)

// AddF32Lit adds `lit` to elements in `src`, placing the result in `dst`.
//
//go:noescape
func AddF32Lit(src, dst []float32, lit float32)

// SubF32Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
//
//go:noescape
func SubF32Lit(src, dst []float32, lit float32)

// MulF32Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
//
//go:noescape
func MulF32Lit(src, dst []float32, lit float32)

// DivF32Lit divides elements in `src` by `lit` , placing the result in `dst`.
//
//go:noescape
func DivF32Lit(src, dst []float32, lit float32)

// AddF64Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
//
//go:noescape
func AddF64Vec(src1, src2, dst []float64)

// SubF64Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
//
//go:noescape
func SubF64Vec(src1, src2, dst []float64)

// MulF64Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func MulF64Vec(src1, src2, dst []float64)

// DivF64Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func DivF64Vec(src1, src2, dst []float64)

// AddF32Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
//
//go:noescape
func AddF32Vec(src1, src2, dst []float32)

// SubF32Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
//
//go:noescape
func SubF32Vec(src1, src2, dst []float32)

// MulF32Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func MulF32Vec(src1, src2, dst []float32)

// DivF32Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func DivF32Vec(src1, src2, dst []float32)
