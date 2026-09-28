package strop

import (
	"math/rand/v2"
	"testing"
)

func TestASCIIFallbackDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(43, 50))
	for _, length := range []int{0, 1, 7, 8, 9, 15, 16, 17, 257} {
		input := make([]byte, length)
		for i := range input {
			input[i] = byte(rng.Uint32())
		}
		for _, test := range []struct {
			name  string
			run   func([]byte, []byte)
			lower bool
		}{
			{"upper", toUpperASCIIFallback, false}, {"lower", toLowerASCIIFallback, true},
		} {
			want := append([]byte(nil), input...)
			for i, value := range want {
				if test.lower && value >= 'A' && value <= 'Z' {
					want[i] = value + ('a' - 'A')
				}
				if !test.lower && value >= 'a' && value <= 'z' {
					want[i] = value - ('a' - 'A')
				}
			}
			dst := make([]byte, length)
			for i := range dst {
				dst[i] = 0xff
			}
			test.run(input, dst)
			for i := range want {
				if dst[i] != want[i] {
					t.Fatalf("%s length %d row %d: got %x, want %x", test.name, length, i, dst[i], want[i])
				}
			}
			alias := append([]byte(nil), input...)
			test.run(alias, alias)
			for i := range want {
				if alias[i] != want[i] {
					t.Fatalf("%s alias length %d row %d: got %x, want %x", test.name, length, i, alias[i], want[i])
				}
			}
		}
	}
}
