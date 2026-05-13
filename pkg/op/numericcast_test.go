//go:build arm64

package op

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

const testLenCast int = 5096

var benchLenCasts []int = []int{10, 100, 1_000, 10_000, 100_000, 1_000_000}

// fallback comparison of CastI64ToF64
func castI64ToF64FB(src []int64, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = float64(src[i])
	}
}

// fallback comparison of CastI32ToF32
func castI32ToF32FB(src []int32, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = float32(src[i])
	}
}

// fallback comparison of CastF64ToI64
func castF64ToI64FB(src []float64, dst []int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = int64(src[i])
	}
}

// fallback comparison of CastF32ToI32
func castF32ToI32FB(src []float32, dst []int32) {
	for i := 0; i < len(src); i++ {
		dst[i] = int32(src[i])
	}
}

// fallback comparison of CastF32ToF64
func castF32ToF64FB(src []float32, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = float64(src[i])
	}
}

// fallback comparison of CastI32ToI64
func castI32ToI64FB(src []int32, dst []int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = int64(src[i])
	}
}

// fallback comparison of CastI32ToF64
func castI32ToF64FB(src []int32, dst []float64) {
	for i := 0; i < len(src); i++ {
		dst[i] = float64(src[i])
	}
}

// fallback comparison of CastF32ToI64
func castF32ToI64FB(src []float32, dst []int64) {
	for i := 0; i < len(src); i++ {
		dst[i] = int64(src[i])
	}
}

// fallback comparison of CastI64ToF32
func castI64ToF32FB(src []int64, dst []float32) {
	for i := 0; i < len(src); i++ {
		dst[i] = float32(src[i])
	}
}

// fallback comparison of CastF64ToI32
func castF64ToI32FB(src []float64, dst []int32) {
	for i := 0; i < len(src); i++ {
		dst[i] = int32(src[i])
	}
}

func genRandCastDataI64(l int) []int64 {
	s := make([]int64, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Int64()
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genRandCastDataI32(l int) []int32 {
	s := make([]int32, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Int32()
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genRandCastDataF64(l int) []float64 {
	s := make([]float64, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Float64() * 100_000
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genRandCastDataF32(l int) []float32 {
	s := make([]float32, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Float32() * 100_000
		if i%2 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func TestCastI64ToF64(t *testing.T) {
	src := genRandCastDataI64(testLenCast)
	dst := make([]float64, testLenCast)
	dstFB := make([]float64, testLenCast)

	CastI64ToF64(src, dst)
	castI64ToF64FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastI32ToF32(t *testing.T) {
	src := genRandCastDataI32(testLenCast)
	dst := make([]float32, testLenCast)
	dstFB := make([]float32, testLenCast)

	CastI32ToF32(src, dst)
	castI32ToF32FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastF64ToI64(t *testing.T) {
	src := genRandCastDataF64(testLenCast)
	dst := make([]int64, testLenCast)
	dstFB := make([]int64, testLenCast)

	CastF64ToI64(src, dst)
	castF64ToI64FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastF32ToI32(t *testing.T) {
	src := genRandCastDataF32(testLenCast)
	dst := make([]int32, testLenCast)
	dstFB := make([]int32, testLenCast)

	CastF32ToI32(src, dst)
	castF32ToI32FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastF32ToF64(t *testing.T) {
	src := genRandCastDataF32(testLenCast)
	dst := make([]float64, testLenCast)
	dstFB := make([]float64, testLenCast)

	CastF32ToF64(src, dst)
	castF32ToF64FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastI32ToI64(t *testing.T) {
	src := genRandCastDataI32(testLenCast)
	dst := make([]int64, testLenCast)
	dstFB := make([]int64, testLenCast)

	CastI32ToI64(src, dst)
	castI32ToI64FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}
func TestCastI32ToF64(t *testing.T) {
	src := genRandCastDataI32(testLenCast)
	dst := make([]float64, testLenCast)
	dstFB := make([]float64, testLenCast)

	CastI32ToF64(src, dst)
	castI32ToF64FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastF32ToI64(t *testing.T) {
	src := genRandCastDataF32(testLenCast)
	dst := make([]int64, testLenCast)
	dstFB := make([]int64, testLenCast)

	CastF32ToI64(src, dst)
	castF32ToI64FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastI64ToF32(t *testing.T) {
	src := genRandCastDataI64(testLenCast)
	dst := make([]float32, testLenCast)
	dstFB := make([]float32, testLenCast)

	CastI64ToF32(src, dst)
	castI64ToF32FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

func TestCastF64ToI32(t *testing.T) {
	src := genRandCastDataF64(testLenCast)
	dst := make([]int32, testLenCast)
	dstFB := make([]int32, testLenCast)

	CastF64ToI32(src, dst)
	castF64ToI32FB(src, dstFB)

	for i := 0; i < testLenCast; i++ {
		if dst[i] != dstFB[i] {
			t.Errorf("On %d: got %v, expected %v", i, dst[i], dstFB[i])
		}
	}
}

var bhI64Cast []int64
var bhF64Cast []float64
var bhI32Cast []int32
var bhF32Cast []float32

func BenchmarkCastI64ToF64(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataI64(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastI64ToF64(src, dst)
				bhF64Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataI64(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castI64ToF64FB(src, dst)
				bhF64Cast = dst
			}
		})
	}
}

func BenchmarkCastI32ToF32(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataI32(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastI32ToF32(src, dst)
				bhF32Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataI32(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castI32ToF32FB(src, dst)
				bhF32Cast = dst
			}
		})
	}
}

func BenchmarkCastF64ToI64(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataF64(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastF64ToI64(src, dst)
				bhI64Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataF64(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castF64ToI64FB(src, dst)
				bhI64Cast = dst
			}
		})
	}
}

func BenchmarkCastF32ToI32(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataF32(s)
			dst := make([]int32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastF32ToI32(src, dst)
				bhI32Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataF32(s)
			dst := make([]int32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castF32ToI32FB(src, dst)
				bhI32Cast = dst
			}
		})
	}
}

func BenchmarkCastF32ToF64(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataF32(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastF32ToF64(src, dst)
				bhF64Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataF32(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castF32ToF64FB(src, dst)
				bhF64Cast = dst
			}
		})
	}
}

func BenchmarkCastI32ToI64(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataI32(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastI32ToI64(src, dst)
				bhI64Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataI32(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castI32ToI64FB(src, dst)
				bhI64Cast = dst
			}
		})
	}
}

func BenchmarkCastI32ToF64(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataI32(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastI32ToF64(src, dst)
				bhF64Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataI32(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castI32ToF64FB(src, dst)
				bhF64Cast = dst
			}
		})
	}
}

func BenchmarkCastF32ToI64(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataF32(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastF32ToI64(src, dst)
				bhI64Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataF32(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castF32ToI64FB(src, dst)
				bhI64Cast = dst
			}
		})
	}
}

func BenchmarkCastI64ToF32(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataI64(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastI64ToF32(src, dst)
				bhF32Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataI64(s)
			dst := make([]float32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castI64ToF32FB(src, dst)
				bhF32Cast = dst
			}
		})
	}
}

func BenchmarkCastF64ToI32(b *testing.B) {
	for _, s := range benchLenCasts {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genRandCastDataF64(s)
			dst := make([]int32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CastF64ToI32(src, dst)
				bhI32Cast = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genRandCastDataF64(s)
			dst := make([]int32, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				castF64ToI32FB(src, dst)
				bhI32Cast = dst
			}
		})
	}
}
