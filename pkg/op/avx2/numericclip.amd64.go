//go:build amd64

package op

//go:noescape
func clipF64VecWithF64Bounds(src, dst []float64, lower, upper float64)

//go:noescape
func clipF32VecWithF32Bounds(src, dst []float32, lower, upper float32)

//go:noescape
func clipI32VecWithI32Bounds(src, dst []int32, lower, upper int32)

//go:noescape
func clipI64VecWithI64Bounds(src, dst []int64, lower, upper int64)

//go:noescape
func clipI64VecWithF64Bounds(src []int64, dst []float64, lower, upper float64)

//go:noescape
func clipI32VecWithF32Bounds(src []int32, dst []float32, lower, upper float32)
