//go:build arm64

package op

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

const testLenSum int = 10_000

var benchLenSum []int = []int{10, 100, 1_000, 10_000, 100_000, 1_000_000}

// fallback comparison of SumI64
func sumI64FB(src, dst []int64) {
	var sum int64
	for i := 0; i < len(src); i++ {
		sum += src[i]
	}
	dst[0] = sum
}

// fallback comparison of SumI32
func sumI32FB(src []int32, dst []int64) {
	var sum int64
	for i := 0; i < len(src); i++ {
		sum += int64(src[i])
	}
	dst[0] = sum
}

// fallback comparison of SumF64
func sumF64FB(src, dst []float64) {
	var sum float64
	for i := 0; i < len(src); i++ {
		if src[i] == src[i] {
			sum += src[i]
		}
	}
	dst[0] = sum
}

// fallback comparison of SumF32
func sumF32FB(src []float32, dst []float64) {
	var sum float64
	for i := 0; i < len(src); i++ {
		if src[i] == src[i] {
			sum += float64(src[i])
		}
	}
	dst[0] = sum
}

// fallback comparison of SumI64WithValidity
func sumI64WithValidityFB(src, dst []int64, validity []byte) {
	var sum int64
	for i := 0; i < len(src); i++ {
		if (validity[i/8]>>(i%8))&byte(1) == 1 {
			sum += src[i]
		}
	}
	dst[0] = sum
}

// fallback comparison of SumI32WithValidity
func sumI32WithValidityFB(src []int32, dst []int64, validity []byte) {
	var sum int64
	for i := 0; i < len(src); i++ {
		if (validity[i/8]>>(i%8))&byte(1) == 1 {
			sum += int64(src[i])
		}
	}
	dst[0] = sum
}

// fallback comparison of SumF64WithValidity
func sumF64WithValidityFB(src, dst []float64, validity []byte) {
	var sum float64
	for i := 0; i < len(src); i++ {
		if ((validity[i/8]>>(i%8))&byte(1) == 1) && src[i] == src[i] {
			sum += src[i]
		}
	}
	dst[0] = sum
}

// fallback comparison of SumF32WithValidity
func sumF32WithValidityFB(src []float32, dst []float64, validity []byte) {
	var sum float64
	for i := 0; i < len(src); i++ {
		if ((validity[i/8]>>(i%8))&byte(1) == 1) && src[i] == src[i] {
			sum += float64(src[i])
		}
	}
	dst[0] = sum
}

func genSumDataI64(l int) []int64 {
	s := make([]int64, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Int64()
		if i%5 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genSumDataI32(l int) []int32 {
	s := make([]int32, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Int32()
		if i%5 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genSumDataF64(l int) []float64 {
	s := make([]float64, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Float64() * 100_000
		if i%7 == 0 {
			s[i] = math.NaN()
			continue
		}
		if i%5 == 0 {
			s[i] *= -1
		}
	}
	return s
}

func genSumDataF32(l int) []float32 {
	s := make([]float32, l)
	for i := 0; i < l; i++ {
		s[i] = rand.Float32() * 100_000
		if i%5 == 0 {
			s[i] *= -1
			continue
		}
		if i%7 == 0 {
			s[i] = float32(math.NaN())
		}
	}
	return s
}

func genSumDataB(l int) []byte {
	s := make([]byte, (l+7)/8)
	for i := 0; i < len(s); i++ {
		s[i] = byte(rand.Int32N(255))
	}
	return s
}

func checkIfCloseF64(x, y float64) bool {
	acceptableDiff := float64(1 / math.Pow10(7))
	if math.Abs(x-y) < acceptableDiff {
		return true
	}
	return false
}

func TestSumI64(t *testing.T) {
	src := genSumDataI64(testLenSum)
	dst, dstFB := make([]int64, 1), make([]int64, 1)

	SumI64(src, dst)
	sumI64FB(src, dstFB)

	if dst[0] != dstFB[0] {
		t.Errorf("Got %d, expected %d", dst[0], dstFB[0])
	}
}

func TestSumI32(t *testing.T) {
	src := genSumDataI32(testLenSum)
	dst, dstFB := make([]int64, 1), make([]int64, 1)

	SumI32(src, dst)
	sumI32FB(src, dstFB)

	if dst[0] != dstFB[0] {
		t.Errorf("Got %d, expected %d", dst[0], dstFB[0])
	}
}

func TestSumF64(t *testing.T) {
	src := genSumDataF64(testLenSum)
	dst, dstFB := make([]float64, 1), make([]float64, 1)

	SumF64(src, dst)
	sumF64FB(src, dstFB)

	if !checkIfCloseF64(dst[0], dstFB[0]) {
		t.Errorf("Got %.10f, expected %.10f, diff: %.10f", dst[0], dstFB[0], dst[0]-dstFB[0])
	}
}

func TestSumF32(t *testing.T) {
	src := genSumDataF32(testLenSum)
	dst, dstFB := make([]float64, 1), make([]float64, 1)

	SumF32(src, dst)
	sumF32FB(src, dstFB)

	if !checkIfCloseF64(dst[0], dstFB[0]) {
		t.Errorf("Got %.10f, expected %.10f, diff: %.10f", dst[0], dstFB[0], dst[0]-dstFB[0])
	}
}

func TestSumI64WithValidity(t *testing.T) {
	src := genSumDataI64(testLenSum)
	validity := genSumDataB(testLenSum)
	dst, dstFB := make([]int64, 1), make([]int64, 1)

	SumI64WithValidity(src, dst, validity)
	sumI64WithValidityFB(src, dstFB, validity)

	if dst[0] != dstFB[0] {
		t.Errorf("Got %d, expected %d", dst[0], dstFB[0])
	}
}

func TestSumI32WithValidity(t *testing.T) {
	src := genSumDataI32(testLenSum)
	validity := genSumDataB(testLenSum)
	dst, dstFB := make([]int64, 1), make([]int64, 1)

	SumI32WithValidity(src, dst, validity)
	sumI32WithValidityFB(src, dstFB, validity)

	if dst[0] != dstFB[0] {
		t.Errorf("Got %d, expected %d", dst[0], dstFB[0])
	}
}

func TestSumF64WithValidity(t *testing.T) {
	src := genSumDataF64(testLenSum)
	validity := genSumDataB(testLenSum)
	dst, dstFB := make([]float64, 1), make([]float64, 1)

	SumF64WithValidity(src, dst, validity)
	sumF64WithValidityFB(src, dstFB, validity)

	if !checkIfCloseF64(dst[0], dstFB[0]) {
		t.Errorf("Got %.10f, expected %.10f, diff: %.10f", dst[0], dstFB[0], dst[0]-dstFB[0])
	}
}

func TestSumF32WithValidity(t *testing.T) {
	src := genSumDataF32(testLenSum)
	validity := genSumDataB(testLenSum)
	dst, dstFB := make([]float64, 1), make([]float64, 1)

	SumF32WithValidity(src, dst, validity)
	sumF32WithValidityFB(src, dstFB, validity)

	if !checkIfCloseF64(dst[0], dstFB[0]) {
		t.Errorf("Got %.10f, expected %.10f, diff: %.10f", dst[0], dstFB[0], dst[0]-dstFB[0])
	}
}

var bhI64Sum []int64
var bhF64Sum []float64

func BenchmarkSumI64(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataI64(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumI64(src, dst)
				bhI64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataI64(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumI64FB(src, dst)
				bhI64Sum = dst
			}
		})
	}
}

func BenchmarkSumI32(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataI32(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumI32(src, dst)
				bhI64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataI32(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumI32FB(src, dst)
				bhI64Sum = dst
			}
		})
	}
}

func BenchmarkSumF64(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataF64(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumF64(src, dst)
				bhF64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataF64(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumF64FB(src, dst)
				bhF64Sum = dst
			}
		})
	}
}

func BenchmarkSumF32(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataF32(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumF32(src, dst)
				bhF64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataF32(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumF32FB(src, dst)
				bhF64Sum = dst
			}
		})
	}
}

func BenchmarkSumI64WithValidity(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataI64(s)
			validity := genSumDataB(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumI64WithValidity(src, dst, validity)
				bhI64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataI64(s)
			validity := genSumDataB(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumI64WithValidityFB(src, dst, validity)
				bhI64Sum = dst
			}
		})
	}
}

func BenchmarkSumI32WithValidity(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataI32(s)
			validity := genSumDataB(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumI32WithValidity(src, dst, validity)
				bhI64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataI32(s)
			validity := genSumDataB(s)
			dst := make([]int64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumI32WithValidityFB(src, dst, validity)
				bhI64Sum = dst
			}
		})
	}
}

func BenchmarkSumF64WithValidity(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataF64(s)
			validity := genSumDataB(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumF64WithValidity(src, dst, validity)
				bhF64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataF64(s)
			validity := genSumDataB(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumF64WithValidityFB(src, dst, validity)
				bhF64Sum = dst
			}
		})
	}
}

func BenchmarkSumF32WithValidity(b *testing.B) {
	for _, s := range benchLenSum {
		// assembly
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			src := genSumDataF32(s)
			validity := genSumDataB(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SumF32WithValidity(src, dst, validity)
				bhF64Sum = dst
			}
		})
	}
	for _, s := range benchLens4Clip {
		// fallback
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			src := genSumDataF32(s)
			validity := genSumDataB(s)
			dst := make([]float64, s)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sumF32WithValidityFB(src, dst, validity)
				bhF64Sum = dst
			}
		})
	}
}
