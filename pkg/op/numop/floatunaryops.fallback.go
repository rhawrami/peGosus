package numop

import "math"

func sqrtF64Fallback(src, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = math.Sqrt(src[i])
	}
}

func sqF64Fallback(src, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * src[i]
	}
}

func absF64Fallback(src, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = math.Float64frombits(math.Float64bits(src[i]) &^ uint64(1<<63))
	}
}

func negF64Fallback(src, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = math.Float64frombits(math.Float64bits(src[i]) ^ uint64(1<<63))
	}
}

func recipF64Fallback(src, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = 1 / src[i]
	}
}

func sqrtF32Fallback(src, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = float32(math.Sqrt(float64(src[i])))
	}
}

func sqF32Fallback(src, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = src[i] * src[i]
	}
}

func absF32Fallback(src, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = math.Float32frombits(math.Float32bits(src[i]) &^ uint32(1<<31))
	}
}

func negF32Fallback(src, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = math.Float32frombits(math.Float32bits(src[i]) ^ uint32(1<<31))
	}
}

func recipF32Fallback(src, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = 1 / src[i]
	}
}
