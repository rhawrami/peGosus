package plan

import "github.com/rhawrami/peGosus/pkg/mem"

func sortPackedKeyIDs(a *mem.Allocator, keys, ids []uint64, width int) bool {
	if len(keys) != len(ids) || width < 0 || width > 64 {
		return false
	}
	if len(keys) <= 1 || width == 0 {
		return true
	}
	if len(keys) > int(^uint(0)>>1)/16 {
		return false
	}
	buffer := a.AllocSegTemp(len(keys) * 16)
	if buffer == nil {
		return false
	}
	defer buffer.Dec()
	scratch := buffer.AsU64T()
	tempKeys, tempIDs := scratch[:len(keys)], scratch[len(keys):]
	sourceKeys, sourceIDs := keys, ids
	destKeys, destIDs := tempKeys, tempIDs
	for shift := uint(0); shift < uint(width); shift += 8 {
		var counts [256]int
		for _, key := range sourceKeys {
			counts[byte(key>>shift)]++
		}
		on := 0
		for i, n := range counts {
			counts[i] = on
			on += n
		}
		for i, key := range sourceKeys {
			bucket := byte(key >> shift)
			at := counts[bucket]
			destKeys[at], destIDs[at] = key, sourceIDs[i]
			counts[bucket]++
		}
		sourceKeys, destKeys = destKeys, sourceKeys
		sourceIDs, destIDs = destIDs, sourceIDs
	}
	if len(keys) != 0 && &sourceKeys[0] != &keys[0] {
		copy(keys, sourceKeys)
		copy(ids, sourceIDs)
	}
	return true
}
