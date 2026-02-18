//go:build arm64

package asimd

//go:noescape
func absI64Vec(src, dst []int64)

//go:noescape
func absI32Vec(src, dst []int32)

//go:noescape
func negI64Vec(src, dst []int64)

//go:noescape
func negI32Vec(src, dst []int32)

//go:noescape
func sqI32Vec(src, dst []int32)

//go:noescape
func sqI64Vec(src, dst []int64)

//go:noescape
func sqrtI64Vec(src []int64, dst []float64)

//go:noescape
func sqrtI32Vec(src []int32, dst []float32)

//go:noescape
func recipI64Vec(src []int64, dst []float64)

//go:noescape
func recipI32Vec(src []int32, dst []float32)
