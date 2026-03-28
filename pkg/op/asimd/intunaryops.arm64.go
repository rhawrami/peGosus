//go:build arm64

package op

//go:noescape
func absI64(src, dst []int64)

//go:noescape
func absI32(src, dst []int32)

//go:noescape
func negI64(src, dst []int64)

//go:noescape
func negI32(src, dst []int32)

//go:noescape
func sqI32(src, dst []int32)

//go:noescape
func sqI64(src, dst []int64)

//go:noescape
func sqrtI64(src []int64, dst []float64)

//go:noescape
func sqrtI32(src []int32, dst []float32)

//go:noescape
func recipI64(src []int64, dst []float64)

//go:noescape
func recipI32(src []int32, dst []float32)
