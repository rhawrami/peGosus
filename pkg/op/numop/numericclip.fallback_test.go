package numop

import (
	"math"
	"reflect"
	"testing"
)

func TestNumericClipFallback(t *testing.T) {
	f64 := []float64{math.Inf(-1), -2, 0, 2, math.Inf(1), math.NaN()}
	dstF64 := make([]float64, len(f64))
	clipF64WithF64BoundsFallback(f64, dstF64, -1, 1)
	if !reflect.DeepEqual(dstF64[:5], []float64{-1, -1, 0, 1, 1}) || !math.IsNaN(dstF64[5]) {
		t.Fatalf("ClipF64WithF64Bounds: got %v", dstF64)
	}

	f32 := []float32{float32(math.Inf(-1)), -2, 0, 2, float32(math.Inf(1)), float32(math.NaN())}
	dstF32 := make([]float32, len(f32))
	clipF32WithF32BoundsFallback(f32, dstF32, -1, 1)
	if !reflect.DeepEqual(dstF32[:5], []float32{-1, -1, 0, 1, 1}) || !math.IsNaN(float64(dstF32[5])) {
		t.Fatalf("ClipF32WithF32Bounds: got %v", dstF32)
	}

	i64 := []int64{math.MinInt64, -1, 0, 1, math.MaxInt64}
	dstI64 := make([]int64, len(i64))
	clipI64WithI64BoundsFallback(i64, dstI64, -10, 10)
	if expected := []int64{-10, -1, 0, 1, 10}; !reflect.DeepEqual(dstI64, expected) {
		t.Fatalf("ClipI64WithI64Bounds: got %v, expected %v", dstI64, expected)
	}

	i32 := []int32{math.MinInt32, -1, 0, 1, math.MaxInt32}
	dstI32 := make([]int32, len(i32))
	clipI32WithI32BoundsFallback(i32, dstI32, -10, 10)
	if expected := []int32{-10, -1, 0, 1, 10}; !reflect.DeepEqual(dstI32, expected) {
		t.Fatalf("ClipI32WithI32Bounds: got %v, expected %v", dstI32, expected)
	}

	dstF64 = make([]float64, len(i64))
	clipI64WithF64BoundsFallback(i64, dstF64, -10.5, 10.5)
	if expected := []float64{-10.5, -1, 0, 1, 10.5}; !reflect.DeepEqual(dstF64, expected) {
		t.Fatalf("ClipI64WithF64Bounds: got %v, expected %v", dstF64, expected)
	}

	dstF32 = make([]float32, len(i32))
	clipI32WithF32BoundsFallback(i32, dstF32, -10.5, 10.5)
	if expected := []float32{-10.5, -1, 0, 1, 10.5}; !reflect.DeepEqual(dstF32, expected) {
		t.Fatalf("ClipI32WithF32Bounds: got %v, expected %v", dstF32, expected)
	}

	clipI64WithI64BoundsFallback(nil, nil, -1, 1)
}
