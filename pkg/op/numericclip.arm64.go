//go:build arm64

package op

// ClipF64WithF64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
//
//go:noescape
func ClipF64WithF64Bounds(src, dst []float64, lower, upper float64)

// ClipF32WithF32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
//
//go:noescape
func ClipF32WithF32Bounds(src, dst []float32, lower, upper float32)

// ClipI32WithI32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
//
//go:noescape
func ClipI32WithI32Bounds(src, dst []int32, lower, upper int32)

// ClipI64WithI64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
//
//go:noescape
func ClipI64WithI64Bounds(src, dst []int64, lower, upper int64)

// ClipI64WithF64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`. Elements are converted to float64.
//
//go:noescape
func ClipI64WithF64Bounds(src []int64, dst []float64, lower, upper float64)

// ClipI32WithF32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`. Elements are converted to float32.
//
//go:noescape
func ClipI32WithF32Bounds(src []int32, dst []float32, lower, upper float32)
