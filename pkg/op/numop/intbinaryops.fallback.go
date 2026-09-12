package numop

func addI64LitFallback(src, dst []int64, lit int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] + lit
	}
}

func subI64LitFallback(src, dst []int64, lit int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] - lit
	}
}

func mulI64LitFallback(src, dst []int64, lit int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * lit
	}
}

func divI64LitFallback(src []int64, dst []float64, lit float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = float64(src[i]) / lit
	}
}

func addI32LitFallback(src, dst []int32, lit int32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] + lit
	}
}

func subI32LitFallback(src, dst []int32, lit int32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] - lit
	}
}

func mulI32LitFallback(src, dst []int32, lit int32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * lit
	}
}

func divI32LitFallback(src []int32, dst []float32, lit float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = float32(src[i]) / lit
	}
}

func addI64VecFallback(src1, src2, dst []int64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] + src2[i]
	}
}

func subI64VecFallback(src1, src2, dst []int64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] - src2[i]
	}
}

func mulI64VecFallback(src1, src2, dst []int64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] * src2[i]
	}
}

func divI64VecFallback(src1, src2 []int64, dst []float64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = float64(src1[i]) / float64(src2[i])
	}
}

func addI32VecFallback(src1, src2, dst []int32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] + src2[i]
	}
}

func subI32VecFallback(src1, src2, dst []int32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] - src2[i]
	}
}

func mulI32VecFallback(src1, src2, dst []int32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] * src2[i]
	}
}

func divI32VecFallback(src1, src2 []int32, dst []float32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = float32(src1[i]) / float32(src2[i])
	}
}
