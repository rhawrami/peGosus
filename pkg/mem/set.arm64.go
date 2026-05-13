//go:build arm64

package mem

// setU8 sets every byte, starting at `a` and going on for
// `n` iterations, with the value `v`.
//
//go:noescape
func setU8(a *byte, n int, v uint8)

// setU32 sets every four bytes, starting at `a` and going on for
// `n` iterations, with the value `v`.
//
//go:noescape
func setU32(a *byte, n int, v uint32)

// setU64 sets every eight bytes, starting at `a` and going on for
// `n` iterations, with the value `v`.
//
//go:noescape
func setU64(a *byte, n int, v uint64)
