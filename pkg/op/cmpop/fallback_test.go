package cmpop

import (
	"bytes"
	"math"
	"strconv"
	"testing"
)

type fallbackTest struct {
	name string
	run  func([]byte)
	want []byte
}

func testFallbacks(t *testing.T, tests []fallbackTest) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dst := bytes.Repeat([]byte{0xff}, len(test.want))
			test.run(dst)
			if !bytes.Equal(dst, test.want) {
				t.Fatalf("got %08b, want %08b", dst, test.want)
			}
		})
	}
}

func TestI32Fallbacks(t *testing.T) {
	src := []int32{-2, -1, 0, 1, 2, 3, 4, 5, 6, 7}
	testFallbacks(t, []fallbackTest{
		{"Gt", func(dst []byte) { gtI32Fallback(src, dst, 2) }, []byte{0xe0, 0x03}},
		{"Lt", func(dst []byte) { ltI32Fallback(src, dst, 2) }, []byte{0x0f, 0x00}},
		{"Ge", func(dst []byte) { geI32Fallback(src, dst, 2) }, []byte{0xf0, 0x03}},
		{"Le", func(dst []byte) { leI32Fallback(src, dst, 2) }, []byte{0x1f, 0x00}},
		{"Eq", func(dst []byte) { eqI32Fallback(src, dst, 2) }, []byte{0x10, 0x00}},
		{"Neq", func(dst []byte) { neqI32Fallback(src, dst, 2) }, []byte{0xef, 0x03}},
		{"Bet", func(dst []byte) { betI32Fallback(src, dst, 0, 4) }, []byte{0x7c, 0x00}},
		{"NBet", func(dst []byte) { nBetI32Fallback(src, dst, 0, 4) }, []byte{0x83, 0x03}},
	})
}

func TestF32Fallbacks(t *testing.T) {
	src := []float32{-2, -1, 0, 1, 2, float32(math.NaN()), 4, 5, 6, 7}
	testFallbacks(t, []fallbackTest{
		{"Gt", func(dst []byte) { gtF32Fallback(src, dst, 2) }, []byte{0xc0, 0x03}},
		{"Lt", func(dst []byte) { ltF32Fallback(src, dst, 2) }, []byte{0x0f, 0x00}},
		{"Ge", func(dst []byte) { geF32Fallback(src, dst, 2) }, []byte{0xd0, 0x03}},
		{"Le", func(dst []byte) { leF32Fallback(src, dst, 2) }, []byte{0x1f, 0x00}},
		{"Eq", func(dst []byte) { eqF32Fallback(src, dst, 2) }, []byte{0x10, 0x00}},
		{"Neq", func(dst []byte) { neqF32Fallback(src, dst, 2) }, []byte{0xef, 0x03}},
		{"Bet", func(dst []byte) { betF32Fallback(src, dst, 0, 4) }, []byte{0x5c, 0x00}},
		{"NBet", func(dst []byte) { nBetF32Fallback(src, dst, 0, 4) }, []byte{0x83, 0x03}},
	})
}

func TestI64Fallbacks(t *testing.T) {
	src := []int64{-2, -1, 0, 1, 2, 3, 4, 5, 6, 7}
	testFallbacks(t, []fallbackTest{
		{"Gt", func(dst []byte) { gtI64Fallback(src, dst, 2) }, []byte{0xe0, 0x03}},
		{"Lt", func(dst []byte) { ltI64Fallback(src, dst, 2) }, []byte{0x0f, 0x00}},
		{"Ge", func(dst []byte) { geI64Fallback(src, dst, 2) }, []byte{0xf0, 0x03}},
		{"Le", func(dst []byte) { leI64Fallback(src, dst, 2) }, []byte{0x1f, 0x00}},
		{"Eq", func(dst []byte) { eqI64Fallback(src, dst, 2) }, []byte{0x10, 0x00}},
		{"Neq", func(dst []byte) { neqI64Fallback(src, dst, 2) }, []byte{0xef, 0x03}},
		{"Bet", func(dst []byte) { betI64Fallback(src, dst, 0, 4) }, []byte{0x7c, 0x00}},
		{"NBet", func(dst []byte) { nBetI64Fallback(src, dst, 0, 4) }, []byte{0x83, 0x03}},
	})
}

func TestF64Fallbacks(t *testing.T) {
	src := []float64{-2, -1, 0, 1, 2, math.NaN(), 4, 5, 6, 7}
	testFallbacks(t, []fallbackTest{
		{"Gt", func(dst []byte) { gtF64Fallback(src, dst, 2) }, []byte{0xc0, 0x03}},
		{"Lt", func(dst []byte) { ltF64Fallback(src, dst, 2) }, []byte{0x0f, 0x00}},
		{"Ge", func(dst []byte) { geF64Fallback(src, dst, 2) }, []byte{0xd0, 0x03}},
		{"Le", func(dst []byte) { leF64Fallback(src, dst, 2) }, []byte{0x1f, 0x00}},
		{"Eq", func(dst []byte) { eqF64Fallback(src, dst, 2) }, []byte{0x10, 0x00}},
		{"Neq", func(dst []byte) { neqF64Fallback(src, dst, 2) }, []byte{0xef, 0x03}},
		{"Bet", func(dst []byte) { betF64Fallback(src, dst, 0, 4) }, []byte{0x5c, 0x00}},
		{"NBet", func(dst []byte) { nBetF64Fallback(src, dst, 0, 4) }, []byte{0x83, 0x03}},
	})
}

func TestFallbacksEmpty(t *testing.T) {
	tests := []struct {
		name string
		run  func()
	}{
		{"GtI32", func() { gtI32Fallback(nil, nil, 0) }},
		{"LtI32", func() { ltI32Fallback(nil, nil, 0) }},
		{"GeI32", func() { geI32Fallback(nil, nil, 0) }},
		{"LeI32", func() { leI32Fallback(nil, nil, 0) }},
		{"EqI32", func() { eqI32Fallback(nil, nil, 0) }},
		{"NeqI32", func() { neqI32Fallback(nil, nil, 0) }},
		{"BetI32", func() { betI32Fallback(nil, nil, 0, 0) }},
		{"NBetI32", func() { nBetI32Fallback(nil, nil, 0, 0) }},
		{"GtF32", func() { gtF32Fallback(nil, nil, 0) }},
		{"LtF32", func() { ltF32Fallback(nil, nil, 0) }},
		{"GeF32", func() { geF32Fallback(nil, nil, 0) }},
		{"LeF32", func() { leF32Fallback(nil, nil, 0) }},
		{"EqF32", func() { eqF32Fallback(nil, nil, 0) }},
		{"NeqF32", func() { neqF32Fallback(nil, nil, 0) }},
		{"BetF32", func() { betF32Fallback(nil, nil, 0, 0) }},
		{"NBetF32", func() { nBetF32Fallback(nil, nil, 0, 0) }},
		{"GtI64", func() { gtI64Fallback(nil, nil, 0) }},
		{"LtI64", func() { ltI64Fallback(nil, nil, 0) }},
		{"GeI64", func() { geI64Fallback(nil, nil, 0) }},
		{"LeI64", func() { leI64Fallback(nil, nil, 0) }},
		{"EqI64", func() { eqI64Fallback(nil, nil, 0) }},
		{"NeqI64", func() { neqI64Fallback(nil, nil, 0) }},
		{"BetI64", func() { betI64Fallback(nil, nil, 0, 0) }},
		{"NBetI64", func() { nBetI64Fallback(nil, nil, 0, 0) }},
		{"GtF64", func() { gtF64Fallback(nil, nil, 0) }},
		{"LtF64", func() { ltF64Fallback(nil, nil, 0) }},
		{"GeF64", func() { geF64Fallback(nil, nil, 0) }},
		{"LeF64", func() { leF64Fallback(nil, nil, 0) }},
		{"EqF64", func() { eqF64Fallback(nil, nil, 0) }},
		{"NeqF64", func() { neqF64Fallback(nil, nil, 0) }},
		{"BetF64", func() { betF64Fallback(nil, nil, 0, 0) }},
		{"NBetF64", func() { nBetF64Fallback(nil, nil, 0, 0) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { test.run() })
	}
}

func TestFallbackByteBoundaries(t *testing.T) {
	for _, length := range []int{7, 8, 9, 15, 16, 17} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			src := make([]int32, length)
			dst := bytes.Repeat([]byte{0xff}, (length+7)/8)
			geI32Fallback(src, dst, 0)
			for i, value := range dst {
				want := byte(0xff)
				if i == len(dst)-1 && length%8 != 0 {
					want = 1<<(length%8) - 1
				}
				if value != want {
					t.Fatalf("byte %d: got %08b, want %08b", i, value, want)
				}
			}
		})
	}
}
