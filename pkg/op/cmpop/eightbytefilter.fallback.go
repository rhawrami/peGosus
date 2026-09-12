package cmpop

func gtI64Fallback(src []int64, dst []byte, lit int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value > lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func ltI64Fallback(src []int64, dst []byte, lit int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value < lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func geI64Fallback(src []int64, dst []byte, lit int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func leI64Fallback(src []int64, dst []byte, lit int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value <= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func eqI64Fallback(src []int64, dst []byte, lit int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value == lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func gtF64Fallback(src []float64, dst []byte, lit float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value > lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func ltF64Fallback(src []float64, dst []byte, lit float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value < lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func geF64Fallback(src []float64, dst []byte, lit float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func leF64Fallback(src []float64, dst []byte, lit float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value <= lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func eqF64Fallback(src []float64, dst []byte, lit float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value == lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func neqI64Fallback(src []int64, dst []byte, lit int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value != lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func neqF64Fallback(src []float64, dst []byte, lit float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value != lit {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func betI64Fallback(src []int64, dst []byte, min int64, max int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= min && value <= max {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func nBetI64Fallback(src []int64, dst []byte, min int64, max int64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if !(value >= min && value <= max) {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func betF64Fallback(src []float64, dst []byte, min float64, max float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value >= min && value <= max {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}

func nBetF64Fallback(src []float64, dst []byte, min float64, max float64) {
	clear(dst[:(len(src)+7)/8])
	for i, value := range src {
		if value < min || value > max {
			dst[i/8] |= 1 << (i % 8)
		}
	}
}
