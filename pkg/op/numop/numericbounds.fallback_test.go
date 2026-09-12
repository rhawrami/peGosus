package numop

import (
	"math"
	"reflect"
	"testing"
)

func TestNumericBoundsFallback(t *testing.T) {
	i64 := []int64{math.MaxInt64, 0, math.MinInt64}
	dstI64 := make([]int64, 2)
	maxI64Fallback(i64, dstI64[:1])
	if dstI64[0] != math.MaxInt64 {
		t.Fatalf("MaxI64: got %d", dstI64[0])
	}
	minI64Fallback(i64, dstI64[:1])
	if dstI64[0] != math.MinInt64 {
		t.Fatalf("MinI64: got %d", dstI64[0])
	}
	minMaxI64Fallback(i64, dstI64)
	if expected := []int64{math.MinInt64, math.MaxInt64}; !reflect.DeepEqual(dstI64, expected) {
		t.Fatalf("MinMaxI64: got %v, expected %v", dstI64, expected)
	}

	i32 := []int32{math.MaxInt32, 0, math.MinInt32}
	dstI32 := make([]int32, 2)
	maxI32Fallback(i32, dstI32[:1])
	if dstI32[0] != math.MaxInt32 {
		t.Fatalf("MaxI32: got %d", dstI32[0])
	}
	minI32Fallback(i32, dstI32[:1])
	if dstI32[0] != math.MinInt32 {
		t.Fatalf("MinI32: got %d", dstI32[0])
	}
	minMaxI32Fallback(i32, dstI32)
	if expected := []int32{math.MinInt32, math.MaxInt32}; !reflect.DeepEqual(dstI32, expected) {
		t.Fatalf("MinMaxI32: got %v, expected %v", dstI32, expected)
	}

	f64 := []float64{math.NaN(), math.Inf(1), 0, math.Inf(-1)}
	dstF64 := make([]float64, 2)
	maxF64Fallback(f64, dstF64[:1])
	if !math.IsInf(dstF64[0], 1) {
		t.Fatalf("MaxF64: got %v", dstF64[0])
	}
	minF64Fallback(f64, dstF64[:1])
	if !math.IsInf(dstF64[0], -1) {
		t.Fatalf("MinF64: got %v", dstF64[0])
	}
	minMaxF64Fallback(f64, dstF64)
	if !math.IsInf(dstF64[0], -1) || !math.IsInf(dstF64[1], 1) {
		t.Fatalf("MinMaxF64: got %v", dstF64)
	}

	f32 := []float32{float32(math.NaN()), float32(math.Inf(1)), 0, float32(math.Inf(-1))}
	dstF32 := make([]float32, 2)
	maxF32Fallback(f32, dstF32[:1])
	if !math.IsInf(float64(dstF32[0]), 1) {
		t.Fatalf("MaxF32: got %v", dstF32[0])
	}
	minF32Fallback(f32, dstF32[:1])
	if !math.IsInf(float64(dstF32[0]), -1) {
		t.Fatalf("MinF32: got %v", dstF32[0])
	}
	minMaxF32Fallback(f32, dstF32)
	if !math.IsInf(float64(dstF32[0]), -1) || !math.IsInf(float64(dstF32[1]), 1) {
		t.Fatalf("MinMaxF32: got %v", dstF32)
	}
}

func TestNumericBoundsFallbackAllNaN(t *testing.T) {
	dstF64 := make([]float64, 2)
	minMaxF64Fallback([]float64{math.NaN()}, dstF64)
	if !math.IsInf(dstF64[0], 1) || !math.IsInf(dstF64[1], -1) {
		t.Fatalf("all-NaN MinMaxF64: got %v", dstF64)
	}
}

func TestNumericBoundsFallbackValidity(t *testing.T) {
	validity := []byte{0b00000001, 0b00000010}
	invalid := []byte{0, 0}
	i64 := []int64{-10, 100, 100, 100, 100, 100, 100, 100, 100, 20}
	dstI64 := make([]int64, 2)
	maxI64WithValidityFallback(i64, dstI64[:1], validity)
	if dstI64[0] != 20 {
		t.Fatalf("MaxI64WithValidity: got %d", dstI64[0])
	}
	minI64WithValidityFallback(i64, dstI64[:1], validity)
	if dstI64[0] != -10 {
		t.Fatalf("MinI64WithValidity: got %d", dstI64[0])
	}
	minMaxI64WithValidityFallback(i64, dstI64, validity)
	if expected := []int64{-10, 20}; !reflect.DeepEqual(dstI64, expected) {
		t.Fatalf("MinMaxI64WithValidity: got %v, expected %v", dstI64, expected)
	}

	i32 := []int32{-10, 100, 100, 100, 100, 100, 100, 100, 100, 20}
	dstI32 := make([]int32, 2)
	maxI32WithValidityFallback(i32, dstI32[:1], validity)
	minI32WithValidityFallback(i32, dstI32[1:], validity)
	if expected := []int32{20, -10}; !reflect.DeepEqual(dstI32, expected) {
		t.Fatalf("I32 validity bounds: got %v, expected %v", dstI32, expected)
	}
	minMaxI32WithValidityFallback(i32, dstI32, invalid)
	if expected := []int32{math.MaxInt32, math.MinInt32}; !reflect.DeepEqual(dstI32, expected) {
		t.Fatalf("all-invalid MinMaxI32WithValidity: got %v, expected %v", dstI32, expected)
	}

	f64 := []float64{math.NaN(), 100, 100, 100, 100, 100, 100, 100, 100, math.Inf(1)}
	dstF64 := make([]float64, 2)
	maxF64WithValidityFallback(f64, dstF64[:1], validity)
	minF64WithValidityFallback(f64, dstF64[1:], validity)
	if !math.IsInf(dstF64[0], 1) || !math.IsInf(dstF64[1], 1) {
		t.Fatalf("F64 validity bounds: got %v", dstF64)
	}
	minMaxF64WithValidityFallback(f64, dstF64, invalid)
	if !math.IsInf(dstF64[0], 1) || !math.IsInf(dstF64[1], -1) {
		t.Fatalf("all-invalid MinMaxF64WithValidity: got %v", dstF64)
	}

	f32 := []float32{float32(math.Inf(-1)), 100, 100, 100, 100, 100, 100, 100, 100, float32(math.NaN())}
	dstF32 := make([]float32, 2)
	maxF32WithValidityFallback(f32, dstF32[:1], validity)
	minF32WithValidityFallback(f32, dstF32[1:], validity)
	if !math.IsInf(float64(dstF32[0]), -1) || !math.IsInf(float64(dstF32[1]), -1) {
		t.Fatalf("F32 validity bounds: got %v", dstF32)
	}
	minMaxF32WithValidityFallback(f32, dstF32, validity)
	if !math.IsInf(float64(dstF32[0]), -1) || !math.IsInf(float64(dstF32[1]), -1) {
		t.Fatalf("MinMaxF32WithValidity: got %v", dstF32)
	}
}
