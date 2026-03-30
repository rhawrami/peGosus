//go:build arm64

package op

// AddI64Lit adds `lit` to elements in `src`, placing the result in `dst`.
//
//go:noescape
func AddI64Lit(src, dst []int64, lit int64)

// SubI64Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
//
//go:noescape
func SubI64Lit(src, dst []int64, lit int64)

// MulI64Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
//
//go:noescape
func MulI64Lit(src, dst []int64, lit int64)

// DivI64Lit divides elements in `src` by `lit` , placing the result in `dst`.
//
//go:noescape
func DivI64Lit(src []int64, dst []float64, lit float64)

// AddI32Lit adds `lit` to elements in `src`, placing the result in `dst`.
//
//go:noescape
func AddI32Lit(src, dst []int32, lit int32)

// SubI32Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
//
//go:noescape
func SubI32Lit(src, dst []int32, lit int32)

// MulI32Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
//
//go:noescape
func MulI32Lit(src, dst []int32, lit int32)

// DivI32Lit divides elements in `src` by `lit` , placing the result in `dst`.
//
//go:noescape
func DivI32Lit(src []int32, dst []float32, lit float32)

// AddI64Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
//
//go:noescape
func AddI64Vec(src1, src2, dst []int64)

// SubI64Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
//
//go:noescape
func SubI64Vec(src1, src2, dst []int64)

// MulI64Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func MulI64Vec(src1, src2, dst []int64)

// DivI64Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func DivI64Vec(src1, src2 []int64, dst []float64)

// AddI32Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
//
//go:noescape
func AddI32Vec(src1, src2, dst []int32)

// SubI32Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
//
//go:noescape
func SubI32Vec(src1, src2, dst []int32)

// MulI32Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func MulI32Vec(src1, src2, dst []int32)

// DivI32Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
//
//go:noescape
func DivI32Vec(src1, src2 []int32, dst []float32)
