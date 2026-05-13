//go:build arm64

package op

// SqrtF64 takes the square root of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqrtF64(src, dst []float64)

// SqF64 takes the square of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqF64(src, dst []float64)

// AbsF64 takes the absolute value of elements in `src`, and places the result in `dst`.
//
//go:noescape
func AbsF64(src, dst []float64)

// NegF64 negates elements in `src`, and places the result in `dst`.
//
//go:noescape
func NegF64(src, dst []float64)

// RecipF64 takes the reciprocal elements in `src`, and places the result in `dst`.
//
//go:noescape
func RecipF64(src, dst []float64)

// SqrtF32 takes the square root of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqrtF32(src, dst []float32)

// SqF32 takes the square of elements in `src`, and places the result in `dst`.
//
//go:noescape
func SqF32(src, dst []float32)

// AbsF32 takes the absolute value of elements in `src`, and places the result in `dst`.
//
//go:noescape
func AbsF32(src, dst []float32)

// NegF32 negates elements in `src`, and places the result in `dst`.
//
//go:noescape
func NegF32(src, dst []float32)

// RecipF32 takes the reciprocal elements in `src`, and places the result in `dst`.
//
//go:noescape
func RecipF32(src, dst []float32)
