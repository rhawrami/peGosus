//go:build arm64

package op

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"testing"
)

var testLens []int = []int{0, 100, 1_000, 10_000, 100_000, 1_000_000}
var benchLens []int = []int{10, 100, 1_000, 10_000, 100_000, 1_000_000}

// fallback comparison for BitWiseAndWithPopCount
func bitWiseAndWithPopCountFB(src1, src2, dst []byte) uint64 {
	var pc uint64
	for i := 0; i < len(src1); i++ {
		res := src1[i] & src2[i]
		pc += uint64(bits.OnesCount8(res))
		dst[i] = res
	}
	return pc
}

// fallback comparison for BitWiseOrWithPopCount
func bitWiseOrWithPopCountFB(src1, src2, dst []byte) uint64 {
	var pc uint64
	for i := 0; i < len(src1); i++ {
		res := src1[i] | src2[i]
		pc += uint64(bits.OnesCount8(res))
		dst[i] = res
	}
	return pc
}

// fallback comparison for PopCount
func popCountFB(src []byte) uint64 {
	var pc uint64
	for i := 0; i < len(src); i++ {
		pc += uint64(bits.OnesCount8(src[i]))
	}
	return pc
}

func genRandByteData(l int) []byte {
	a := make([]byte, l)
	for i := 0; i < l; i++ {
		a[i] = byte(rand.Uint32N(256))
	}
	return a
}

func genByteSlice(l int) []byte {
	return make([]byte, l)
}

func TestBitWiseAndWithPopCount(t *testing.T) {
	for _, s := range testLens {
		t.Run(fmt.Sprintf("BitWiseAndWithPopCount Size %d", s), func(t *testing.T) {
			// get rand src, dst slices
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)
			dstFB := genByteSlice(s)
			// run
			pc := BitWiseAndWithPopCount(src1, src2, dst)
			pcFB := bitWiseAndWithPopCountFB(src1, src2, dstFB)
			// check population count
			if pc != pcFB {
				t.Errorf("POPCOUNT: got %d, expected %d", pc, pcFB)
			}
			// check bitwise AND result
			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dst[i], dstFB[i])
				}
			}
		})
	}
}

func TestBitWiseOrWithPopCount(t *testing.T) {
	for _, s := range testLens {
		t.Run(fmt.Sprintf("BitWiseOrWithPopCount Size %d", s), func(t *testing.T) {
			// get rand src, dst slices
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)
			dstFB := genByteSlice(s)
			// run
			pc := BitWiseOrWithPopCount(src1, src2, dst)
			pcFB := bitWiseOrWithPopCountFB(src1, src2, dstFB)
			// check population count
			if pc != pcFB {
				t.Errorf("POPCOUNT: got %d, expected %d", pc, pcFB)
			}
			// check bitwise AND result
			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dst[i], dstFB[i])
				}
			}
		})
	}
}

func TestPopCount(t *testing.T) {
	for _, s := range testLens {
		t.Run(fmt.Sprintf("PopCount Size %d", s), func(t *testing.T) {
			// get rand src
			src := genRandByteData(s)
			// run
			pc := PopCount(src)
			pcFB := popCountFB(src)
			// check population count
			if pc != pcFB {
				t.Errorf("got %d, expected %d", pc, pcFB)
			}
		})
	}
}

var blackhole uint64

func BenchmarkBitWiseAndWithPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := BitWiseAndWithPopCount(src1, src2, dst)
				blackhole = pc
			}
		})
	}
	// fallback
	for _, s := range benchLens {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := bitWiseAndWithPopCountFB(src1, src2, dst)
				blackhole = pc
			}
		})
	}
}

func BenchmarkBitWiseOrWithPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := BitWiseOrWithPopCount(src1, src2, dst)
				blackhole = pc
			}
		})
	}
	// fallback
	for _, s := range benchLens {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := bitWiseOrWithPopCountFB(src1, src2, dst)
				blackhole = pc
			}
		})
	}
}

func BenchmarkPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandByteData(s)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := PopCount(src)
				blackhole = pc
			}
		})
	}
	// fallback
	for _, s := range benchLens {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandByteData(s)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := popCountFB(src)
				blackhole = pc
			}
		})
	}
}
