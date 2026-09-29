//go:build amd64

package numop

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

func clipF64WithF64BoundsChecked(src, dst []float64, lower, upper float64) {
	if clipF64NeedsScalar(src, lower, upper) {
		clipF64WithF64BoundsFallback(src, dst, lower, upper)
		return
	}
	clipF64WithF64Bounds(src, dst, lower, upper)
}

func clipF32WithF32BoundsChecked(src, dst []float32, lower, upper float32) {
	if clipF32NeedsScalar(src, lower, upper) {
		clipF32WithF32BoundsFallback(src, dst, lower, upper)
		return
	}
	clipF32WithF32Bounds(src, dst, lower, upper)
}

func clipI64WithF64BoundsChecked(src []int64, dst []float64, lower, upper float64) {
	if lower == 0 || upper == 0 || lower != lower || upper != upper || lower > upper {
		clipI64WithF64BoundsFallback(src, dst, lower, upper)
		return
	}
	clipI64WithF64Bounds(src, dst, lower, upper)
}

func clipI32WithF32BoundsChecked(src []int32, dst []float32, lower, upper float32) {
	if lower == 0 || upper == 0 || lower != lower || upper != upper || lower > upper {
		clipI32WithF32BoundsFallback(src, dst, lower, upper)
		return
	}
	clipI32WithF32Bounds(src, dst, lower, upper)
}
