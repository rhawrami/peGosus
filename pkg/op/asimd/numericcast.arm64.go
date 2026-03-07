//go:build arm64

package asimd

//go:noescape
func castI64ToF64(src []int64, dst []float64)

//go:noescape
func castI32ToF32(src []int32, dst []float32)

//go:noescape
func castF64ToI64(src []float64, dst []int64)

//go:noescape
func castF32ToI32(src []float32, dst []int32)

//go:noescape
func castF32ToF64(src []float32, dst []float64)

//go:noescape
func castI32ToI64(src []int32, dst []float64)

//go:noescape
func castI32ToF64(src []int32, dst []float64)

//go:noescape
func castF32ToI64(src []float32, dst []int64)

//go:noescape
func castI64ToF32(src []int64, dst []float32)

//go:noescape
func castF64ToI32(src []float64, dst []int32)
