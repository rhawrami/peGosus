package numop

import (
	"math"
	"slices"
	"testing"
)

func TestIntegerUnarySameTypeFallback(t *testing.T) {
	i64 := []struct {
		name string
		fn   func([]int64, []int64)
		want func(int64) int64
	}{
		{"sq", sqI64Fallback, func(v int64) int64 { return v * v }},
		{"abs", absI64Fallback, func(v int64) int64 {
			if v < 0 {
				return -v
			}
			return v
		}},
		{"neg", negI64Fallback, func(v int64) int64 { return -v }},
	}
	i32 := []struct {
		name string
		fn   func([]int32, []int32)
		want func(int32) int32
	}{
		{"sq", sqI32Fallback, func(v int32) int32 { return v * v }},
		{"abs", absI32Fallback, func(v int32) int32 {
			if v < 0 {
				return -v
			}
			return v
		}},
		{"neg", negI32Fallback, func(v int32) int32 { return -v }},
	}
	src64 := []int64{math.MinInt64, math.MaxInt64, -9, -1, 0, 1, 2, 3, 9}
	src32 := []int32{math.MinInt32, math.MaxInt32, -9, -1, 0, 1, 2, 3, 9}

	for _, n := range []int{0, 1, 3, 4, 7, 8, 9} {
		for _, tc := range i64 {
			want := make([]int64, n)
			for i, v := range src64[:n] {
				want[i] = tc.want(v)
			}
			dst := append(make([]int64, n), 99)
			tc.fn(src64[:n], dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("i64 %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]int64(nil), src64[:n]...)
			tc.fn(alias, alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("i64 %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
		for _, tc := range i32 {
			want := make([]int32, n)
			for i, v := range src32[:n] {
				want[i] = tc.want(v)
			}
			dst := append(make([]int32, n), 99)
			tc.fn(src32[:n], dst)
			if !slices.Equal(dst, append(want, 99)) {
				t.Fatalf("i32 %s length %d: got %v", tc.name, n, dst)
			}
			alias := append([]int32(nil), src32[:n]...)
			tc.fn(alias, alias)
			if !slices.Equal(alias, want) {
				t.Fatalf("i32 %s alias: got %v, want %v", tc.name, alias, want)
			}
		}
	}
}

func TestIntegerUnaryFloatFallback(t *testing.T) {
	src64 := []int64{-1, 0, 1, 4, 9, math.MaxInt64, 16, 25, 36}
	src32 := []int32{-1, 0, 1, 4, 9, math.MaxInt32, 16, 25, 36}
	for _, n := range []int{0, 1, 3, 4, 7, 8, 9} {
		sqrt64 := append(make([]float64, n), 99)
		recip64 := append(make([]float64, n), 99)
		sqrtI64Fallback(src64[:n], sqrt64)
		recipI64Fallback(src64[:n], recip64)
		for i, v := range src64[:n] {
			wantSqrt := math.Sqrt(float64(v))
			if !(math.IsNaN(sqrt64[i]) && math.IsNaN(wantSqrt)) && sqrt64[i] != wantSqrt {
				t.Fatalf("sqrt i64 length %d index %d: got %v, want %v", n, i, sqrt64[i], wantSqrt)
			}
			wantRecip := 1 / float64(v)
			if recip64[i] != wantRecip {
				t.Fatalf("recip i64 length %d index %d: got %v, want %v", n, i, recip64[i], wantRecip)
			}
		}
		if sqrt64[n] != 99 || recip64[n] != 99 {
			t.Fatalf("i64 length %d overwrote sentinel", n)
		}

		sqrt32 := append(make([]float32, n), 99)
		recip32 := append(make([]float32, n), 99)
		sqrtI32Fallback(src32[:n], sqrt32)
		recipI32Fallback(src32[:n], recip32)
		for i, v := range src32[:n] {
			wantSqrt := float32(math.Sqrt(float64(v)))
			if !(math.IsNaN(float64(sqrt32[i])) && math.IsNaN(float64(wantSqrt))) && sqrt32[i] != wantSqrt {
				t.Fatalf("sqrt i32 length %d index %d: got %v, want %v", n, i, sqrt32[i], wantSqrt)
			}
			wantRecip := 1 / float32(v)
			if recip32[i] != wantRecip {
				t.Fatalf("recip i32 length %d index %d: got %v, want %v", n, i, recip32[i], wantRecip)
			}
		}
		if sqrt32[n] != 99 || recip32[n] != 99 {
			t.Fatalf("i32 length %d overwrote sentinel", n)
		}
	}
}
