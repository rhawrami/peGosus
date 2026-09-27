package numop

import (
	"bytes"
	"math"
	"math/rand/v2"
	"testing"
)

func TestAdditionalNumericCastsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17, 63, 257} {
		i64 := make([]int64, n)
		f64 := make([]float64, n)
		for i := range i64 {
			i64[i] = int64(rng.Uint64())
			f64[i] = math.Float64frombits(rng.Uint64())
		}
		dstI32 := append(make([]int32, n), 99)
		dstF32 := append(make([]float32, n), 99)
		CastI64ToI32(i64, dstI32)
		CastF64ToF32(f64, dstF32)
		for i := 0; i < n; i++ {
			if dstI32[i] != int32(i64[i]) {
				t.Fatalf("CastI64ToI32 length %d index %d: got %d, want %d", n, i, dstI32[i], int32(i64[i]))
			}
			if math.Float32bits(dstF32[i]) != math.Float32bits(float32(f64[i])) {
				t.Fatalf("CastF64ToF32 length %d index %d: got %v, want %v", n, i, dstF32[i], float32(f64[i]))
			}
		}
		if dstI32[n] != 99 || dstF32[n] != 99 {
			t.Fatalf("length %d overwrote sentinel", n)
		}
	}
}

func TestAdditionalNumericCastSpecialValues(t *testing.T) {
	integers := []int64{math.MinInt64, math.MinInt32 - 1, math.MinInt32, -1, 0, 1, math.MaxInt32, math.MaxInt32 + 1, math.MaxInt64}
	wantIntegers := make([]int32, len(integers))
	for i, value := range integers {
		wantIntegers[i] = int32(value)
	}
	gotIntegers := make([]int32, len(integers))
	CastI64ToI32(integers, gotIntegers)
	for i := range wantIntegers {
		if gotIntegers[i] != wantIntegers[i] {
			t.Fatalf("integer %d: got %d, expected %d", integers[i], gotIntegers[i], wantIntegers[i])
		}
	}

	floats := []float64{
		math.NaN(), math.Inf(-1), math.Inf(1), math.Copysign(0, -1), 0,
		-math.MaxFloat64, -math.MaxFloat32, math.MaxFloat32, math.Nextafter(float64(math.MaxFloat32), math.Inf(1)), math.MaxFloat64,
	}
	gotFloats := make([]float32, len(floats))
	CastF64ToF32(floats, gotFloats)
	for i, value := range floats {
		if math.Float32bits(gotFloats[i]) != math.Float32bits(float32(value)) {
			t.Fatalf("float %v: got %v, expected %v", value, gotFloats[i], float32(value))
		}
	}
}

func TestCheckedNumericCastBoundaries(t *testing.T) {
	f64I32 := []float64{
		math.Inf(-1), math.NaN(), math.Inf(1),
		-2147483649, math.Nextafter(-2147483649, math.Inf(1)), -2147483648.75,
		math.Nextafter(-2147483648, math.Inf(-1)), -2147483648, -1.75, math.Copysign(0, -1),
		0, 1.75, 2147483647, math.Nextafter(2147483647, math.Inf(1)), 2147483647.75,
		math.Nextafter(2147483648, math.Inf(-1)), 2147483648,
	}
	testCheckedF64I32(t, f64I32)

	f64I64 := []float64{
		math.Inf(-1), math.NaN(), math.Inf(1),
		math.Nextafter(-0x1p63, math.Inf(-1)), -0x1p63, -1.75, math.Copysign(0, -1),
		0, 1.75, math.Nextafter(0x1p63, 0), 0x1p63,
	}
	testCheckedF64I64(t, f64I64)

	f32I32 := []float32{
		float32(math.Inf(-1)), float32(math.NaN()), float32(math.Inf(1)),
		math.Float32frombits(math.Float32bits(-0x1p31) + 1), -0x1p31, -1.75, float32(math.Copysign(0, -1)),
		0, 1.75, math.Float32frombits(math.Float32bits(0x1p31) - 1), 0x1p31,
	}
	testCheckedF32I32(t, f32I32)

	f32I64 := []float32{
		float32(math.Inf(-1)), float32(math.NaN()), float32(math.Inf(1)),
		math.Float32frombits(math.Float32bits(-0x1p63) + 1), -0x1p63, -1.75, float32(math.Copysign(0, -1)),
		0, 1.75, math.Float32frombits(math.Float32bits(0x1p63) - 1), 0x1p63,
	}
	testCheckedF32I64(t, f32I64)
}

func TestCheckedNumericCastsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(23, 24))
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17, 257} {
		f64 := make([]float64, n)
		f32 := make([]float32, n)
		for i := range f64 {
			f64[i] = math.Float64frombits(rng.Uint64())
			f32[i] = math.Float32frombits(rng.Uint32())
		}
		testCheckedF64I32(t, f64)
		testCheckedF64I64(t, f64)
		testCheckedF32I32(t, f32)
		testCheckedF32I64(t, f32)
	}
}

func testCheckedF64I32(t *testing.T, src []float64) {
	t.Helper()
	dst := append(make([]int32, len(src)), 99)
	validity := bytes.Repeat([]byte{0xff}, (len(src)+7)/8+1)
	wantValidity := make([]byte, (len(src)+7)/8+1)
	wantValidity[len(wantValidity)-1] = 0xff
	CastF64ToI32Checked(src, dst, validity)
	for i, value := range src {
		valid := value >= -0x1p31 && value < 0x1p31
		if valid {
			wantValidity[i/8] |= 1 << (i % 8)
			if dst[i] != int32(value) {
				t.Fatalf("F64ToI32 index %d: got %d, want %d", i, dst[i], int32(value))
			}
		} else if dst[i] != 0 {
			t.Fatalf("F64ToI32 invalid index %d has payload %d", i, dst[i])
		}
	}
	checkCheckedCastOutput(t, dst[len(src)] == 99, validity, wantValidity)
}

func testCheckedF64I64(t *testing.T, src []float64) {
	t.Helper()
	dst := append(make([]int64, len(src)), 99)
	validity := bytes.Repeat([]byte{0xff}, (len(src)+7)/8+1)
	wantValidity := make([]byte, (len(src)+7)/8+1)
	wantValidity[len(wantValidity)-1] = 0xff
	CastF64ToI64Checked(src, dst, validity)
	for i, value := range src {
		valid := value >= -0x1p63 && value < 0x1p63
		if valid {
			wantValidity[i/8] |= 1 << (i % 8)
			if dst[i] != int64(value) {
				t.Fatalf("F64ToI64 index %d: got %d, want %d", i, dst[i], int64(value))
			}
		} else if dst[i] != 0 {
			t.Fatalf("F64ToI64 invalid index %d has payload %d", i, dst[i])
		}
	}
	checkCheckedCastOutput(t, dst[len(src)] == 99, validity, wantValidity)
}

func testCheckedF32I32(t *testing.T, src []float32) {
	t.Helper()
	dst := append(make([]int32, len(src)), 99)
	validity := bytes.Repeat([]byte{0xff}, (len(src)+7)/8+1)
	wantValidity := make([]byte, (len(src)+7)/8+1)
	wantValidity[len(wantValidity)-1] = 0xff
	CastF32ToI32Checked(src, dst, validity)
	for i, value := range src {
		valid := value >= -0x1p31 && value < 0x1p31
		if valid {
			wantValidity[i/8] |= 1 << (i % 8)
			if dst[i] != int32(value) {
				t.Fatalf("F32ToI32 index %d: got %d, want %d", i, dst[i], int32(value))
			}
		} else if dst[i] != 0 {
			t.Fatalf("F32ToI32 invalid index %d has payload %d", i, dst[i])
		}
	}
	checkCheckedCastOutput(t, dst[len(src)] == 99, validity, wantValidity)
}

func testCheckedF32I64(t *testing.T, src []float32) {
	t.Helper()
	dst := append(make([]int64, len(src)), 99)
	validity := bytes.Repeat([]byte{0xff}, (len(src)+7)/8+1)
	wantValidity := make([]byte, (len(src)+7)/8+1)
	wantValidity[len(wantValidity)-1] = 0xff
	CastF32ToI64Checked(src, dst, validity)
	for i, value := range src {
		valid := value >= -0x1p63 && value < 0x1p63
		if valid {
			wantValidity[i/8] |= 1 << (i % 8)
			if dst[i] != int64(value) {
				t.Fatalf("F32ToI64 index %d: got %d, want %d", i, dst[i], int64(value))
			}
		} else if dst[i] != 0 {
			t.Fatalf("F32ToI64 invalid index %d has payload %d", i, dst[i])
		}
	}
	checkCheckedCastOutput(t, dst[len(src)] == 99, validity, wantValidity)
}

func checkCheckedCastOutput(t *testing.T, payloadSentinel bool, gotValidity, wantValidity []byte) {
	t.Helper()
	if !payloadSentinel {
		t.Fatal("checked cast overwrote payload sentinel")
	}
	if !bytes.Equal(gotValidity, wantValidity) {
		t.Fatalf("validity got %08b, want %08b", gotValidity, wantValidity)
	}
}
