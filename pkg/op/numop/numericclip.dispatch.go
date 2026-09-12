//go:build !arm64

package numop

// ClipF64WithF64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
func ClipF64WithF64Bounds(src, dst []float64, lower, upper float64) {
	clipF64WithF64BoundsImpl(src, dst, lower, upper)
}

// ClipF32WithF32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
func ClipF32WithF32Bounds(src, dst []float32, lower, upper float32) {
	clipF32WithF32BoundsImpl(src, dst, lower, upper)
}

// ClipI32WithI32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
func ClipI32WithI32Bounds(src, dst []int32, lower, upper int32) {
	clipI32WithI32BoundsImpl(src, dst, lower, upper)
}

// ClipI64WithI64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
func ClipI64WithI64Bounds(src, dst []int64, lower, upper int64) {
	clipI64WithI64BoundsImpl(src, dst, lower, upper)
}

// ClipI64WithF64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`. Elements are converted to float64.
func ClipI64WithF64Bounds(src []int64, dst []float64, lower, upper float64) {
	clipI64WithF64BoundsImpl(src, dst, lower, upper)
}

// ClipI32WithF32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`. Elements are converted to float32.
func ClipI32WithF32Bounds(src []int32, dst []float32, lower, upper float32) {
	clipI32WithF32BoundsImpl(src, dst, lower, upper)
}
