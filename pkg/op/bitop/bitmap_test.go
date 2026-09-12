package bitop

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"slices"
	"testing"
)

var testLens4Bitmap []int = []int{0, 1, 7, 8, 15, 16, 17, 23, 24, 25, 63, 64, 65, 127, 128, 129, 1_000}
var benchLens4Bitmap []int = []int{10, 100, 1_000, 10_000, 100_000, 1_000_000}

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

// fallback comparison for BitWiseXorWithPopCount
func bitWiseXorWithPopCountFB(src1, src2, dst []byte) uint64 {
	var pc uint64
	for i := 0; i < len(src1); i++ {
		res := src1[i] ^ src2[i]
		pc += uint64(bits.OnesCount8(res))
		dst[i] = res
	}
	return pc
}

// fallback comparison for BitWiseAndNWithPopCount
func bitWiseAndNWithPopCountFB(src1, src2, dst []byte) uint64 {
	var pc uint64
	for i := 0; i < len(src1); i++ {
		res := src1[i] & ^src2[i]
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
	for _, s := range testLens4Bitmap {
		t.Run(fmt.Sprintf("BitWiseAndWithPopCount Size %d", s), func(t *testing.T) {
			// get rand src, dst slices
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)
			dstFallback := genByteSlice(s)
			dstFB := genByteSlice(s)
			// run
			pc := BitWiseAndWithPopCount(src1, src2, dst)
			pcFallback := bitWiseAndWithPopCountFallback(src1, src2, dstFallback)
			pcFB := bitWiseAndWithPopCountFB(src1, src2, dstFB)
			// check population count
			if pc != pcFB {
				t.Errorf("POPCOUNT: got %d, expected %d", pc, pcFB)
			}
			if pcFallback != pcFB {
				t.Errorf("fallback POPCOUNT: got %d, expected %d", pcFallback, pcFB)
			}
			// check bitwise AND result
			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dst[i], dstFB[i])
				}
				if dstFallback[i] != dstFB[i] {
					t.Errorf("Fallback on %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dstFB[i], dstFallback[i])
				}
			}
		})
	}
}

func TestBitWiseOrWithPopCount(t *testing.T) {
	for _, s := range testLens4Bitmap {
		t.Run(fmt.Sprintf("BitWiseOrWithPopCount Size %d", s), func(t *testing.T) {
			// get rand src, dst slices
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)
			dstFallback := genByteSlice(s)
			dstFB := genByteSlice(s)
			// run
			pc := BitWiseOrWithPopCount(src1, src2, dst)
			pcFallback := bitWiseOrWithPopCountFallback(src1, src2, dstFallback)
			pcFB := bitWiseOrWithPopCountFB(src1, src2, dstFB)
			// check population count
			if pc != pcFB {
				t.Errorf("POPCOUNT: got %d, expected %d", pc, pcFB)
			}
			if pcFallback != pcFB {
				t.Errorf("fallback POPCOUNT: got %d, expected %d", pcFallback, pcFB)
			}
			// check bitwise AND result
			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dst[i], dstFB[i])
				}
				if dstFallback[i] != dstFB[i] {
					t.Errorf("Fallback on %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dstFB[i], dstFallback[i])
				}
			}
		})
	}
}

func TestBitWiseXorWithPopCount(t *testing.T) {
	for _, s := range testLens4Bitmap {
		t.Run(fmt.Sprintf("BitWiseXorWithPopCount Size %d", s), func(t *testing.T) {
			// get rand src, dst slices
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)
			dstFallback := genByteSlice(s)
			dstFB := genByteSlice(s)
			// run
			pc := BitWiseXorWithPopCount(src1, src2, dst)
			pcFallback := bitWiseXorWithPopCountFallback(src1, src2, dstFallback)
			pcFB := bitWiseXorWithPopCountFB(src1, src2, dstFB)
			// check population count
			if pc != pcFB {
				t.Errorf("POPCOUNT: got %d, expected %d", pc, pcFB)
			}
			if pcFallback != pcFB {
				t.Errorf("fallback POPCOUNT: got %d, expected %d", pcFallback, pcFB)
			}
			// check bitwise AND result
			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dst[i], dstFB[i])
				}
				if dstFallback[i] != dstFB[i] {
					t.Errorf("Fallback on %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dstFB[i], dstFallback[i])
				}
			}
		})
	}
}

func TestBitWiseAndNWithPopCount(t *testing.T) {
	for _, s := range testLens4Bitmap {
		t.Run(fmt.Sprintf("BitWiseAndNWithPopCount Size %d", s), func(t *testing.T) {
			// get rand src, dst slices
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)
			dstFallback := genByteSlice(s)
			dstFB := genByteSlice(s)
			// run
			pc := BitWiseAndNWithPopCount(src1, src2, dst)
			pcFallback := bitWiseAndNWithPopCountFallback(src1, src2, dstFallback)
			pcFB := bitWiseAndNWithPopCountFB(src1, src2, dstFB)
			// check population count
			if pc != pcFB {
				t.Errorf("POPCOUNT: got %d, expected %d", pc, pcFB)
			}
			if pcFallback != pcFB {
				t.Errorf("fallback POPCOUNT: got %d, expected %d", pcFallback, pcFB)
			}
			// check bitwise ANDN result
			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dst[i], dstFB[i])
				}
				if dstFallback[i] != dstFB[i] {
					t.Errorf("Fallback on %d: src1: %08b, src2: %08b; expected %08b, got %08b", i, src1[i], src2[i], dstFB[i], dstFallback[i])
				}
			}
		})
	}
}

func TestPopCount(t *testing.T) {
	for _, s := range testLens4Bitmap {
		t.Run(fmt.Sprintf("PopCount Size %d", s), func(t *testing.T) {
			// get rand src
			src := genRandByteData(s)
			// run
			pc := PopCount(src)
			pcFallback := popCountFallback(src)
			pcFB := popCountFB(src)
			// check population count
			if pc != pcFB {
				t.Errorf("got %d, expected %d", pc, pcFB)
			}
			if pcFallback != pcFB {
				t.Errorf("fallback got %d, expected %d", pcFallback, pcFB)
			}
		})
	}
}

func TestBitmapAliasing(t *testing.T) {
	operations := []struct {
		name      string
		operation func([]byte, []byte, []byte) uint64
		reference func([]byte, []byte, []byte) uint64
	}{
		{"And", BitWiseAndWithPopCount, bitWiseAndWithPopCountFB},
		{"AndFallback", bitWiseAndWithPopCountFallback, bitWiseAndWithPopCountFB},
		{"Or", BitWiseOrWithPopCount, bitWiseOrWithPopCountFB},
		{"OrFallback", bitWiseOrWithPopCountFallback, bitWiseOrWithPopCountFB},
		{"Xor", BitWiseXorWithPopCount, bitWiseXorWithPopCountFB},
		{"XorFallback", bitWiseXorWithPopCountFallback, bitWiseXorWithPopCountFB},
		{"AndN", BitWiseAndNWithPopCount, bitWiseAndNWithPopCountFB},
		{"AndNFallback", bitWiseAndNWithPopCountFallback, bitWiseAndNWithPopCountFB},
	}

	for _, operation := range operations {
		for _, alias := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/Src%d", operation.name, alias), func(t *testing.T) {
				src1 := genRandByteData(65)
				src2 := genRandByteData(65)
				expected := genByteSlice(65)
				expectedCount := operation.reference(src1, src2, expected)

				var count uint64
				if alias == 1 {
					count = operation.operation(src1, src2, src1)
					if !slices.Equal(src1, expected) {
						t.Fatalf("aliased src1 result differs from expected")
					}
				} else {
					count = operation.operation(src1, src2, src2)
					if !slices.Equal(src2, expected) {
						t.Fatalf("aliased src2 result differs from expected")
					}
				}
				if count != expectedCount {
					t.Fatalf("got popcount %d, expected %d", count, expectedCount)
				}
			})
		}
	}
}

var blackhole uint64

func BenchmarkBitWiseAndWithPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens4Bitmap {
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
	for _, s := range benchLens4Bitmap {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := bitWiseAndWithPopCountFallback(src1, src2, dst)
				blackhole = pc
			}
		})
	}
}

func BenchmarkBitWiseOrWithPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens4Bitmap {
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
	for _, s := range benchLens4Bitmap {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := bitWiseOrWithPopCountFallback(src1, src2, dst)
				blackhole = pc
			}
		})
	}
}

func BenchmarkBitWiseXorWithPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens4Bitmap {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := BitWiseXorWithPopCount(src1, src2, dst)
				blackhole = pc
			}
		})
	}
	// fallback
	for _, s := range benchLens4Bitmap {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := bitWiseXorWithPopCountFallback(src1, src2, dst)
				blackhole = pc
			}
		})
	}
}

func BenchmarkBitWiseAndNWithPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens4Bitmap {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := BitWiseAndNWithPopCount(src1, src2, dst)
				blackhole = pc
			}
		})
	}
	// fallback
	for _, s := range benchLens4Bitmap {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src1 := genRandByteData(s)
			src2 := genRandByteData(s)
			dst := genByteSlice(s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := bitWiseAndNWithPopCountFallback(src1, src2, dst)
				blackhole = pc
			}
		})
	}
}

func BenchmarkPopCount(b *testing.B) {
	// assembly
	for _, s := range benchLens4Bitmap {
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
	for _, s := range benchLens4Bitmap {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandByteData(s)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pc := popCountFallback(src)
				blackhole = pc
			}
		})
	}
}
