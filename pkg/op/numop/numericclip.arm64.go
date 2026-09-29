//go:build arm64

package numop

// ClipF64WithF64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
func ClipF64WithF64Bounds(src, dst []float64, lower, upper float64) {
	if clipF64NeedsScalar(src, lower, upper) {
		clipF64WithF64BoundsFallback(src, dst, lower, upper)
		return
	}
	clipF64WithF64BoundsArm64(src, dst, lower, upper)
}

// ClipF32WithF32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
func ClipF32WithF32Bounds(src, dst []float32, lower, upper float32) {
	if clipF32NeedsScalar(src, lower, upper) {
		clipF32WithF32BoundsFallback(src, dst, lower, upper)
		return
	}
	clipF32WithF32BoundsArm64(src, dst, lower, upper)
}

// ClipI32WithI32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
//
//go:noescape
func ClipI32WithI32Bounds(src, dst []int32, lower, upper int32)

// ClipI64WithI64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`.
func ClipI64WithI64Bounds(src, dst []int64, lower, upper int64) {
	const exactFloatLimit int64 = 1 << 53
	if lower < -exactFloatLimit || lower > exactFloatLimit || upper < -exactFloatLimit || upper > exactFloatLimit {
		clipI64WithI64BoundsFallback(src, dst, lower, upper)
		return
	}
	for _, value := range src {
		if value < -exactFloatLimit || value > exactFloatLimit {
			clipI64WithI64BoundsFallback(src, dst, lower, upper)
			return
		}
	}
	clipI64WithI64BoundsArm64(src, dst, lower, upper)
}

// ClipI64WithF64Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`. Elements are converted to float64.
func ClipI64WithF64Bounds(src []int64, dst []float64, lower, upper float64) {
	if lower == 0 || upper == 0 || lower != lower || upper != upper || lower > upper {
		clipI64WithF64BoundsFallback(src, dst, lower, upper)
		return
	}
	clipI64WithF64BoundsArm64(src, dst, lower, upper)
}

// ClipI32WithF32Bounds clips elements in `src` between (inclusive) `lower` and
// `upper`, placing the result in `dst`. Elements are converted to float32.
func ClipI32WithF32Bounds(src []int32, dst []float32, lower, upper float32) {
	if lower == 0 || upper == 0 || lower != lower || upper != upper || lower > upper {
		clipI32WithF32BoundsFallback(src, dst, lower, upper)
		return
	}
	clipI32WithF32BoundsArm64(src, dst, lower, upper)
}

//go:noescape
func clipF64WithF64BoundsArm64(src, dst []float64, lower, upper float64)

//go:noescape
func clipF32WithF32BoundsArm64(src, dst []float32, lower, upper float32)

//go:noescape
func clipI64WithI64BoundsArm64(src, dst []int64, lower, upper int64)

//go:noescape
func clipI64WithF64BoundsArm64(src []int64, dst []float64, lower, upper float64)

//go:noescape
func clipI32WithF32BoundsArm64(src []int32, dst []float32, lower, upper float32)
