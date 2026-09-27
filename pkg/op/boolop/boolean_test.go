package boolop

import (
	"bytes"
	"math/rand/v2"
	"testing"
)

const (
	boolNull = iota
	boolFalse
	boolTrue
)

func TestBooleanOperationsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 32))
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17, 63, 64, 65, 257} {
		byteLen := (n + 7) / 8
		src1 := make([]byte, byteLen)
		validity1 := make([]byte, byteLen)
		src2 := make([]byte, byteLen)
		validity2 := make([]byte, byteLen)
		states1 := make([]int, n)
		states2 := make([]int, n)
		for i := 0; i < n; i++ {
			states1[i] = int(rng.Uint32N(3))
			states2[i] = int(rng.Uint32N(3))
			setBooleanState(src1, validity1, i, states1[i], rng.Uint32N(2) != 0)
			setBooleanState(src2, validity2, i, states2[i], rng.Uint32N(2) != 0)
		}

		testBooleanBinary(t, n, src1, validity1, src2, validity2, states1, states2, And, andReference)
		testBooleanBinary(t, n, src1, validity1, src2, validity2, states1, states2, Or, orReference)
		testBooleanNot(t, n, src1, validity1, states1)
	}
}

func TestBooleanOperationAliasing(t *testing.T) {
	n := 17
	src1 := []byte{0x39, 0xa5, 0xff}
	validity1 := []byte{0xdb, 0x7e, 0xff}
	src2 := []byte{0xc6, 0x5a, 0xff}
	validity2 := []byte{0x7e, 0xdb, 0xff}
	operations := []struct {
		name string
		fn   func([]byte, []byte, []byte, []byte, []byte, []byte, int)
	}{
		{"And", And},
		{"Or", Or},
	}
	for _, operation := range operations {
		want, wantValidity := make([]byte, 3), make([]byte, 3)
		operation.fn(src1, validity1, src2, validity2, want, wantValidity, n)
		for valueAlias := range 4 {
			for validityAlias := range 4 {
				if valueAlias == validityAlias {
					continue
				}
				inputs := [][]byte{
					append([]byte(nil), src1...), append([]byte(nil), validity1...),
					append([]byte(nil), src2...), append([]byte(nil), validity2...),
				}
				operation.fn(inputs[0], inputs[1], inputs[2], inputs[3], inputs[valueAlias], inputs[validityAlias], n)
				if !bytes.Equal(inputs[valueAlias], want) || !bytes.Equal(inputs[validityAlias], wantValidity) {
					t.Fatalf("%s aliases %d/%d: got %08b/%08b, want %08b/%08b", operation.name, valueAlias, validityAlias, inputs[valueAlias], inputs[validityAlias], want, wantValidity)
				}
			}
		}
	}

	want, wantValidity := make([]byte, 3), make([]byte, 3)
	Not(src1, validity1, want, wantValidity, n)
	aliasValue := append([]byte(nil), src1...)
	aliasValidity := append([]byte(nil), validity1...)
	Not(aliasValue, aliasValidity, aliasValue, aliasValidity, n)
	if !bytes.Equal(aliasValue, want) || !bytes.Equal(aliasValidity, wantValidity) {
		t.Fatalf("Not alias: got %08b/%08b, want %08b/%08b", aliasValue, aliasValidity, want, wantValidity)
	}
	aliasValue = append([]byte(nil), src1...)
	aliasValidity = append([]byte(nil), validity1...)
	Not(aliasValue, aliasValidity, aliasValidity, aliasValue, n)
	if !bytes.Equal(aliasValidity, want) || !bytes.Equal(aliasValue, wantValidity) {
		t.Fatalf("Not cross-alias: got %08b/%08b, want %08b/%08b", aliasValidity, aliasValue, want, wantValidity)
	}
}

func testBooleanBinary(t *testing.T, n int, src1, validity1, src2, validity2 []byte, states1, states2 []int, operation func([]byte, []byte, []byte, []byte, []byte, []byte, int), reference func(int, int) int) {
	t.Helper()
	byteLen := (n + 7) / 8
	dst := bytes.Repeat([]byte{0xff}, byteLen+1)
	dstValidity := bytes.Repeat([]byte{0xff}, byteLen+1)
	want := make([]byte, byteLen+1)
	wantValidity := make([]byte, byteLen+1)
	want[byteLen], wantValidity[byteLen] = 0xff, 0xff
	for i := 0; i < n; i++ {
		setBooleanState(want, wantValidity, i, reference(states1[i], states2[i]), false)
	}
	operation(src1, validity1, src2, validity2, dst, dstValidity, n)
	if !bytes.Equal(dst, want) || !bytes.Equal(dstValidity, wantValidity) {
		t.Fatalf("length %d: got %08b/%08b, want %08b/%08b", n, dst, dstValidity, want, wantValidity)
	}
}

func testBooleanNot(t *testing.T, n int, src, validity []byte, states []int) {
	t.Helper()
	byteLen := (n + 7) / 8
	dst := bytes.Repeat([]byte{0xff}, byteLen+1)
	dstValidity := bytes.Repeat([]byte{0xff}, byteLen+1)
	want := make([]byte, byteLen+1)
	wantValidity := make([]byte, byteLen+1)
	want[byteLen], wantValidity[byteLen] = 0xff, 0xff
	for i, state := range states {
		if state == boolTrue {
			state = boolFalse
		} else if state == boolFalse {
			state = boolTrue
		}
		setBooleanState(want, wantValidity, i, state, false)
	}
	Not(src, validity, dst, dstValidity, n)
	if !bytes.Equal(dst, want) || !bytes.Equal(dstValidity, wantValidity) {
		t.Fatalf("length %d: got %08b/%08b, want %08b/%08b", n, dst, dstValidity, want, wantValidity)
	}
}

func setBooleanState(values, validity []byte, i, state int, nullValue bool) {
	if state == boolNull {
		if nullValue {
			values[i/8] |= 1 << (i % 8)
		}
		return
	}
	validity[i/8] |= 1 << (i % 8)
	if state == boolTrue {
		values[i/8] |= 1 << (i % 8)
	}
}

func andReference(a, b int) int {
	if a == boolFalse || b == boolFalse {
		return boolFalse
	}
	if a == boolTrue && b == boolTrue {
		return boolTrue
	}
	return boolNull
}

func orReference(a, b int) int {
	if a == boolTrue || b == boolTrue {
		return boolTrue
	}
	if a == boolFalse && b == boolFalse {
		return boolFalse
	}
	return boolNull
}
