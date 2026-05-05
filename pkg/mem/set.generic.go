//go:build !arm64

package mem

var (
	setU8  func(a *byte, n uint64, v uint8)  = setU8Generic
	setU32 func(a *byte, n uint64, v uint32) = setU32Generic
	setU64 func(a *byte, n uint64, v uint64) = setU64Generic
)

// setU8Generic sets every byte, starting at `a` and going on for
// `n` iterations, with the value `v`.
func setU8Generic(a *byte, n uint64, v uint8) {
	s := asBT(a, int(n))
	for i := 0; i < int(n); i++ {
		s[i] = v
	}
}

// setU32Generic sets every four bytes, starting at `a` and going on for
// `n` iterations, with the value `v`.
func setU32Generic(a *byte, n uint64, v uint32) {
	s := asU32T(a, int(n))
	for i := 0; i < int(n); i++ {
		s[i] = v
	}
}

// setU64Generic sets every eight bytes, starting at `a` and going on for
// `n` iterations, with the value `v`.
func setU64Generic(a *byte, n uint64, v uint64) {
	s := asU64T(a, int(n))
	for i := 0; i < int(n); i++ {
		s[i] = v
	}
}
