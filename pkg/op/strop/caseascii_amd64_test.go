//go:build amd64

package strop

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"golang.org/x/sys/cpu"
)

func TestASCIIAssemblyDifferential(t *testing.T) {
	if !cpu.X86.HasAVX2 {
		t.Skip("AVX2 unavailable")
	}
	rng := rand.New(rand.NewPCG(422, 117))
	for _, test := range []struct {
		name      string
		kernel    func([]byte, []byte)
		reference func([]byte, []byte)
	}{
		{"upper", toUpperASCII, toUpperASCIIFallback},
		{"lower", toLowerASCII, toLowerASCIIFallback},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, n := range []int{0, 1, 2, 3, 7, 15, 16, 31, 32, 63, 64, 65, 127, 128, 129, 257, 1024} {
				for range 50 {
					src := make([]byte, n)
					for i := range src {
						if rng.IntN(3) == 0 {
							src[i] = byte(rng.IntN(256))
						} else {
							src[i] = byte('A' + rng.IntN(58))
						}
					}
					want := make([]byte, n)
					test.reference(src, want)
					dst := bytes.Repeat([]byte{0xa5}, n+8)
					test.kernel(src, dst[:n])
					if !bytes.Equal(dst[:n], want) || !bytes.Equal(dst[n:], bytes.Repeat([]byte{0xa5}, 8)) {
						t.Fatalf("length %d: got %q, want %q", n, dst[:n], want)
					}
					alias := bytes.Clone(src)
					test.kernel(alias, alias)
					if !bytes.Equal(alias, want) {
						t.Fatalf("length %d in-place: got %q, want %q", n, alias, want)
					}
				}
			}
		})
	}
}
