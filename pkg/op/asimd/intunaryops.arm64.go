//go:build arm64

package op

// SqrtI64 takes the square root of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqrtI64(src []int64, dst []float64)

// SqI64 takes the square of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqI64(src, dst []int64)

// AbsI64 takes the absolute value of elements in `src`, and places the result in `dst`.
//
//go:noescape
func AbsI64(src, dst []int64)

// NegI64 negates elements in `src`, and places the result in `dst`.
//
//go:noescape
func NegI64(src, dst []int64)

// RecipI64 takes the reciprocal elements in `src`, and places the result in `dst`.
//
//go:noescape
func RecipI64(src []int64, dst []float64)

// SqrtI32 takes the square root of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqrtI32(src []int32, dst []float32)

// SqI32 takes the square of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqI32(src, dst []int32)

// AbsI32 takes the absolute value of elements in `src`, and places the result in `dst`.
//
//go:noescape
func AbsI32(src, dst []int32)

// NegI32 negates elements in `src`, and places the result in `dst`.
//
//go:noescape
func NegI32(src, dst []int32)

// RecipI32 takes the reciprocal elements in `src`, and places the result in `dst`.
//
//go:noescape
func RecipI32(src []int32, dst []float32)
