//go:build arm64

package op

//go:noescape
func clipF64WithF64Bounds(src, dst []float64, lower, upper float64)

//go:noescape
func clipF32WithF32Bounds(src, dst []float32, lower, upper float32)

//go:noescape
func clipI32WithI32Bounds(src, dst []int32, lower, upper int32)

//go:noescape
func clipI64WithI64Bounds(src, dst []int64, lower, upper int64)

//go:noescape
func clipI64WithF64Bounds(src []int64, dst []float64, lower, upper float64)

//go:noescape
func clipI32WithF32Bounds(src []int32, dst []float32, lower, upper float32)
