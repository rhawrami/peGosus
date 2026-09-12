package numop

import (
	"math"
	"slices"
	"testing"
)

func TestFloatUnaryFallback(t *testing.T) {
	f64 := []struct {
		name string
		fn   func([]float64, []float64)
		want func(float64) float64
	}{
		{"sqrt", sqrtF64Fallback, math.Sqrt},
		{"sq", sqF64Fallback, func(v float64) float64 { return v * v }},
		{"abs", absF64Fallback, math.Abs},
		{"neg", negF64Fallback, func(v float64) float64 { return -v }},
		{"recip", recipF64Fallback, func(v float64) float64 { return 1 / v }},
	}
	f32 := []struct {
		name string
		fn   func([]float32, []float32)
		want func(float32) float32
	}{
		{"sqrt", sqrtF32Fallback, func(v float32) float32 { return float32(math.Sqrt(float64(v))) }},
		{"sq", sqF32Fallback, func(v float32) float32 { return v * v }},
		{"abs", absF32Fallback, func(v float32) float32 { return math.Float32frombits(math.Float32bits(v) &^ (1 << 31)) }},
		{"neg", negF32Fallback, func(v float32) float32 { return -v }},
		{"recip", recipF32Fallback, func(v float32) float32 { return 1 / v }},
	}

	for _, n := range []int{0, 1, 3, 4, 7, 8, 9} {
		for _, tc := range f64 {
			src := []float64{1, 4, 9, 16, 25, 36, 49, 64, 81}[:n]
			want := make([]float64, n)
			for i := range src {
				want[i] = tc.want(src[i])
			}
			dst := append(make([]float64, n), 99)
			tc.fn(src, dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("f64 %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]float64(nil), src...)
			tc.fn(alias, alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("f64 %s alias: got %v, want %v", tc.name, alias, want)
			}
		}

		for _, tc := range f32 {
			src := []float32{1, 4, 9, 16, 25, 36, 49, 64, 81}[:n]
			want := make([]float32, n)
			for i := range src {
				want[i] = tc.want(src[i])
			}
			dst := append(make([]float32, n), 99)
			tc.fn(src, dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("f32 %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]float32(nil), src...)
			tc.fn(alias, alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("f32 %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
	}
}

func TestFloatUnaryFallbackSpecialValues(t *testing.T) {
	nan64 := math.Float64frombits(0xfff8000000001234)
	src64 := []float64{nan64, math.Inf(-1), math.Copysign(0, -1), 0}
	dst64 := make([]float64, len(src64))
	absF64Fallback(src64, dst64)
	wantAbs64 := []uint64{0x7ff8000000001234, 0x7ff0000000000000, 0, 0}
	for i := range dst64 {
		if math.Float64bits(dst64[i]) != wantAbs64[i] {
			t.Fatalf("abs f64 index %d: got %#x, want %#x", i, math.Float64bits(dst64[i]), wantAbs64[i])
		}
	}
	negF64Fallback(src64, dst64)
	for i := range dst64 {
		want := math.Float64bits(src64[i]) ^ (1 << 63)
		if math.Float64bits(dst64[i]) != want {
			t.Fatalf("neg f64 index %d: got %#x, want %#x", i, math.Float64bits(dst64[i]), want)
		}
	}
	recipF64Fallback(src64[2:], dst64[:2])
	if !math.IsInf(dst64[0], -1) || !math.IsInf(dst64[1], 1) {
		t.Fatalf("recip f64 signed zero: got %v", dst64[:2])
	}
	sqrtF64Fallback([]float64{math.Inf(1), -1, math.Copysign(0, -1)}, dst64[:3])
	if !math.IsInf(dst64[0], 1) || !math.IsNaN(dst64[1]) || math.Float64bits(dst64[2]) != 1<<63 {
		t.Fatalf("sqrt f64 specials: got %v", dst64[:3])
	}
	sqF64Fallback(src64, dst64)
	if !math.IsNaN(dst64[0]) || !math.IsInf(dst64[1], 1) || math.Float64bits(dst64[2]) != 0 {
		t.Fatalf("sq f64 specials: got %v", dst64)
	}

	nan32 := math.Float32frombits(0xffc01234)
	src32 := []float32{nan32, float32(math.Inf(-1)), math.Float32frombits(1 << 31), 0}
	dst32 := make([]float32, len(src32))
	absF32Fallback(src32, dst32)
	wantAbs32 := []uint32{0x7fc01234, 0x7f800000, 0, 0}
	for i := range dst32 {
		if math.Float32bits(dst32[i]) != wantAbs32[i] {
			t.Fatalf("abs f32 index %d: got %#x, want %#x", i, math.Float32bits(dst32[i]), wantAbs32[i])
		}
	}
	negF32Fallback(src32, dst32)
	for i := range dst32 {
		want := math.Float32bits(src32[i]) ^ (1 << 31)
		if math.Float32bits(dst32[i]) != want {
			t.Fatalf("neg f32 index %d: got %#x, want %#x", i, math.Float32bits(dst32[i]), want)
		}
	}
	recipF32Fallback(src32[2:], dst32[:2])
	if !math.IsInf(float64(dst32[0]), -1) || !math.IsInf(float64(dst32[1]), 1) {
		t.Fatalf("recip f32 signed zero: got %v", dst32[:2])
	}
	sqrtF32Fallback([]float32{float32(math.Inf(1)), -1, math.Float32frombits(1 << 31)}, dst32[:3])
	if !math.IsInf(float64(dst32[0]), 1) || !math.IsNaN(float64(dst32[1])) || math.Float32bits(dst32[2]) != 1<<31 {
		t.Fatalf("sqrt f32 specials: got %v", dst32[:3])
	}
	sqF32Fallback(src32, dst32)
	if !math.IsNaN(float64(dst32[0])) || !math.IsInf(float64(dst32[1]), 1) || math.Float32bits(dst32[2]) != 0 {
		t.Fatalf("sq f32 specials: got %v", dst32)
	}
}
