//go:build arm64

package op

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

var testLen4Clip = 5096
var benchLens4Clip []int = []int{10, 100, 1_000, 10_000, 100_000, 1_000_000}
var upperI64 int64 = 1_000_000
var upperI32 int32 = 1_000_000
var upperF64 float64 = 1_000_000
var upperF32 float32 = 1_000_000

// fallback comparison of ClipF64WithF64Bounds
func clipF64WithF64BoundsFB(src, dst []float64, lower, upper float64) {
	for i := 0; i < len(src); i++ {
		var x float64 = src[i]
		if x < lower {
			x = lower
		}
		if x > upper {
			x = upper
		}
		dst[i] = x
	}
}

// fallback comparison of ClipF32WithF32Bounds
func clipF32WithF32BoundsFB(src, dst []float32, lower, upper float32) {
	for i := 0; i < len(src); i++ {
		var x float32 = src[i]
		if x < lower {
			x = lower
		}
		if x > upper {
			x = upper
		}
		dst[i] = x
	}
}

// fallback comparison of ClipI32WithI32Bounds
func clipI32WithI32BoundsFB(src, dst []int32, lower, upper int32) {
	for i := 0; i < len(src); i++ {
		var x int32 = src[i]
		if x < lower {
			x = lower
		}
		if x > upper {
			x = upper
		}
		dst[i] = x
	}
}

// fallback comparison of ClipI64WithI64Bounds
func clipI64WithI64BoundsFB(src, dst []int64, lower, upper int64) {
	for i := 0; i < len(src); i++ {
		var x int64 = src[i]
		if x < lower {
			x = lower
		}
		if x > upper {
			x = upper
		}
		dst[i] = x
	}
}

// fallback comparison of ClipI64WithF64Bounds
func clipI64WithF64BoundsFB(src []int64, dst []float64, lower, upper float64) {
	for i := 0; i < len(src); i++ {
		var x float64 = float64(src[i])
		if x < lower {
			x = lower
		}
		if x > upper {
			x = upper
		}
		dst[i] = x
	}
}

// fallback comparison of ClipI32WithF32Bounds
func clipI32WithF32BoundsFB(src []int32, dst []float32, lower, upper float32) {
	for i := 0; i < len(src); i++ {
		var x float32 = float32(src[i])
		if x < lower {
			x = lower
		}
		if x > upper {
			x = upper
		}
		dst[i] = x
	}
}

func genRandI64Data4Clip(l int) []int64 {
	s := make([]int64, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Int64N(upperI64 + 1)
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genRandI32Data4Clip(l int) []int32 {
	s := make([]int32, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Int32N(upperI32 + 1)
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genRandF64Data4Clip(l int) []float64 {
	s := make([]float64, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Float64() * upperF64
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genRandF32Data4Clip(l int) []float32 {
	s := make([]float32, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Float32() * upperF32
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func TestClipI64WithI64Bounds(t *testing.T) {
	src := genRandI64Data4Clip(testLen4Clip)
	dst := make([]int64, testLen4Clip)
	dstFB := make([]int64, testLen4Clip)

	// clip within [-500k, 500k]
	ClipI64WithI64Bounds(src, dst, upperI64/2*-1, upperI64/2)
	clipI64WithI64BoundsFB(src, dstFB, upperI64/2*-1, upperI64/2)

	for i := 0; i < testLen4Clip; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %d, expected %d", i, dst[i], dstFB[i])
		}
	}
}

func TestClipF64WithF64Bounds(t *testing.T) {
	src := genRandF64Data4Clip(testLen4Clip)
	dst := make([]float64, testLen4Clip)
	dstFB := make([]float64, testLen4Clip)

	// clip within [-500k, 500k]
	ClipF64WithF64Bounds(src, dst, upperF64/2*-1, upperF64/2)
	clipF64WithF64BoundsFB(src, dstFB, upperF64/2*-1, upperF64/2)

	for i := 0; i < testLen4Clip; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
		}
	}
}

func TestClipI32WithI32Bounds(t *testing.T) {
	src := genRandI32Data4Clip(testLen4Clip)
	dst := make([]int32, testLen4Clip)
	dstFB := make([]int32, testLen4Clip)

	// clip within [-500k, 500k]
	ClipI32WithI32Bounds(src, dst, upperI32/2*-1, upperI32/2)
	clipI32WithI32BoundsFB(src, dstFB, upperI32/2*-1, upperI32/2)

	for i := 0; i < testLen4Clip; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %d, expected %d", i, dst[i], dstFB[i])
		}
	}
}

func TestClipF32WithF32Bounds(t *testing.T) {
	src := genRandF32Data4Clip(testLen4Clip)
	dst := make([]float32, testLen4Clip)
	dstFB := make([]float32, testLen4Clip)

	// clip within [-500k, 500k]
	ClipF32WithF32Bounds(src, dst, upperF32/2*-1, upperF32/2)
	clipF32WithF32BoundsFB(src, dstFB, upperF32/2*-1, upperF32/2)

	for i := 0; i < testLen4Clip; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
		}
	}
}

func TestClipI64WithF64Bounds(t *testing.T) {
	src := genRandI64Data4Clip(testLen4Clip)
	dst := make([]float64, testLen4Clip)
	dstFB := make([]float64, testLen4Clip)

	// clip within [-500k, 500k]
	ClipI64WithF64Bounds(src, dst, upperF64/2*-1, upperF64/2)
	clipI64WithF64BoundsFB(src, dstFB, upperF64/2*-1, upperF64/2)

	for i := 0; i < testLen4Clip; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
		}
	}
}

func TestClipI32WithF32Bounds(t *testing.T) {
	src := genRandI32Data4Clip(testLen4Clip)
	dst := make([]float32, testLen4Clip)
	dstFB := make([]float32, testLen4Clip)

	// clip within [-500k, 500k]
	ClipI32WithF32Bounds(src, dst, upperF32/2*-1, upperF32/2)
	clipI32WithF32BoundsFB(src, dstFB, upperF32/2*-1, upperF32/2)

	for i := 0; i < testLen4Clip; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
		}
	}
}

var bhI64Clip []int64
var bhF64Clip []float64
var bhI32Clip []int32
var bhF32Clip []float32

func BenchmarkClipI64WithI64Bounds(b *testing.B) {
	for _, s := range benchLens4Clip {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandI64Data4Clip(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ClipI64WithI64Bounds(src, dst, upperI64/2*-1, upperI64/2)
				bhI64Clip = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandI64Data4Clip(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clipI64WithI64BoundsFB(src, dst, upperI64/2*-1, upperI64/2)
				bhI64Clip = dst
			}
		})
	}
}

func BenchmarkClipF64WithF64Bounds(b *testing.B) {
	for _, s := range benchLens4Clip {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandF64Data4Clip(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ClipF64WithF64Bounds(src, dst, upperF64/2*-1, upperF64/2)
				bhF64Clip = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandF64Data4Clip(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clipF64WithF64BoundsFB(src, dst, upperF64/2*-1, upperF64/2)
				bhF64Clip = dst
			}
		})
	}
}

func BenchmarkClipI32WithI32Bounds(b *testing.B) {
	for _, s := range benchLens4Clip {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandI32Data4Clip(s)
			dst := make([]int32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ClipI32WithI32Bounds(src, dst, upperI32/2*-1, upperI32/2)
				bhI32Clip = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandI32Data4Clip(s)
			dst := make([]int32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clipI32WithI32BoundsFB(src, dst, upperI32/2*-1, upperI32/2)
				bhI32Clip = dst
			}
		})
	}
}

func BenchmarkClipF32WithF32Bounds(b *testing.B) {
	for _, s := range benchLens4Clip {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandF32Data4Clip(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ClipF32WithF32Bounds(src, dst, upperF32/2*-1, upperF32/2)
				bhF32Clip = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandF32Data4Clip(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clipF32WithF32BoundsFB(src, dst, upperF32/2*-1, upperF32/2)
				bhF32Clip = dst
			}
		})
	}
}

func BenchmarkClipI64WithF64Bounds(b *testing.B) {
	for _, s := range benchLens4Clip {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandI64Data4Clip(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ClipI64WithF64Bounds(src, dst, upperF64/2*-1, upperF64/2)
				bhF64Clip = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandI64Data4Clip(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clipI64WithF64BoundsFB(src, dst, upperF64/2*-1, upperF64/2)
				bhF64Clip = dst
			}
		})
	}
}

func BenchmarkClipI32WithF32Bounds(b *testing.B) {
	for _, s := range benchLens4Clip {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandI32Data4Clip(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ClipI32WithF32Bounds(src, dst, upperF32/2*-1, upperF32/2)
				bhF32Clip = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandI32Data4Clip(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clipI32WithF32BoundsFB(src, dst, upperF32/2*-1, upperF32/2)
				bhF32Clip = dst
			}
		})
	}
}
