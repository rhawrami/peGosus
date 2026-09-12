package numop

import "math"

func maxI64Fallback(src, dst []int64) {
	result := int64(math.MinInt64)
	for _, value := range src {
		if value > result {
			result = value
		}
	}
	dst[0] = result
}

func minI64Fallback(src, dst []int64) {
	result := int64(math.MaxInt64)
	for _, value := range src {
		if value < result {
			result = value
		}
	}
	dst[0] = result
}

func maxI32Fallback(src, dst []int32) {
	result := int32(math.MinInt32)
	for _, value := range src {
		if value > result {
			result = value
		}
	}
	dst[0] = result
}

func minI32Fallback(src, dst []int32) {
	result := int32(math.MaxInt32)
	for _, value := range src {
		if value < result {
			result = value
		}
	}
	dst[0] = result
}

func maxF64Fallback(src, dst []float64) {
	result := math.Inf(-1)
	found := false
	for _, value := range src {
		if value == value && (!found || value > result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func minF64Fallback(src, dst []float64) {
	result := math.Inf(1)
	found := false
	for _, value := range src {
		if value == value && (!found || value < result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func maxF32Fallback(src, dst []float32) {
	result := float32(math.Inf(-1))
	found := false
	for _, value := range src {
		if value == value && (!found || value > result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func minF32Fallback(src, dst []float32) {
	result := float32(math.Inf(1))
	found := false
	for _, value := range src {
		if value == value && (!found || value < result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func minMaxI64Fallback(src, dst []int64) {
	minimum, maximum := int64(math.MaxInt64), int64(math.MinInt64)
	for _, value := range src {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	dst[0], dst[1] = minimum, maximum
}

func minMaxI32Fallback(src, dst []int32) {
	minimum, maximum := int32(math.MaxInt32), int32(math.MinInt32)
	for _, value := range src {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	dst[0], dst[1] = minimum, maximum
}

func minMaxF64Fallback(src, dst []float64) {
	minimum, maximum := math.Inf(1), math.Inf(-1)
	found := false
	for _, value := range src {
		if value != value {
			continue
		}
		if !found || value < minimum {
			minimum = value
		}
		if !found || value > maximum {
			maximum = value
		}
		found = true
	}
	dst[0], dst[1] = minimum, maximum
}

func minMaxF32Fallback(src, dst []float32) {
	minimum, maximum := float32(math.Inf(1)), float32(math.Inf(-1))
	found := false
	for _, value := range src {
		if value != value {
			continue
		}
		if !found || value < minimum {
			minimum = value
		}
		if !found || value > maximum {
			maximum = value
		}
		found = true
	}
	dst[0], dst[1] = minimum, maximum
}

func maxI64WithValidityFallback(src, dst []int64, validity []byte) {
	result := int64(math.MinInt64)
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value > result {
			result = value
		}
	}
	dst[0] = result
}

func minI64WithValidityFallback(src, dst []int64, validity []byte) {
	result := int64(math.MaxInt64)
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value < result {
			result = value
		}
	}
	dst[0] = result
}

func maxI32WithValidityFallback(src, dst []int32, validity []byte) {
	result := int32(math.MinInt32)
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value > result {
			result = value
		}
	}
	dst[0] = result
}

func minI32WithValidityFallback(src, dst []int32, validity []byte) {
	result := int32(math.MaxInt32)
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value < result {
			result = value
		}
	}
	dst[0] = result
}

func maxF64WithValidityFallback(src, dst []float64, validity []byte) {
	result := math.Inf(-1)
	found := false
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value == value && (!found || value > result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func minF64WithValidityFallback(src, dst []float64, validity []byte) {
	result := math.Inf(1)
	found := false
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value == value && (!found || value < result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func maxF32WithValidityFallback(src, dst []float32, validity []byte) {
	result := float32(math.Inf(-1))
	found := false
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value == value && (!found || value > result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func minF32WithValidityFallback(src, dst []float32, validity []byte) {
	result := float32(math.Inf(1))
	found := false
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value == value && (!found || value < result) {
			result = value
			found = true
		}
	}
	dst[0] = result
}

func minMaxI64WithValidityFallback(src, dst []int64, validity []byte) {
	minimum, maximum := int64(math.MaxInt64), int64(math.MinInt64)
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) == 0 {
			continue
		}
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	dst[0], dst[1] = minimum, maximum
}

func minMaxI32WithValidityFallback(src, dst []int32, validity []byte) {
	minimum, maximum := int32(math.MaxInt32), int32(math.MinInt32)
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) == 0 {
			continue
		}
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	dst[0], dst[1] = minimum, maximum
}

func minMaxF64WithValidityFallback(src, dst []float64, validity []byte) {
	minimum, maximum := math.Inf(1), math.Inf(-1)
	found := false
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) == 0 || value != value {
			continue
		}
		if !found || value < minimum {
			minimum = value
		}
		if !found || value > maximum {
			maximum = value
		}
		found = true
	}
	dst[0], dst[1] = minimum, maximum
}

func minMaxF32WithValidityFallback(src, dst []float32, validity []byte) {
	minimum, maximum := float32(math.Inf(1)), float32(math.Inf(-1))
	found := false
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) == 0 || value != value {
			continue
		}
		if !found || value < minimum {
			minimum = value
		}
		if !found || value > maximum {
			maximum = value
		}
		found = true
	}
	dst[0], dst[1] = minimum, maximum
}
