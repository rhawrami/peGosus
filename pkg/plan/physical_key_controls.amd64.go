//go:build amd64 && goexperiment.simd

package plan

import "simd/archsimd"

func init() {
	if archsimd.X86.AVX2() {
		keyControlMasks = keyControlMasksAVX2
	}
}

func keyControlMasksAVX2(controls []byte, fingerprint byte) (uint16, uint16) {
	values := archsimd.LoadUint8x16(controls)
	return uint16(values.Equal(archsimd.BroadcastUint8x16(fingerprint)).ToBits()), uint16(values.Equal(archsimd.BroadcastUint8x16(0)).ToBits())
}
