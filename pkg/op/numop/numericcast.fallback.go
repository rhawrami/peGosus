package numop

func castI64ToF64Fallback(src []int64, dst []float64) {
	for i, value := range src {
		dst[i] = float64(value)
	}
}

func castI32ToF32Fallback(src []int32, dst []float32) {
	for i, value := range src {
		dst[i] = float32(value)
	}
}

func castF64ToI64Fallback(src []float64, dst []int64) {
	for i, value := range src {
		dst[i] = int64(value)
	}
}

func castF32ToI32Fallback(src []float32, dst []int32) {
	for i, value := range src {
		dst[i] = int32(value)
	}
}

func castF32ToF64Fallback(src []float32, dst []float64) {
	for i, value := range src {
		dst[i] = float64(value)
	}
}

func castI32ToI64Fallback(src []int32, dst []int64) {
	for i, value := range src {
		dst[i] = int64(value)
	}
}

func castI32ToF64Fallback(src []int32, dst []float64) {
	for i, value := range src {
		dst[i] = float64(value)
	}
}

func castF32ToI64Fallback(src []float32, dst []int64) {
	for i, value := range src {
		dst[i] = int64(value)
	}
}

func castI64ToF32Fallback(src []int64, dst []float32) {
	for i, value := range src {
		dst[i] = float32(value)
	}
}

func castF64ToI32Fallback(src []float64, dst []int32) {
	for i, value := range src {
		dst[i] = int32(value)
	}
}
