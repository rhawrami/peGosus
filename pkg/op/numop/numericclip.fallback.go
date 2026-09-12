package numop

func clipF64WithF64BoundsFallback(src, dst []float64, lower, upper float64) {
	for i, value := range src {
		if value < lower {
			value = lower
		}
		if value > upper {
			value = upper
		}
		dst[i] = value
	}
}

func clipF32WithF32BoundsFallback(src, dst []float32, lower, upper float32) {
	for i, value := range src {
		if value < lower {
			value = lower
		}
		if value > upper {
			value = upper
		}
		dst[i] = value
	}
}

func clipI32WithI32BoundsFallback(src, dst []int32, lower, upper int32) {
	for i, value := range src {
		if value < lower {
			value = lower
		}
		if value > upper {
			value = upper
		}
		dst[i] = value
	}
}

func clipI64WithI64BoundsFallback(src, dst []int64, lower, upper int64) {
	for i, value := range src {
		if value < lower {
			value = lower
		}
		if value > upper {
			value = upper
		}
		dst[i] = value
	}
}

func clipI64WithF64BoundsFallback(src []int64, dst []float64, lower, upper float64) {
	for i, integer := range src {
		value := float64(integer)
		if value < lower {
			value = lower
		}
		if value > upper {
			value = upper
		}
		dst[i] = value
	}
}

func clipI32WithF32BoundsFallback(src []int32, dst []float32, lower, upper float32) {
	for i, integer := range src {
		value := float32(integer)
		if value < lower {
			value = lower
		}
		if value > upper {
			value = upper
		}
		dst[i] = value
	}
}
