package bitop

import "math/bits"

func bitWiseAndWithPopCountFallback(src1, src2, dst []byte) uint64 {
	var count uint64
	for i := range src1 {
		dst[i] = src1[i] & src2[i]
		count += uint64(bits.OnesCount8(dst[i]))
	}
	return count
}

func bitWiseOrWithPopCountFallback(src1, src2, dst []byte) uint64 {
	var count uint64
	for i := range src1 {
		dst[i] = src1[i] | src2[i]
		count += uint64(bits.OnesCount8(dst[i]))
	}
	return count
}

func bitWiseAndNWithPopCountFallback(src1, src2, dst []byte) uint64 {
	var count uint64
	for i := range src1 {
		dst[i] = src1[i] &^ src2[i]
		count += uint64(bits.OnesCount8(dst[i]))
	}
	return count
}

func bitWiseXorWithPopCountFallback(src1, src2, dst []byte) uint64 {
	var count uint64
	for i := range src1 {
		dst[i] = src1[i] ^ src2[i]
		count += uint64(bits.OnesCount8(dst[i]))
	}
	return count
}

func popCountFallback(src []byte) uint64 {
	var count uint64
	for _, value := range src {
		count += uint64(bits.OnesCount8(value))
	}
	return count
}
