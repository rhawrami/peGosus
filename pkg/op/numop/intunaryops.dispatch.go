//go:build !arm64

package numop

// SqrtI64 takes the square root of elements in `src`, and places the result in `dst`.
func SqrtI64(src []int64, dst []float64) { sqrtI64Impl(src, dst) }

// SqI64 takes the square of elements in `src`, and places the result in `dst`.
func SqI64(src, dst []int64) { sqI64Impl(src, dst) }

// AbsI64 takes the absolute value of elements in `src`, and places the result in `dst`.
func AbsI64(src, dst []int64) { absI64Impl(src, dst) }

// NegI64 negates elements in `src`, and places the result in `dst`.
func NegI64(src, dst []int64) { negI64Impl(src, dst) }

// RecipI64 takes the reciprocal elements in `src`, and places the result in `dst`.
func RecipI64(src []int64, dst []float64) { recipI64Impl(src, dst) }

// SqrtI32 takes the square root of elements in `src`, and places the result in `dst`.
func SqrtI32(src []int32, dst []float32) { sqrtI32Impl(src, dst) }

// SqI32 takes the square of elements in `src`, and places the result in `dst`.
func SqI32(src, dst []int32) { sqI32Impl(src, dst) }

// AbsI32 takes the absolute value of elements in `src`, and places the result in `dst`.
func AbsI32(src, dst []int32) { absI32Impl(src, dst) }

// NegI32 negates elements in `src`, and places the result in `dst`.
func NegI32(src, dst []int32) { negI32Impl(src, dst) }

// RecipI32 takes the reciprocal elements in `src`, and places the result in `dst`.
func RecipI32(src []int32, dst []float32) { recipI32Impl(src, dst) }
