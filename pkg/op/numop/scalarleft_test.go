package numop

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestIntegerScalarLeftRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17, 63, 257} {
		src64 := make([]int64, n)
		src32 := make([]int32, n)
		for i := range src64 {
			src64[i] = int64(rng.Uint64())
			src32[i] = int32(rng.Uint32())
		}
		if n > 0 {
			src64[0], src32[0] = 0, 0
		}

		wantSub64 := make([]int64, n)
		wantDiv64 := make([]float64, n)
		for i, value := range src64 {
			wantSub64[i] = math.MinInt64 - value
			wantDiv64[i] = -7.5 / float64(value)
		}
		dstSub64 := append(make([]int64, n), 99)
		dstDiv64 := append(make([]float64, n), 99)
		SubI64LitLeft(src64, dstSub64, math.MinInt64)
		DivI64LitLeft(src64, dstDiv64, -7.5)
		checkI64Result(t, dstSub64, wantSub64)
		checkF64Result(t, dstDiv64, wantDiv64)
		alias64 := append([]int64(nil), src64...)
		SubI64LitLeft(alias64, alias64, math.MinInt64)
		if !slices.Equal(alias64, wantSub64) {
			t.Fatalf("SubI64LitLeft alias length %d: got %v, want %v", n, alias64, wantSub64)
		}

		wantSub32 := make([]int32, n)
		wantDiv32 := make([]float32, n)
		for i, value := range src32 {
			wantSub32[i] = math.MinInt32 - value
			wantDiv32[i] = -7.5 / float32(value)
		}
		dstSub32 := append(make([]int32, n), 99)
		dstDiv32 := append(make([]float32, n), 99)
		SubI32LitLeft(src32, dstSub32, math.MinInt32)
		DivI32LitLeft(src32, dstDiv32, -7.5)
		checkI32Result(t, dstSub32, wantSub32)
		checkF32Result(t, dstDiv32, wantDiv32)
		alias32 := append([]int32(nil), src32...)
		SubI32LitLeft(alias32, alias32, math.MinInt32)
		if !slices.Equal(alias32, wantSub32) {
			t.Fatalf("SubI32LitLeft alias length %d: got %v, want %v", n, alias32, wantSub32)
		}
	}
}

func TestFloatScalarLeftRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 14))
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17, 63, 257} {
		src64 := make([]float64, n)
		src32 := make([]float32, n)
		for i := range src64 {
			src64[i] = math.Float64frombits(rng.Uint64())
			src32[i] = math.Float32frombits(rng.Uint32())
		}
		if n > 4 {
			src64[0], src64[1], src64[2], src64[3], src64[4] = 0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN()
			src32[0], src32[1], src32[2], src32[3], src32[4] = 0, float32(math.Copysign(0, -1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())
		}
		testF64ScalarLeft(t, src64, "Sub", SubF64LitLeft, func(value float64) float64 { return math.Copysign(0, -1) - value })
		testF64ScalarLeft(t, src64, "Div", DivF64LitLeft, func(value float64) float64 { return math.Copysign(0, -1) / value })
		testF32ScalarLeft(t, src32, "Sub", SubF32LitLeft, func(value float32) float32 { return float32(math.Copysign(0, -1)) - value })
		testF32ScalarLeft(t, src32, "Div", DivF32LitLeft, func(value float32) float32 { return float32(math.Copysign(0, -1)) / value })
	}
}

func testF64ScalarLeft(t *testing.T, src []float64, name string, fn func([]float64, []float64, float64), wantFn func(float64) float64) {
	t.Helper()
	want := make([]float64, len(src))
	for i, value := range src {
		want[i] = wantFn(value)
	}
	dst := append(make([]float64, len(src)), 99)
	fn(src, dst, math.Copysign(0, -1))
	checkF64Result(t, dst, want)
	alias := append([]float64(nil), src...)
	fn(alias, alias, math.Copysign(0, -1))
	if !equalF64Slices(alias, want) {
		t.Fatalf("F64 %s alias: got %v, want %v", name, alias, want)
	}
}

func testF32ScalarLeft(t *testing.T, src []float32, name string, fn func([]float32, []float32, float32), wantFn func(float32) float32) {
	t.Helper()
	want := make([]float32, len(src))
	for i, value := range src {
		want[i] = wantFn(value)
	}
	dst := append(make([]float32, len(src)), 99)
	fn(src, dst, float32(math.Copysign(0, -1)))
	checkF32Result(t, dst, want)
	alias := append([]float32(nil), src...)
	fn(alias, alias, float32(math.Copysign(0, -1)))
	if !equalF32Slices(alias, want) {
		t.Fatalf("F32 %s alias: got %v, want %v", name, alias, want)
	}
}

func checkI64Result(t *testing.T, got, want []int64) {
	t.Helper()
	if !slices.Equal(got[:len(want)], want) || got[len(want)] != 99 {
		t.Fatalf("got %v, want %v with sentinel", got, want)
	}
}

func checkI32Result(t *testing.T, got, want []int32) {
	t.Helper()
	if !slices.Equal(got[:len(want)], want) || got[len(want)] != 99 {
		t.Fatalf("got %v, want %v with sentinel", got, want)
	}
}

func checkF64Result(t *testing.T, got, want []float64) {
	t.Helper()
	if !equalF64Slices(got[:len(want)], want) || got[len(want)] != 99 {
		t.Fatalf("got %v, want %v with sentinel", got, want)
	}
}

func checkF32Result(t *testing.T, got, want []float32) {
	t.Helper()
	if !equalF32Slices(got[:len(want)], want) || got[len(want)] != 99 {
		t.Fatalf("got %v, want %v with sentinel", got, want)
	}
}

func equalF64Slices(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Float64bits(a[i]) != math.Float64bits(b[i]) {
			return false
		}
	}
	return true
}

func equalF32Slices(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) {
			return false
		}
	}
	return true
}
