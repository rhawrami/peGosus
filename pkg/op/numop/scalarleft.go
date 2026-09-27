package numop

// SubI64LitLeft subtracts elements in src from lit, placing the result in dst.
func SubI64LitLeft(src, dst []int64, lit int64) {
	for i := range src {
		dst[i] = lit - src[i]
	}
}

// DivI64LitLeft divides lit by elements in src, placing FLOAT64 results in dst.
func DivI64LitLeft(src []int64, dst []float64, lit float64) {
	for i := range src {
		dst[i] = lit / float64(src[i])
	}
}

// SubI32LitLeft subtracts elements in src from lit, placing the result in dst.
func SubI32LitLeft(src, dst []int32, lit int32) {
	for i := range src {
		dst[i] = lit - src[i]
	}
}

// DivI32LitLeft divides lit by elements in src, placing FLOAT32 results in dst.
func DivI32LitLeft(src []int32, dst []float32, lit float32) {
	for i := range src {
		dst[i] = lit / float32(src[i])
	}
}

// SubF64LitLeft subtracts elements in src from lit, placing the result in dst.
func SubF64LitLeft(src, dst []float64, lit float64) {
	for i := range src {
		dst[i] = lit - src[i]
	}
}

// DivF64LitLeft divides lit by elements in src, placing the result in dst.
func DivF64LitLeft(src, dst []float64, lit float64) {
	for i := range src {
		dst[i] = lit / src[i]
	}
}

// SubF32LitLeft subtracts elements in src from lit, placing the result in dst.
func SubF32LitLeft(src, dst []float32, lit float32) {
	for i := range src {
		dst[i] = lit - src[i]
	}
}

// DivF32LitLeft divides lit by elements in src, placing the result in dst.
func DivF32LitLeft(src, dst []float32, lit float32) {
	for i := range src {
		dst[i] = lit / src[i]
	}
}
