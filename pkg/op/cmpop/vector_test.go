package cmpop

import (
	"bytes"
	"math"
	"math/rand/v2"
	"testing"
)

var vectorComparisonLengths = []int{0, 1, 7, 8, 9, 15, 16, 17, 31, 32, 33, 257}

func TestI32VectorComparisonsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	operations := []struct {
		name string
		fn   func([]int32, []int32, []byte)
		want func(int32, int32) bool
	}{
		{"Eq", EqI32Vec, func(a, b int32) bool { return a == b }},
		{"Neq", NeqI32Vec, func(a, b int32) bool { return a != b }},
		{"Lt", LtI32Vec, func(a, b int32) bool { return a < b }},
		{"Le", LeI32Vec, func(a, b int32) bool { return a <= b }},
		{"Gt", GtI32Vec, func(a, b int32) bool { return a > b }},
		{"Ge", GeI32Vec, func(a, b int32) bool { return a >= b }},
	}
	for _, n := range vectorComparisonLengths {
		src1 := make([]int32, n)
		src2 := make([]int32, n)
		for i := range src1 {
			src1[i] = int32(rng.Uint32())
			src2[i] = int32(rng.Uint32())
			if i%5 == 0 {
				src2[i] = src1[i]
			}
		}
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				testI32VectorComparison(t, src1, src2, operation.fn, operation.want)
			})
		}
	}
}

func TestI64VectorComparisonsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	operations := []struct {
		name string
		fn   func([]int64, []int64, []byte)
		want func(int64, int64) bool
	}{
		{"Eq", EqI64Vec, func(a, b int64) bool { return a == b }},
		{"Neq", NeqI64Vec, func(a, b int64) bool { return a != b }},
		{"Lt", LtI64Vec, func(a, b int64) bool { return a < b }},
		{"Le", LeI64Vec, func(a, b int64) bool { return a <= b }},
		{"Gt", GtI64Vec, func(a, b int64) bool { return a > b }},
		{"Ge", GeI64Vec, func(a, b int64) bool { return a >= b }},
	}
	for _, n := range vectorComparisonLengths {
		src1 := make([]int64, n)
		src2 := make([]int64, n)
		for i := range src1 {
			src1[i] = int64(rng.Uint64())
			src2[i] = int64(rng.Uint64())
			if i%5 == 0 {
				src2[i] = src1[i]
			}
		}
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				testI64VectorComparison(t, src1, src2, operation.fn, operation.want)
			})
		}
	}
}

func TestF32VectorComparisonsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	operations := []struct {
		name string
		fn   func([]float32, []float32, []byte)
		want func(float32, float32) bool
	}{
		{"Eq", EqF32Vec, func(a, b float32) bool { return a == b }},
		{"Neq", NeqF32Vec, func(a, b float32) bool { return a != b }},
		{"Lt", LtF32Vec, func(a, b float32) bool { return a < b }},
		{"Le", LeF32Vec, func(a, b float32) bool { return a <= b }},
		{"Gt", GtF32Vec, func(a, b float32) bool { return a > b }},
		{"Ge", GeF32Vec, func(a, b float32) bool { return a >= b }},
	}
	for _, n := range vectorComparisonLengths {
		src1 := make([]float32, n)
		src2 := make([]float32, n)
		for i := range src1 {
			src1[i] = math.Float32frombits(rng.Uint32())
			src2[i] = math.Float32frombits(rng.Uint32())
			if i%5 == 0 {
				src2[i] = src1[i]
			}
		}
		setF32ComparisonSpecials(src1, src2)
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				testF32VectorComparison(t, src1, src2, operation.fn, operation.want)
			})
		}
	}
}

func TestF64VectorComparisonsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	operations := []struct {
		name string
		fn   func([]float64, []float64, []byte)
		want func(float64, float64) bool
	}{
		{"Eq", EqF64Vec, func(a, b float64) bool { return a == b }},
		{"Neq", NeqF64Vec, func(a, b float64) bool { return a != b }},
		{"Lt", LtF64Vec, func(a, b float64) bool { return a < b }},
		{"Le", LeF64Vec, func(a, b float64) bool { return a <= b }},
		{"Gt", GtF64Vec, func(a, b float64) bool { return a > b }},
		{"Ge", GeF64Vec, func(a, b float64) bool { return a >= b }},
	}
	for _, n := range vectorComparisonLengths {
		src1 := make([]float64, n)
		src2 := make([]float64, n)
		for i := range src1 {
			src1[i] = math.Float64frombits(rng.Uint64())
			src2[i] = math.Float64frombits(rng.Uint64())
			if i%5 == 0 {
				src2[i] = src1[i]
			}
		}
		setF64ComparisonSpecials(src1, src2)
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				testF64VectorComparison(t, src1, src2, operation.fn, operation.want)
			})
		}
	}
}

func testI32VectorComparison(t *testing.T, src1, src2 []int32, fn func([]int32, []int32, []byte), predicate func(int32, int32) bool) {
	t.Helper()
	testVectorBitmap(t, len(src1), func(dst []byte) { fn(src1, src2, dst) }, func(i int) bool { return predicate(src1[i], src2[i]) })
}

func testI64VectorComparison(t *testing.T, src1, src2 []int64, fn func([]int64, []int64, []byte), predicate func(int64, int64) bool) {
	t.Helper()
	testVectorBitmap(t, len(src1), func(dst []byte) { fn(src1, src2, dst) }, func(i int) bool { return predicate(src1[i], src2[i]) })
}

func testF32VectorComparison(t *testing.T, src1, src2 []float32, fn func([]float32, []float32, []byte), predicate func(float32, float32) bool) {
	t.Helper()
	testVectorBitmap(t, len(src1), func(dst []byte) { fn(src1, src2, dst) }, func(i int) bool { return predicate(src1[i], src2[i]) })
}

func testF64VectorComparison(t *testing.T, src1, src2 []float64, fn func([]float64, []float64, []byte), predicate func(float64, float64) bool) {
	t.Helper()
	testVectorBitmap(t, len(src1), func(dst []byte) { fn(src1, src2, dst) }, func(i int) bool { return predicate(src1[i], src2[i]) })
}

func testVectorBitmap(t *testing.T, n int, run func([]byte), predicate func(int) bool) {
	t.Helper()
	byteLen := (n + 7) / 8
	dst := bytes.Repeat([]byte{0xff}, byteLen+1)
	want := make([]byte, byteLen+1)
	want[byteLen] = 0xff
	for i := 0; i < n; i++ {
		if predicate(i) {
			want[i/8] |= 1 << (i % 8)
		}
	}
	run(dst)
	if !bytes.Equal(dst, want) {
		t.Fatalf("length %d: got %08b, want %08b", n, dst, want)
	}
}

func setF32ComparisonSpecials(src1, src2 []float32) {
	values1 := []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)), 0, float32(math.Copysign(0, -1))}
	values2 := []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Copysign(0, -1)), 0}
	for i := 0; i < len(src1) && i < len(values1); i++ {
		src1[i], src2[i] = values1[i], values2[i]
	}
}

func setF64ComparisonSpecials(src1, src2 []float64) {
	values1 := []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, math.Copysign(0, -1)}
	values2 := []float64{math.NaN(), math.Inf(1), math.Inf(1), math.Copysign(0, -1), 0}
	for i := 0; i < len(src1) && i < len(values1); i++ {
		src1[i], src2[i] = values1[i], values2[i]
	}
}
