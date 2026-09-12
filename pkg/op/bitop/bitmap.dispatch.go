//go:build !arm64

package bitop

// BitWiseAndWithPopCount combines `src1` and `src2`, places the bitwise
// AND of their bytes in `dst`, and returns the final population count of `dst`.
func BitWiseAndWithPopCount(src1, src2, dst []byte) uint64 {
	return bitWiseAndWithPopCountImpl(src1, src2, dst)
}

// BitWiseOrWithPopCount combines `src1` and `src2`, places the bitwise
// OR of their bytes in `dst`, and returns the final population count of `dst`.
func BitWiseOrWithPopCount(src1, src2, dst []byte) uint64 {
	return bitWiseOrWithPopCountImpl(src1, src2, dst)
}

// BitWiseAndNWithPopCount combines `src1` and `src2`, places the bitwise
// AND NOT of their bytes in `dst`, and returns the final population count of `dst`.
func BitWiseAndNWithPopCount(src1, src2, dst []byte) uint64 {
	return bitWiseAndNWithPopCountImpl(src1, src2, dst)
}

// BitWiseXorWithPopCount combines `src1` and `src2`, places the bitwise
// XOR of their bytes in `dst`, and returns the final population count of `dst`.
func BitWiseXorWithPopCount(src1, src2, dst []byte) uint64 {
	return bitWiseXorWithPopCountImpl(src1, src2, dst)
}

// PopCount returns the population count of `src`.
func PopCount(src []byte) uint64 {
	return popCountImpl(src)
}
