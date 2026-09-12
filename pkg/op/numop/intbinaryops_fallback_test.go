package numop

import (
	"math"
	"slices"
	"testing"
)

func TestIntegerBinaryFallback(t *testing.T) {
	i64Lit := []struct {
		name string
		fn   func([]int64, []int64, int64)
		want func(int64, int64) int64
	}{
		{"add", addI64LitFallback, func(a, b int64) int64 { return a + b }},
		{"sub", subI64LitFallback, func(a, b int64) int64 { return a - b }},
		{"mul", mulI64LitFallback, func(a, b int64) int64 { return a * b }},
	}
	i32Lit := []struct {
		name string
		fn   func([]int32, []int32, int32)
		want func(int32, int32) int32
	}{
		{"add", addI32LitFallback, func(a, b int32) int32 { return a + b }},
		{"sub", subI32LitFallback, func(a, b int32) int32 { return a - b }},
		{"mul", mulI32LitFallback, func(a, b int32) int32 { return a * b }},
	}
	i64Vec := []struct {
		name string
		fn   func([]int64, []int64, []int64)
		want func(int64, int64) int64
	}{
		{"add", addI64VecFallback, func(a, b int64) int64 { return a + b }},
		{"sub", subI64VecFallback, func(a, b int64) int64 { return a - b }},
		{"mul", mulI64VecFallback, func(a, b int64) int64 { return a * b }},
	}
	i32Vec := []struct {
		name string
		fn   func([]int32, []int32, []int32)
		want func(int32, int32) int32
	}{
		{"add", addI32VecFallback, func(a, b int32) int32 { return a + b }},
		{"sub", subI32VecFallback, func(a, b int32) int32 { return a - b }},
		{"mul", mulI32VecFallback, func(a, b int32) int32 { return a * b }},
	}

	base64 := []int64{math.MinInt64, math.MaxInt64, -8, -1, 0, 1, 2, 4, 8}
	other64 := []int64{-1, 1, 2, -2, 3, -3, 4, -4, 5}
	base32 := []int32{math.MinInt32, math.MaxInt32, -8, -1, 0, 1, 2, 4, 8}
	other32 := []int32{-1, 1, 2, -2, 3, -3, 4, -4, 5}
	for _, n := range []int{0, 1, 3, 4, 7, 8, 9} {
		for _, tc := range i64Lit {
			want := make([]int64, n)
			for i, v := range base64[:n] {
				want[i] = tc.want(v, 3)
			}
			dst := append(make([]int64, n), 99)
			tc.fn(base64[:n], dst, 3)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("i64 lit %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]int64(nil), base64[:n]...)
			tc.fn(alias, alias, 3)
			if !slices.Equal(alias, want) {
				t.Fatalf("i64 lit %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
		for _, tc := range i32Lit {
			want := make([]int32, n)
			for i, v := range base32[:n] {
				want[i] = tc.want(v, 3)
			}
			dst := append(make([]int32, n), 99)
			tc.fn(base32[:n], dst, 3)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("i32 lit %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]int32(nil), base32[:n]...)
			tc.fn(alias, alias, 3)
			if !slices.Equal(alias, want) {
				t.Fatalf("i32 lit %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
		for _, tc := range i64Vec {
			want := make([]int64, n)
			for i, v := range base64[:n] {
				want[i] = tc.want(v, other64[i])
			}
			dst := append(make([]int64, n), 99)
			tc.fn(base64[:n], other64[:n], dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("i64 vec %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]int64(nil), base64[:n]...)
			tc.fn(alias, other64[:n], alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("i64 vec %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
		for _, tc := range i32Vec {
			want := make([]int32, n)
			for i, v := range base32[:n] {
				want[i] = tc.want(v, other32[i])
			}
			dst := append(make([]int32, n), 99)
			tc.fn(base32[:n], other32[:n], dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("i32 vec %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]int32(nil), base32[:n]...)
			tc.fn(alias, other32[:n], alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("i32 vec %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
	}
}

func TestIntegerDivisionFallback(t *testing.T) {
	src64 := []int64{math.MinInt64, -9, 0, 9, math.MaxInt64, -4, 4, -8, 8}
	div64 := []int64{-2, 3, 1, -3, 2, 2, -2, 4, -4}
	src32 := []int32{math.MinInt32, -9, 0, 9, math.MaxInt32, -4, 4, -8, 8}
	div32 := []int32{-2, 3, 1, -3, 2, 2, -2, 4, -4}
	for _, n := range []int{0, 1, 3, 4, 7, 8, 9} {
		dst64 := append(make([]float64, n), 99)
		divI64LitFallback(src64[:n], dst64, 2)
		for i, v := range src64[:n] {
			if dst64[i] != float64(v)/2 {
				t.Fatalf("i64 lit length %d index %d: got %v", n, i, dst64[i])
			}
		}
		if dst64[n] != 99 {
			t.Fatalf("i64 lit length %d overwrote sentinel", n)
		}
		divI64VecFallback(src64[:n], div64[:n], dst64)
		for i, v := range src64[:n] {
			want := float64(v) / float64(div64[i])
			if dst64[i] != want {
				t.Fatalf("i64 vec length %d index %d: got %v, want %v", n, i, dst64[i], want)
			}
		}

		dst32 := append(make([]float32, n), 99)
		divI32LitFallback(src32[:n], dst32, 2)
		for i, v := range src32[:n] {
			if dst32[i] != float32(v)/2 {
				t.Fatalf("i32 lit length %d index %d: got %v", n, i, dst32[i])
			}
		}
		if dst32[n] != 99 {
			t.Fatalf("i32 lit length %d overwrote sentinel", n)
		}
		divI32VecFallback(src32[:n], div32[:n], dst32)
		for i, v := range src32[:n] {
			want := float32(v) / float32(div32[i])
			if dst32[i] != want {
				t.Fatalf("i32 vec length %d index %d: got %v, want %v", n, i, dst32[i], want)
			}
		}
	}
}
