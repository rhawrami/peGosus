package cmpop

func gtI32Fallback(src []int32, dst []byte, lit int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value > lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func ltI32Fallback(src []int32, dst []byte, lit int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value < lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func geI32Fallback(src []int32, dst []byte, lit int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func leI32Fallback(src []int32, dst []byte, lit int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value <= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func eqI32Fallback(src []int32, dst []byte, lit int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value == lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func gtF32Fallback(src []float32, dst []byte, lit float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value > lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func ltF32Fallback(src []float32, dst []byte, lit float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value < lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func geF32Fallback(src []float32, dst []byte, lit float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func leF32Fallback(src []float32, dst []byte, lit float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value <= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func eqF32Fallback(src []float32, dst []byte, lit float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value == lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func neqI32Fallback(src []int32, dst []byte, lit int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value != lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func neqF32Fallback(src []float32, dst []byte, lit float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value != lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func betI32Fallback(src []int32, dst []byte, min int32, max int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= min && value <= max {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func nBetI32Fallback(src []int32, dst []byte, min int32, max int32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if !(value >= min && value <= max) {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func betF32Fallback(src []float32, dst []byte, min float32, max float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= min && value <= max {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func nBetF32Fallback(src []float32, dst []byte, min float32, max float32) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value < min || value > max {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}
