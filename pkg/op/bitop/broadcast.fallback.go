package bitop

func broadcastU8Fallback(dst []byte, lit byte) {
	for i := range dst {
		dst[i] = lit
	}
}

func broadcastI64Fallback(dst []int64, lit int64) {
	for i := range dst {
		dst[i] = lit
	}
}

func broadcastI32Fallback(dst []int32, lit int32) {
	for i := range dst {
		dst[i] = lit
	}
}

func broadcastF64Fallback(dst []float64, lit float64) {
	for i := range dst {
		dst[i] = lit
	}
}

func broadcastF32Fallback(dst []float32, lit float32) {
	for i := range dst {
		dst[i] = lit
	}
}
