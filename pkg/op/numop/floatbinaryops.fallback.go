package numop

func addF64LitFallback(src, dst []float64, lit float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] + lit
	}
}

func subF64LitFallback(src, dst []float64, lit float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] - lit
	}
}

func mulF64LitFallback(src, dst []float64, lit float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * lit
	}
}

func divF64LitFallback(src, dst []float64, lit float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] / lit
	}
}

func addF32LitFallback(src, dst []float32, lit float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] + lit
	}
}

func subF32LitFallback(src, dst []float32, lit float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] - lit
	}
}

func mulF32LitFallback(src, dst []float32, lit float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * lit
	}
}

func divF32LitFallback(src, dst []float32, lit float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] / lit
	}
}

func addF64VecFallback(src1, src2, dst []float64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] + src2[i]
	}
}

func subF64VecFallback(src1, src2, dst []float64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] - src2[i]
	}
}

func mulF64VecFallback(src1, src2, dst []float64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] * src2[i]
	}
}

func divF64VecFallback(src1, src2, dst []float64) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] / src2[i]
	}
}

func addF32VecFallback(src1, src2, dst []float32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] + src2[i]
	}
}

func subF32VecFallback(src1, src2, dst []float32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] - src2[i]
	}
}

func mulF32VecFallback(src1, src2, dst []float32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] * src2[i]
	}
}

func divF32VecFallback(src1, src2, dst []float32) {
	for i := 0; i < len(src1); i++ {
		dst[i] = src1[i] / src2[i]
	}
}
