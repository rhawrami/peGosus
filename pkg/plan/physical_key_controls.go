package plan

import "encoding/binary"

const (
	keyControlGroup       = 16
	keyControlMinCapacity = 1024
)

var keyControlMasks = keyControlMasksScalar

func keyControlMasksScalar(controls []byte, fingerprint byte) (uint16, uint16) {
	broadcast := uint64(fingerprint) * 0x0101010101010101
	var candidates, empties uint16
	for word := range 2 {
		value := binary.LittleEndian.Uint64(controls[word*8:])
		x := value ^ broadcast
		matches := ^(((x & 0x7f7f7f7f7f7f7f7f) + 0x7f7f7f7f7f7f7f7f) | x | 0x7f7f7f7f7f7f7f7f)
		empty := ^(((value & 0x7f7f7f7f7f7f7f7f) + 0x7f7f7f7f7f7f7f7f) | value | 0x7f7f7f7f7f7f7f7f)
		candidates |= uint16((matches&0x8080808080808080)*0x0002040810204081>>56) << (word * 8)
		empties |= uint16((empty&0x8080808080808080)*0x0002040810204081>>56) << (word * 8)
	}
	return candidates, empties
}
