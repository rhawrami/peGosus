//go:build !arm64

package numop

// AddI64Lit adds `lit` to elements in `src`, placing the result in `dst`.
func AddI64Lit(src, dst []int64, lit int64) { addI64LitImpl(src, dst, lit) }

// SubI64Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
func SubI64Lit(src, dst []int64, lit int64) { subI64LitImpl(src, dst, lit) }

// MulI64Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
func MulI64Lit(src, dst []int64, lit int64) { mulI64LitImpl(src, dst, lit) }

// DivI64Lit divides elements in `src` by `lit` , placing the result in `dst`.
func DivI64Lit(src []int64, dst []float64, lit float64) { divI64LitImpl(src, dst, lit) }

// AddI32Lit adds `lit` to elements in `src`, placing the result in `dst`.
func AddI32Lit(src, dst []int32, lit int32) { addI32LitImpl(src, dst, lit) }

// SubI32Lit subtracts `lit` from elements in `src`, placing the result in `dst`.
func SubI32Lit(src, dst []int32, lit int32) { subI32LitImpl(src, dst, lit) }

// MulI32Lit multiplies `lit` by elements in `src`, placing the result in `dst`.
func MulI32Lit(src, dst []int32, lit int32) { mulI32LitImpl(src, dst, lit) }

// DivI32Lit divides elements in `src` by `lit` , placing the result in `dst`.
func DivI32Lit(src []int32, dst []float32, lit float32) { divI32LitImpl(src, dst, lit) }

// AddI64Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
func AddI64Vec(src1, src2, dst []int64) { addI64VecImpl(src1, src2, dst) }

// SubI64Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
func SubI64Vec(src1, src2, dst []int64) { subI64VecImpl(src1, src2, dst) }

// MulI64Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
func MulI64Vec(src1, src2, dst []int64) { mulI64VecImpl(src1, src2, dst) }

// DivI64Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
func DivI64Vec(src1, src2 []int64, dst []float64) { divI64VecImpl(src1, src2, dst) }

// AddI32Vec adds elements in `src1` to elements in `src2`, placing the result in `dst`.
func AddI32Vec(src1, src2, dst []int32) { addI32VecImpl(src1, src2, dst) }

// SubI32Vec subtracts elements in `src2` from elements in `src1`, placing the result in `dst`.
func SubI32Vec(src1, src2, dst []int32) { subI32VecImpl(src1, src2, dst) }

// MulI32Vec multiplies elements in `src1` by elements in `src2`, placing the result in `dst`.
func MulI32Vec(src1, src2, dst []int32) { mulI32VecImpl(src1, src2, dst) }

// DivI32Vec divides elements in `src1` by elements in `src2`, placing the result in `dst`.
func DivI32Vec(src1, src2 []int32, dst []float32) { divI32VecImpl(src1, src2, dst) }
