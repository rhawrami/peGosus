//go:build amd64

package mem

import "golang.org/x/sys/cpu"

func init() {
	if cpu.X86.HasAVX2 {
		setU8 = setU8UnalignedAVX2
		setU32 = setU32UnalignedAVX2
		setU64 = setU64UnalignedAVX2
	}
}

// setU8UnalignedAVX2 sets every byte, starting at `a` and going on for
// `n` bytes, with the value `v`.
//
//go:noescape
func setU8UnalignedAVX2(a *byte, n uint64, v uint8)

// setU32UnalignedAVX2 sets every four bytes, starting at `a` and going on for
// `n` bytes, with the value `v`.
//
//go:noescape
func setU32UnalignedAVX2(a *byte, n uint64, v uint32)

// setU64UnalignedAVX2 sets every eight bytes, starting at `a` and going on for
// `n` bytes, with the value `v`.
//
//go:noescape
func setU64UnalignedAVX2(a *byte, n uint64, v uint64)
