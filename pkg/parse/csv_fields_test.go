package parse

import (
	"math/rand/v2"
	"testing"
)

func TestCSVFieldClassifierDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(712, 63))
	for length := range 65 {
		for range 60 {
			chunk := make([]byte, length)
			for i := range chunk {
				chunk[i] = []byte{'a', ',', '"', '\r', '\n', 0, 0xff}[rng.IntN(7)]
			}
			for _, delimiter := range []byte{',', '|', 0xff} {
				want := classifyCSVScalar(chunk, delimiter)
				if got := classifyCSV(chunk, delimiter); got != want {
					t.Fatalf("length=%d delimiter=%q: got %+v, want %+v", length, delimiter, got, want)
				}
			}
		}
	}
}

func TestCSVStructuralScannerDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(883, 29))
	for _, length := range []int{0, 1, 7, 15, 16, 17, 31, 32, 33, 63, 64, 65, 127, 128, 129, 257, 1025} {
		for range 100 {
			input := make([]byte, length)
			for i := range input {
				input[i] = []byte{'a', ',', '"', '\r', '\n', 0, 0xff}[rng.IntN(7)]
			}
			initialQuote := rng.IntN(2) != 0
			wantQuote, wantHasQuote, wantEnded := initialQuote, false, false
			want := []int{999}
			for i, value := range input {
				if value == '"' {
					wantQuote = !wantQuote
					wantHasQuote = true
				}
				if !wantQuote {
					if value == ',' {
						want = append(want, i+1)
					}
					if value == '\n' {
						wantEnded = true
					}
				}
			}
			for _, split := range []int{length, min(length, 1), min(length, 15), min(length, 31), min(length, 63), min(length, 64), min(length, 65), min(length, 127)} {
				got, firstQuote, carry, firstEnded := scanCSVChunk(input[:split], ',', []int{999}, 1, initialQuote)
				got, secondQuote, carry, secondEnded := scanCSVChunk(input[split:], ',', got, split+1, carry)
				if carry != wantQuote || (firstQuote || secondQuote) != wantHasQuote || (firstEnded || secondEnded) != wantEnded || len(got) != len(want) {
					t.Fatalf("length=%d split=%d: quote=%t hasQuote=%t ended=%t offsets=%v; want %t %t %t %v", length, split, carry, firstQuote || secondQuote, firstEnded || secondEnded, got, wantQuote, wantHasQuote, wantEnded, want)
				}
				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("length=%d split=%d offset %d: got %d, want %d", length, split, i, got[i], want[i])
					}
				}
			}
		}
	}
}
