//go:build arm64 && goexperiment.simd

package plan

import "simd/archsimd"

func init() { keyControlMasks = keyControlMasksNEON }

func keyControlMasksNEON(controls []byte, fingerprint byte) (uint16, uint16) {
	values := archsimd.LoadUint8x16(controls)
	matches := values.Equal(archsimd.BroadcastUint8x16(fingerprint)).ToInt8x16().ToBits().ReshapeToUint64s()
	empties := values.Equal(archsimd.BroadcastUint8x16(0)).ToInt8x16().ToBits().ReshapeToUint64s()
	mask := func(word uint64) uint16 { return uint16((word & 0x8080808080808080) * 0x0002040810204081 >> 56) }
	return mask(matches.GetElem(0)) | mask(matches.GetElem(1))<<8, mask(empties.GetElem(0)) | mask(empties.GetElem(1))<<8
}
