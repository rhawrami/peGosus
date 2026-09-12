package numop

func sumI64Fallback(src, dst []int64) {
	var sum int64
	for _, value := range src {
		sum += value
	}
	dst[0] = sum
}

func sumI32Fallback(src []int32, dst []int64) {
	var sum int64
	for _, value := range src {
		sum += int64(value)
	}
	dst[0] = sum
}

func sumF64Fallback(src, dst []float64) {
	var sum float64
	for _, value := range src {
		if value == value {
			sum += value
		}
	}
	dst[0] = sum
}

func sumF32Fallback(src []float32, dst []float64) {
	var sum float64
	for _, value := range src {
		if value == value {
			sum += float64(value)
		}
	}
	dst[0] = sum
}

func sumI64WithValidityFallback(src, dst []int64, validity []byte) {
	var sum int64
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 {
			sum += value
		}
	}
	dst[0] = sum
}

func sumI32WithValidityFallback(src []int32, dst []int64, validity []byte) {
	var sum int64
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 {
			sum += int64(value)
		}
	}
	dst[0] = sum
}

func sumF64WithValidityFallback(src, dst []float64, validity []byte) {
	var sum float64
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value == value {
			sum += value
		}
	}
	dst[0] = sum
}

func sumF32WithValidityFallback(src []float32, dst []float64, validity []byte) {
	var sum float64
	for i, value := range src {
		if validity[i/8]&(1<<uint(i%8)) != 0 && value == value {
			sum += float64(value)
		}
	}
	dst[0] = sum
}
