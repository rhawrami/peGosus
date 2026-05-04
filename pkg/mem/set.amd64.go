//go:build amd64

package mem

import "golang.org/x/sys/cpu"

func init() {
	if cpu.X86.HasAVX2 {
		setU8 = setU8AlignedAVX2
		setU32 = setU32AlignedAVX2
		setU64 = setU64AlignedAVX2
	}
}

// setU8AlignedAVX2 sets every byte, starting at `a` and going on for
// `l` bytes, with the value `v`; assumes 32-byte alignment.
//
//go:noescape
func setU8AlignedAVX2(a *byte, l uint64, v uint8)

// setU32AlignedAVX2 sets every four bytes, starting at `a` and going on for
// `l` bytes, with the value `v`; assumes 32-byte alignment.
//
//go:noescape
func setU32AlignedAVX2(a *byte, l uint64, v uint32)

// setU64AlignedAVX2 sets every eight bytes, starting at `a` and going on for
// `l` bytes, with the value `v`; assumes 32-byte alignment.
//
//go:noescape
func setU64AlignedAVX2(a *byte, l uint64, v uint64)
