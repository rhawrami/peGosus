//go:build arm64

package bitop

// BitWiseAndWithPopCount combines `src1` and `src2`, places the bitwise
// AND of their bytes in `dst`, and returns the final population count of `dst`.
//
//go:noescape
func BitWiseAndWithPopCount(src1, src2, dst []byte) uint64

// BitWiseOrWithPopCount combines `src1` and `src2`, places the bitwise
// OR of their bytes in `dst`, and returns the final population count of `dst`.
//
//go:noescape
func BitWiseOrWithPopCount(src1, src2, dst []byte) uint64

// BitWiseAndNWithPopCount combines `src1` and `src2`, places the bitwise
// AND NOT of their bytes in `dst`, and returns the final population count of `dst`.
//
//go:noescape
func BitWiseAndNWithPopCount(src1, src2, dst []byte) uint64

// PopCount returns the population count of `src`.
//
//go:noescape
func PopCount(src []byte) uint64
