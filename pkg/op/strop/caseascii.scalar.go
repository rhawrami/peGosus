package strop

func toUpperASCIIFallback(src []byte, dst []byte) {
	for i, value := range src {
		if value >= 'a' && value <= 'z' {
			value -= 'a' - 'A'
		}
		dst[i] = value
	}
}

func toLowerASCIIFallback(src []byte, dst []byte) {
	for i, value := range src {
		if value >= 'A' && value <= 'Z' {
			value += 'a' - 'A'
		}
		dst[i] = value
	}
}
