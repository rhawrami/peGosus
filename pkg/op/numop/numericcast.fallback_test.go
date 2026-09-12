package numop

import (
	"math"
	"reflect"
	"testing"
)

func TestNumericCastFallback(t *testing.T) {
	i64 := []int64{math.MinInt64, -1, 0, 1, math.MaxInt64}
	i32 := []int32{math.MinInt32, -1, 0, 1, math.MaxInt32}
	f64 := []float64{-123.75, -1, 0, 1, 123.75}
	f32 := []float32{-123.75, -1, 0, 1, 123.75}

	dstF64 := make([]float64, len(i64))
	castI64ToF64Fallback(i64, dstF64)
	for i := range i64 {
		if dstF64[i] != float64(i64[i]) {
			t.Fatalf("CastI64ToF64[%d]: got %v, expected %v", i, dstF64[i], float64(i64[i]))
		}
	}

	dstF32 := make([]float32, len(i32))
	castI32ToF32Fallback(i32, dstF32)
	for i := range i32 {
		if dstF32[i] != float32(i32[i]) {
			t.Fatalf("CastI32ToF32[%d]: got %v, expected %v", i, dstF32[i], float32(i32[i]))
		}
	}

	dstI64 := make([]int64, len(f64))
	castF64ToI64Fallback(f64, dstI64)
	if expected := []int64{-123, -1, 0, 1, 123}; !reflect.DeepEqual(dstI64, expected) {
		t.Fatalf("CastF64ToI64: got %v, expected %v", dstI64, expected)
	}

	dstI32 := make([]int32, len(f32))
	castF32ToI32Fallback(f32, dstI32)
	if expected := []int32{-123, -1, 0, 1, 123}; !reflect.DeepEqual(dstI32, expected) {
		t.Fatalf("CastF32ToI32: got %v, expected %v", dstI32, expected)
	}

	dstF64 = make([]float64, len(f32))
	castF32ToF64Fallback(f32, dstF64)
	for i := range f32 {
		if dstF64[i] != float64(f32[i]) {
			t.Fatalf("CastF32ToF64[%d]: got %v, expected %v", i, dstF64[i], float64(f32[i]))
		}
	}

	dstI64 = make([]int64, len(i32))
	castI32ToI64Fallback(i32, dstI64)
	for i := range i32 {
		if dstI64[i] != int64(i32[i]) {
			t.Fatalf("CastI32ToI64[%d]: got %v, expected %v", i, dstI64[i], int64(i32[i]))
		}
	}

	dstF64 = make([]float64, len(i32))
	castI32ToF64Fallback(i32, dstF64)
	for i := range i32 {
		if dstF64[i] != float64(i32[i]) {
			t.Fatalf("CastI32ToF64[%d]: got %v, expected %v", i, dstF64[i], float64(i32[i]))
		}
	}

	dstI64 = make([]int64, len(f32))
	castF32ToI64Fallback(f32, dstI64)
	if expected := []int64{-123, -1, 0, 1, 123}; !reflect.DeepEqual(dstI64, expected) {
		t.Fatalf("CastF32ToI64: got %v, expected %v", dstI64, expected)
	}

	dstF32 = make([]float32, len(i64))
	castI64ToF32Fallback(i64, dstF32)
	for i := range i64 {
		if dstF32[i] != float32(i64[i]) {
			t.Fatalf("CastI64ToF32[%d]: got %v, expected %v", i, dstF32[i], float32(i64[i]))
		}
	}

	dstI32 = make([]int32, len(f64))
	castF64ToI32Fallback(f64, dstI32)
	if expected := []int32{-123, -1, 0, 1, 123}; !reflect.DeepEqual(dstI32, expected) {
		t.Fatalf("CastF64ToI32: got %v, expected %v", dstI32, expected)
	}

	castI64ToF64Fallback(nil, nil)
}
