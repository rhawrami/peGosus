package numop

import "math"

func sqrtI64Fallback(src []int64, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = math.Sqrt(float64(src[i]))
	}
}

func sqI64Fallback(src, dst []int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * src[i]
	}
}

func absI64Fallback(src, dst []int64) {
	for i := 0; i < len(src); i++ {
		if src[i] < 0 {
			dst[i] = -src[i]
		} else {
			dst[i] = src[i]
		}
	}
}

func negI64Fallback(src, dst []int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = -src[i]
	}
}

func recipI64Fallback(src []int64, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = 1 / float64(src[i])
	}
}

func sqrtI32Fallback(src []int32, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = float32(math.Sqrt(float64(src[i])))
	}
}

func sqI32Fallback(src, dst []int32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * src[i]
	}
}

func absI32Fallback(src, dst []int32) {
	for i := 0; i < len(src); i++ {
		if src[i] < 0 {
			dst[i] = -src[i]
		} else {
			dst[i] = src[i]
		}
	}
}

func negI32Fallback(src, dst []int32) {
	for i := 0; i < len(src); i++ {
		dst[i] = -src[i]
	}
}

func recipI32Fallback(src []int32, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = 1 / float32(src[i])
	}
}
