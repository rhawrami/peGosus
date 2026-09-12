//go:build !arm64

package numop

// SqrtF64 takes the square root of elements in `src`, and places the result in `dst`.
func SqrtF64(src, dst []float64) { sqrtF64Impl(src, dst) }

// SqF64 takes the square of elements in `src`, and places the result in `dst`.
func SqF64(src, dst []float64) { sqF64Impl(src, dst) }

// AbsF64 takes the absolute value of elements in `src`, and places the result in `dst`.
func AbsF64(src, dst []float64) { absF64Impl(src, dst) }

// NegF64 negates elements in `src`, and places the result in `dst`.
func NegF64(src, dst []float64) { negF64Impl(src, dst) }

// RecipF64 takes the reciprocal elements in `src`, and places the result in `dst`.
func RecipF64(src, dst []float64) { recipF64Impl(src, dst) }

// SqrtF32 takes the square root of elements in `src`, and places the result in `dst`.
func SqrtF32(src, dst []float32) { sqrtF32Impl(src, dst) }

// SqF32 takes the square of elements in `src`, and places the result in `dst`.
func SqF32(src, dst []float32) { sqF32Impl(src, dst) }

// AbsF32 takes the absolute value of elements in `src`, and places the result in `dst`.
func AbsF32(src, dst []float32) { absF32Impl(src, dst) }

// NegF32 negates elements in `src`, and places the result in `dst`.
func NegF32(src, dst []float32) { negF32Impl(src, dst) }

// RecipF32 takes the reciprocal elements in `src`, and places the result in `dst`.
func RecipF32(src, dst []float32) { recipF32Impl(src, dst) }
