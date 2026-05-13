//go:build arm64

package op

// CastI64ToF64 casts int64 elements in `src` to float64,
// placing the result in `dst`.
//
//go:noescape
func CastI64ToF64(src []int64, dst []float64)

// CastI32ToF32 casts int32 elements in `src` to float32,
// placing the result in `dst`.
//
//go:noescape
func CastI32ToF32(src []int32, dst []float32)

// CastF64ToI64 casts float64 elements in `src` to int64,
// placing the result in `dst`.
//
//go:noescape
func CastF64ToI64(src []float64, dst []int64)

// CastF32ToI32 casts float32 elements in `src` to int32,
// placing the result in `dst`.
//
//go:noescape
func CastF32ToI32(src []float32, dst []int32)

// CastF32ToF64 casts float32 elements in `src` to float64,
// placing the result in `dst`.
//
//go:noescape
func CastF32ToF64(src []float32, dst []float64)

// CastI32ToI64 casts int32 elements in `src` to int64,
// placing the result in `dst`.
//
//go:noescape
func CastI32ToI64(src []int32, dst []int64)

// CastI32ToF64 casts int32 elements in `src` to float64,
// placing the result in `dst`.
//
//go:noescape
func CastI32ToF64(src []int32, dst []float64)

// CastF32ToI64 casts float32 elements in `src` to int64,
// placing the result in `dst`.
//
//go:noescape
func CastF32ToI64(src []float32, dst []int64)

// CastI64ToF32 casts int64 elements in `src` to float32,
// placing the result in `dst`.
//
//go:noescape
func CastI64ToF32(src []int64, dst []float32)

// CastF64ToI32 casts float64 elements in `src` to int32,
// placing the result in `dst`.
//
//go:noescape
func CastF64ToI32(src []float64, dst []int32)
