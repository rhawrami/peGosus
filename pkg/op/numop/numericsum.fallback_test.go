package numop

import (
	"math"
	"testing"
)

func TestNumericSumFallback(t *testing.T) {
	dstI64 := make([]int64, 1)
	sumI64Fallback([]int64{math.MaxInt64, 1}, dstI64)
	if dstI64[0] != math.MinInt64 {
		t.Fatalf("SumI64 overflow: got %d", dstI64[0])
	}

	sumI32Fallback([]int32{math.MaxInt32, math.MaxInt32, math.MinInt32}, dstI64)
	if dstI64[0] != int64(math.MaxInt32-1) {
		t.Fatalf("SumI32: got %d", dstI64[0])
	}

	dstF64 := []float64{99}
	sumF64Fallback([]float64{math.NaN(), math.Inf(1), 2}, dstF64)
	if !math.IsInf(dstF64[0], 1) {
		t.Fatalf("SumF64: got %v", dstF64[0])
	}

	sumF32Fallback([]float32{float32(math.NaN()), 0.1, 0.2}, dstF64)
	if expected := float64(float32(0.1)) + float64(float32(0.2)); dstF64[0] != expected {
		t.Fatalf("SumF32: got %.17g, expected %.17g", dstF64[0], expected)
	}
}

func TestNumericSumFallbackValidity(t *testing.T) {
	validity := []byte{0b00000001, 0b00000010}
	dstI64 := []int64{99}

	sumI64WithValidityFallback([]int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, dstI64, validity)
	if dstI64[0] != 11 {
		t.Fatalf("SumI64WithValidity: got %d", dstI64[0])
	}

	sumI32WithValidityFallback([]int32{math.MaxInt32, 2, 3, 4, 5, 6, 7, 8, 9, math.MaxInt32}, dstI64, validity)
	if dstI64[0] != 2*int64(math.MaxInt32) {
		t.Fatalf("SumI32WithValidity: got %d", dstI64[0])
	}

	dstF64 := []float64{99}
	sumF64WithValidityFallback([]float64{math.NaN(), 2, 3, 4, 5, 6, 7, 8, math.Inf(-1), 10}, dstF64, validity)
	if dstF64[0] != 10 {
		t.Fatalf("SumF64WithValidity: got %v", dstF64[0])
	}

	sumF32WithValidityFallback([]float32{1.5, 2, 3, 4, 5, 6, 7, 8, 9, 2.5}, dstF64, validity)
	if dstF64[0] != 4 {
		t.Fatalf("SumF32WithValidity: got %v", dstF64[0])
	}

	allInvalid := []byte{0, 0}
	sumI64WithValidityFallback(make([]int64, 10), dstI64, allInvalid)
	if dstI64[0] != 0 {
		t.Fatalf("all-invalid SumI64WithValidity: got %d", dstI64[0])
	}
}
