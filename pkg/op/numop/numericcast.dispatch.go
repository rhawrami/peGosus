//go:build !arm64

package numop

// CastI64ToF64 casts int64 elements in `src` to float64,
// placing the result in `dst`.
func CastI64ToF64(src []int64, dst []float64) { castI64ToF64Impl(src, dst) }

// CastI32ToF32 casts int32 elements in `src` to float32,
// placing the result in `dst`.
func CastI32ToF32(src []int32, dst []float32) { castI32ToF32Impl(src, dst) }

// CastF64ToI64 casts float64 elements in `src` to int64,
// placing the result in `dst`.
func CastF64ToI64(src []float64, dst []int64) { castF64ToI64Impl(src, dst) }

// CastF32ToI32 casts float32 elements in `src` to int32,
// placing the result in `dst`.
func CastF32ToI32(src []float32, dst []int32) { castF32ToI32Impl(src, dst) }

// CastF32ToF64 casts float32 elements in `src` to float64,
// placing the result in `dst`.
func CastF32ToF64(src []float32, dst []float64) { castF32ToF64Impl(src, dst) }

// CastI32ToI64 casts int32 elements in `src` to int64,
// placing the result in `dst`.
func CastI32ToI64(src []int32, dst []int64) { castI32ToI64Impl(src, dst) }

// CastI32ToF64 casts int32 elements in `src` to float64,
// placing the result in `dst`.
func CastI32ToF64(src []int32, dst []float64) { castI32ToF64Impl(src, dst) }

// CastF32ToI64 casts float32 elements in `src` to int64,
// placing the result in `dst`.
func CastF32ToI64(src []float32, dst []int64) { castF32ToI64Impl(src, dst) }

// CastI64ToF32 casts int64 elements in `src` to float32,
// placing the result in `dst`.
func CastI64ToF32(src []int64, dst []float32) { castI64ToF32Impl(src, dst) }

// CastF64ToI32 casts float64 elements in `src` to int32,
// placing the result in `dst`.
func CastF64ToI32(src []float64, dst []int32) { castF64ToI32Impl(src, dst) }
