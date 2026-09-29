package numop

import (
	"math"
	"testing"
)

func TestClipSemanticEdgeCases(t *testing.T) {
	for _, n := range []int{0, 1, 3, 4, 7, 9, 16, 17, 31, 32, 33, 65, 73, 127} {
		src32, src64 := make([]float32, n), make([]float64, n)
		ints32, ints64 := make([]int32, n), make([]int64, n)
		for i := range n {
			value := float64(i%13-6) / 2
			switch i % 17 {
			case 0:
				value = math.Copysign(0, -1)
			case 5:
				value = math.Inf(1)
			case 7:
				value = math.Inf(-1)
			case 11:
				value = math.Float64frombits(0x7ff8000000000031)
			}
			src32[i], src64[i] = float32(value), value
			ints32[i] = int32(i*100003 - 3000000)
			ints64[i] = int64(i)*1000000000000003 - 3000000000000000
		}
		for _, bounds := range [][2]float64{
			{-5, 5}, {5, -5}, {0, 5}, {math.Copysign(0, -1), 0},
			{math.Inf(-1), math.Inf(1)}, {math.NaN(), 5}, {-5, math.NaN()},
		} {
			lower, upper := bounds[0], bounds[1]
			want64, got64 := make([]float64, n), make([]float64, n)
			clipF64WithF64BoundsFallback(src64, want64, lower, upper)
			ClipF64WithF64Bounds(src64, got64, lower, upper)
			for i := range n {
				if math.Float64bits(got64[i]) != math.Float64bits(want64[i]) {
					t.Fatalf("F64 len %d bounds %v/%v row %d: got %v want %v", n, lower, upper, i, got64[i], want64[i])
				}
			}
			alias64 := append([]float64(nil), src64...)
			ClipF64WithF64Bounds(alias64, alias64, lower, upper)
			for i := range n {
				if math.Float64bits(alias64[i]) != math.Float64bits(want64[i]) {
					t.Fatalf("F64 alias len %d bounds %v/%v row %d", n, lower, upper, i)
				}
			}
			want32, got32 := make([]float32, n), make([]float32, n)
			clipF32WithF32BoundsFallback(src32, want32, float32(lower), float32(upper))
			ClipF32WithF32Bounds(src32, got32, float32(lower), float32(upper))
			for i := range n {
				if math.Float32bits(got32[i]) != math.Float32bits(want32[i]) {
					t.Fatalf("F32 len %d bounds %v/%v row %d: got %v want %v", n, lower, upper, i, got32[i], want32[i])
				}
			}
			alias32 := append([]float32(nil), src32...)
			ClipF32WithF32Bounds(alias32, alias32, float32(lower), float32(upper))
			for i := range n {
				if math.Float32bits(alias32[i]) != math.Float32bits(want32[i]) {
					t.Fatalf("F32 alias len %d bounds %v/%v row %d", n, lower, upper, i)
				}
			}
			clipI64WithF64BoundsFallback(ints64, want64, lower, upper)
			ClipI64WithF64Bounds(ints64, got64, lower, upper)
			for i := range n {
				if math.Float64bits(got64[i]) != math.Float64bits(want64[i]) {
					t.Fatalf("I64/F64 len %d bounds %v/%v row %d: got %v want %v", n, lower, upper, i, got64[i], want64[i])
				}
			}
			clipI32WithF32BoundsFallback(ints32, want32, float32(lower), float32(upper))
			ClipI32WithF32Bounds(ints32, got32, float32(lower), float32(upper))
			for i := range n {
				if math.Float32bits(got32[i]) != math.Float32bits(want32[i]) {
					t.Fatalf("I32/F32 len %d bounds %v/%v row %d: got %v want %v", n, lower, upper, i, got32[i], want32[i])
				}
			}
		}
		for _, bounds := range [][2]int64{{-5, 5}, {5, -5}, {math.MinInt64, math.MaxInt64}} {
			want64, got64 := make([]int64, n), make([]int64, n)
			clipI64WithI64BoundsFallback(ints64, want64, bounds[0], bounds[1])
			ClipI64WithI64Bounds(ints64, got64, bounds[0], bounds[1])
			for i := range n {
				if got64[i] != want64[i] {
					t.Fatalf("I64 len %d bounds %v row %d: got %d want %d", n, bounds, i, got64[i], want64[i])
				}
			}
			want32, got32 := make([]int32, n), make([]int32, n)
			clipI32WithI32BoundsFallback(ints32, want32, int32(bounds[0]), int32(bounds[1]))
			ClipI32WithI32Bounds(ints32, got32, int32(bounds[0]), int32(bounds[1]))
			for i := range n {
				if got32[i] != want32[i] {
					t.Fatalf("I32 len %d bounds %v row %d: got %d want %d", n, bounds, i, got32[i], want32[i])
				}
			}
		}
	}
}
