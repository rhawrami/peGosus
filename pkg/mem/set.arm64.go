//go:build arm64

package mem

// setU8 sets every byte, starting at `a` and going on for
// `l` iterations, with the value `v`.
//
//go:noescape
func setU8(a *byte, l uint64, v uint8)

// setU32 sets every four bytes, starting at `a` and going on for
// `l` iterations, with the value `v`.
//
//go:noescape
func setU32(a *byte, l uint64, v uint32)

// setU64 sets every eight bytes, starting at `a` and going on for
// `l` iterations, with the value `v`.
//
//go:noescape
func setU64(a *byte, l uint64, v uint64)
