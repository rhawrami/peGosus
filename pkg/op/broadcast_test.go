//go:build arm64

package op

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"
)

var testLens4Broadcast []int = []int{0, 100, 100_000, 1_000_000}
var testFloatConsts []float64 = []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
var benchLens4Broadcast []int = []int{10, 100, 1_000, 10_000, 100_000, 1_000_000}

// fallback comparison of BroadcastU8
func broadcastU8FB(dst []byte, lit byte) {
	for i := 0; i < len(dst); i++ {
		dst[i] = lit
	}
}

// fallback comparison of BroadcastI64
func broadcastI64FB(dst []int64, lit int64) {
	for i := 0; i < len(dst); i++ {
		dst[i] = lit
	}
}

// fallback comparison of BroadcastI32
func broadcastI32FB(dst []int32, lit int32) {
	for i := 0; i < len(dst); i++ {
		dst[i] = lit
	}
}

// fallback comparison of BroadcastF64
func broadcastF64FB(dst []float64, lit float64) {
	for i := 0; i < len(dst); i++ {
		dst[i] = lit
	}
}

// fallback comparison of BroadcastF32
func broadcastF32FB(dst []float32, lit float32) {
	for i := 0; i < len(dst); i++ {
		dst[i] = lit
	}
}

func TestBroadcastU8(t *testing.T) {
	for _, s := range testLens4Broadcast {
		t.Run(fmt.Sprintf("Size %d", s), func(t *testing.T) {
			lit := byte(rand.Int32N(256))
			dst := make([]byte, s)
			dstFB := make([]byte, s)

			BroadcastU8(dst, lit)
			broadcastU8FB(dstFB, lit)

			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: got %d, expected %d", i, dst[i], dstFB[i])
				}
			}
		})
	}
}

func TestBroadcastI64(t *testing.T) {
	for _, s := range testLens4Broadcast {
		t.Run(fmt.Sprintf("Size %d", s), func(t *testing.T) {
			lit := rand.Int64()
			if time.Now().Second()%2 == 0 {
				lit *= -1
			}
			dst := make([]int64, s)
			dstFB := make([]int64, s)

			BroadcastI64(dst, lit)
			broadcastI64FB(dstFB, lit)

			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: got %d, expected %d", i, dst[i], dstFB[i])
				}
			}
		})
	}
}

func TestBroadcastI32(t *testing.T) {
	for _, s := range testLens4Broadcast {
		t.Run(fmt.Sprintf("Size %d", s), func(t *testing.T) {
			lit := rand.Int32()
			if time.Now().Second()%2 == 0 {
				lit *= -1
			}
			dst := make([]int32, s)
			dstFB := make([]int32, s)

			BroadcastI32(dst, lit)
			broadcastI32FB(dstFB, lit)

			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: got %d, expected %d", i, dst[i], dstFB[i])
				}
			}
		})
	}
}

func TestBroadcastF64(t *testing.T) {
	for _, s := range testLens4Broadcast {
		t.Run(fmt.Sprintf("Size %d", s), func(t *testing.T) {
			lit := rand.Float64() * 100_000
			if time.Now().Second()%2 == 0 {
				lit *= -1
			}
			dst := make([]float64, s)
			dstFB := make([]float64, s)

			BroadcastF64(dst, lit)
			broadcastF64FB(dstFB, lit)

			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
				}
			}
		})
	}

	for _, x := range testFloatConsts {
		t.Run(fmt.Sprintf("X = %.02f", x), func(t *testing.T) {
			length := 5096
			dst := make([]float64, length)
			dstFB := make([]float64, length)

			BroadcastF64(dst, x)
			broadcastF64FB(dstFB, x)
			for i := 0; i < length; i++ {
				// handle NaN
				if math.Float64bits(dst[i]) != math.Float64bits(dstFB[i]) {
					t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
				}
			}
		})
	}
}

func TestBroadcastF32(t *testing.T) {
	for _, s := range testLens4Broadcast {
		t.Run(fmt.Sprintf("Size %d", s), func(t *testing.T) {
			lit := float32(rand.Float64()) * 100_000
			if time.Now().Second()%2 == 0 {
				lit *= -1
			}
			dst := make([]float32, s)
			dstFB := make([]float32, s)

			BroadcastF32(dst, lit)
			broadcastF32FB(dstFB, lit)

			for i := 0; i < s; i++ {
				if dst[i] != dstFB[i] {
					t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
				}
			}
		})
	}

	for _, x := range testFloatConsts {
		t.Run(fmt.Sprintf("X = %.02f", x), func(t *testing.T) {
			length := 5096
			dst := make([]float32, length)
			dstFB := make([]float32, length)

			BroadcastF32(dst, float32(x))
			broadcastF32FB(dstFB, float32(x))
			for i := 0; i < length; i++ {
				if dst[i] != dstFB[i] {
					// handle NaN
					if math.Float32bits(dst[i]) != math.Float32bits(dstFB[i]) {
						t.Errorf("On %d: got %.02f, expected %.02f", i, dst[i], dstFB[i])
					}
				}
			}
		})
	}
}

var bhU8 []byte
var bhI64 []int64
var bhI32 []int32
var bhF64 []float64
var bhF32 []float32

func BenchmarkBroadcastU8(b *testing.B) {
	// assembly
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			lit := byte(rand.Uint32N(256))
			dst := make([]byte, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				BroadcastU8(dst, lit)
				bhU8 = dst
			}
		})
	}
	// fallback
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			lit := byte(rand.Uint32N(256))
			dst := make([]byte, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				broadcastU8FB(dst, lit)
				bhU8 = dst
			}
		})
	}
}

func BenchmarkBroadcastI64(b *testing.B) {
	// assembly
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			lit := rand.Int64()
			dst := make([]int64, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				BroadcastI64(dst, lit)
				bhI64 = dst
			}
		})
	}
	// fallback
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			lit := rand.Int64()
			dst := make([]int64, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				broadcastI64FB(dst, lit)
				bhI64 = dst
			}
		})
	}
}

func BenchmarkBroadcastI32(b *testing.B) {
	// assembly
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			lit := rand.Int32()
			dst := make([]int32, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				BroadcastI32(dst, lit)
				bhI32 = dst
			}
		})
	}
	// fallback
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			lit := rand.Int32()
			dst := make([]int32, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				broadcastI32FB(dst, lit)
				bhI32 = dst
			}
		})
	}
}

func BenchmarkBroadcastF64(b *testing.B) {
	// assembly
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			lit := rand.Float64()
			dst := make([]float64, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				BroadcastF64(dst, lit)
				bhF64 = dst
			}
		})
	}
	// fallback
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			lit := rand.Float64()
			dst := make([]float64, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				broadcastF64FB(dst, lit)
				bhF64 = dst
			}
		})
	}
}

func BenchmarkBroadcastF32(b *testing.B) {
	// assembly
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("ASM Size %d", s), func(b *testing.B) {
			lit := rand.Float32()
			dst := make([]float32, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				BroadcastF32(dst, lit)
				bhF32 = dst
			}
		})
	}
	// fallback
	for _, s := range benchLens4Broadcast {
		b.Run(fmt.Sprintf("FB Size %d", s), func(b *testing.B) {
			lit := rand.Float32()
			dst := make([]float32, s)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				broadcastF32FB(dst, lit)
				bhF32 = dst
			}
		})
	}
}
