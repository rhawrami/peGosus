package numop

import (
	"slices"
	"testing"
)

func TestFloatBinaryLitFallback(t *testing.T) {
	f64 := []struct {
		name string
		fn   func([]float64, []float64, float64)
		want func(float64, float64) float64
	}{
		{"add", addF64LitFallback, func(a, b float64) float64 { return a + b }},
		{"sub", subF64LitFallback, func(a, b float64) float64 { return a - b }},
		{"mul", mulF64LitFallback, func(a, b float64) float64 { return a * b }},
		{"div", divF64LitFallback, func(a, b float64) float64 { return a / b }},
	}
	f32 := []struct {
		name string
		fn   func([]float32, []float32, float32)
		want func(float32, float32) float32
	}{
		{"add", addF32LitFallback, func(a, b float32) float32 { return a + b }},
		{"sub", subF32LitFallback, func(a, b float32) float32 { return a - b }},
		{"mul", mulF32LitFallback, func(a, b float32) float32 { return a * b }},
		{"div", divF32LitFallback, func(a, b float32) float32 { return a / b }},
	}

	for _, n := range []int{0, 1, 3, 4, 7, 8, 9} {
		for _, tc := range f64 {
			src := []float64{-8, -4, -2, -1, 1, 2, 4, 8, 16}[:n]
			dst := make([]float64, n+1)
			dst[n] = 99
			tc.fn(src, dst, 2)
			want := make([]float64, n+1)
			for i := range src {
				want[i] = tc.want(src[i], 2)
			}
			want[n] = 99
			if !slices.Equal(dst, want) {
				t.Fatalf("f64 %s length %d: got %v, want %v", tc.name, n, dst, want)
			}

			alias := append([]float64(nil), src...)
			tc.fn(alias, alias, 2)
			if !slices.Equal(alias, want[:n]) {
				t.Fatalf("f64 %s alias: got %v, want %v", tc.name, alias, want[:n])
			}
		}

		for _, tc := range f32 {
			src := []float32{-8, -4, -2, -1, 1, 2, 4, 8, 16}[:n]
			dst := make([]float32, n+1)
			dst[n] = 99
			tc.fn(src, dst, 2)
			want := make([]float32, n+1)
			for i := range src {
				want[i] = tc.want(src[i], 2)
			}
			want[n] = 99
			if !slices.Equal(dst, want) {
				t.Fatalf("f32 %s length %d: got %v, want %v", tc.name, n, dst, want)
			}

			alias := append([]float32(nil), src...)
			tc.fn(alias, alias, 2)
			if !slices.Equal(alias, want[:n]) {
				t.Fatalf("f32 %s alias: got %v, want %v", tc.name, alias, want[:n])
			}
		}
	}
}

func TestFloatBinaryVecFallback(t *testing.T) {
	f64 := []struct {
		name string
		fn   func([]float64, []float64, []float64)
		want func(float64, float64) float64
	}{
		{"add", addF64VecFallback, func(a, b float64) float64 { return a + b }},
		{"sub", subF64VecFallback, func(a, b float64) float64 { return a - b }},
		{"mul", mulF64VecFallback, func(a, b float64) float64 { return a * b }},
		{"div", divF64VecFallback, func(a, b float64) float64 { return a / b }},
	}
	f32 := []struct {
		name string
		fn   func([]float32, []float32, []float32)
		want func(float32, float32) float32
	}{
		{"add", addF32VecFallback, func(a, b float32) float32 { return a + b }},
		{"sub", subF32VecFallback, func(a, b float32) float32 { return a - b }},
		{"mul", mulF32VecFallback, func(a, b float32) float32 { return a * b }},
		{"div", divF32VecFallback, func(a, b float32) float32 { return a / b }},
	}

	for _, n := range []int{0, 1, 3, 4, 7, 8, 9} {
		for _, tc := range f64 {
			src1 := []float64{-8, -4, -2, -1, 1, 2, 4, 8, 16}[:n]
			src2 := []float64{2, -2, 4, -4, 8, -8, 16, -16, 32}[:n]
			want := make([]float64, n)
			for i := range src1 {
				want[i] = tc.want(src1[i], src2[i])
			}
			dst := append(make([]float64, n), 99)
			tc.fn(src1, src2, dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("f64 %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]float64(nil), src1...)
			tc.fn(alias, src2, alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("f64 %s alias: got %v, want %v", tc.name, alias, want)
			}
		}

		for _, tc := range f32 {
			src1 := []float32{-8, -4, -2, -1, 1, 2, 4, 8, 16}[:n]
			src2 := []float32{2, -2, 4, -4, 8, -8, 16, -16, 32}[:n]
			want := make([]float32, n)
			for i := range src1 {
				want[i] = tc.want(src1[i], src2[i])
			}
			dst := append(make([]float32, n), 99)
			tc.fn(src1, src2, dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("f32 %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]float32(nil), src1...)
			tc.fn(alias, src2, alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("f32 %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
	}
}
