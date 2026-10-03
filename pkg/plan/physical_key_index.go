package plan

import (
	"bytes"
	"hash/maphash"
	"math/bits"

	"github.com/rhawrami/peGosus/pkg/mem"
)

type keyIndex struct {
	slots       *mem.Segment
	controls    *mem.Segment
	scope       *mem.AllocationScope
	seed        maphash.Seed
	used        int
	stringKeys  bool
	tryControls bool
}

func (h *keyIndex) release() {
	if h.slots != nil {
		h.slots.Dec()
	}
	if h.controls != nil {
		h.controls.Dec()
	}
	*h = keyIndex{}
}

func (h *keyIndex) bytes() int64 {
	if h.slots == nil {
		return 0
	}
	bytes := int64(h.slots.Len())
	if h.controls != nil {
		bytes += int64(h.controls.Len())
	}
	return bytes
}

func (h *keyIndex) lookup(key []byte, keys *keyArena) (int, bool) {
	if h.slots == nil {
		return 0, false
	}
	slots := h.slots.AsU64T()
	mask := uint64(len(slots)/2 - 1)
	hash := h.hash(key)
	if h.controls != nil {
		return h.lookupControlled(hash, key, "", false, false, keys)
	}
	for at := hash & mask; ; at = (at + 1) & mask {
		index := slots[2*at+1]
		if index == 0 {
			return 0, false
		}
		if slots[2*at] == hash && bytes.Equal(keys.key(int(index-1)), key) {
			return int(index - 1), true
		}
	}
}

func (h *keyIndex) put(a *mem.Allocator, key []byte, index int, budget int64) bool {
	if index < 0 {
		return false
	}
	capacity := 0
	if h.slots != nil {
		capacity = h.slots.Len() / 16
	}
	if capacity == 0 || h.used+1 > capacity-capacity/4 {
		next := 16
		if capacity != 0 {
			if capacity > int(^uint(0)>>1)/32 {
				return false
			}
			next = capacity * 2
		}
		if h.controls != nil {
			h.controls.Dec()
			h.controls = nil
		}
		if int64(next)*16 > budget-h.bytes() {
			return false
		}
		newSlots, ok := allocOperatorSegment(a, h.scope, next*16)
		if !ok {
			return false
		}
		entries := newSlots.AsU64T()
		clear(entries)
		if h.slots != nil {
			old := h.slots.AsU64T()
			for i := range capacity {
				if old[2*i+1] == 0 {
					continue
				}
				hash := old[2*i]
				at := hash & uint64(next-1)
				for entries[2*at+1] != 0 {
					at = (at + 1) & uint64(next-1)
				}
				entries[2*at], entries[2*at+1] = hash, old[2*i+1]
			}
			h.slots.Dec()
		} else {
			h.seed = maphash.MakeSeed()
		}
		h.slots = newSlots
		capacity = next
		h.makeControls(a, budget)
	}
	slots := h.slots.AsU64T()
	hash := h.hash(key)
	at := hash & uint64(capacity-1)
	for slots[2*at+1] != 0 {
		at = (at + 1) & uint64(capacity-1)
	}
	slots[2*at], slots[2*at+1] = hash, uint64(index)+1
	if h.controls != nil {
		h.setControl(int(at), byte(hash>>57)+1)
	}
	h.used++
	return true
}

func (h *keyIndex) hash(key []byte) uint64 {
	// Single string keys keep their canonical encoding but hash only the payload.
	if h.stringKeys && key[0] != 0 {
		key = key[9:]
	}
	return maphash.Bytes(h.seed, key)
}

func (h *keyIndex) lookupString(text string, isNull bool, keys *keyArena) (int, bool) {
	if h.slots == nil {
		return 0, false
	}
	hash := maphash.String(h.seed, text)
	if isNull {
		hash = maphash.Bytes(h.seed, []byte{0})
	}
	if h.controls != nil {
		return h.lookupControlled(hash, nil, text, isNull, true, keys)
	}
	slots := h.slots.AsU64T()
	mask := uint64(len(slots)/2 - 1)
	for at := hash & mask; ; at = (at + 1) & mask {
		index := slots[2*at+1]
		if index == 0 {
			return 0, false
		}
		if slots[2*at] != hash {
			continue
		}
		key := keys.key(int(index - 1))
		if isNull && key[0] == 0 || !isNull && key[0] != 0 && string(key[9:]) == text {
			return int(index - 1), true
		}
	}
}

func (h *keyIndex) makeControls(a *mem.Allocator, budget int64) {
	capacity := h.slots.Len() / 16
	if !h.tryControls || capacity < keyControlMinCapacity || int64(capacity+keyControlGroup) > budget-h.bytes() {
		return
	}
	if h.scope != nil {
		h.controls, _ = h.scope.TryAllocSeg(capacity + keyControlGroup)
	} else {
		h.controls = a.AllocSeg(capacity + keyControlGroup)
	}
	if h.controls == nil {
		return
	}
	clear(h.controls.AsBytes())
	slots := h.slots.AsU64T()
	for at := range capacity {
		if slots[2*at+1] != 0 {
			h.setControl(at, byte(slots[2*at]>>57)+1)
		}
	}
}

func (h *keyIndex) setControl(at int, fingerprint byte) {
	controls := h.controls.AsBytes()
	controls[at] = fingerprint
	if at < keyControlGroup {
		controls[h.slots.Len()/16+at] = fingerprint
	}
}

func (h *keyIndex) lookupControlled(hash uint64, encoded []byte, text string, isNull, stringKey bool, keys *keyArena) (int, bool) {
	slots := h.slots.AsU64T()
	controls := h.controls.AsBytes()
	mask := uint64(len(slots)/2 - 1)
	fingerprint := byte(hash>>57) + 1
	for at := hash & mask; ; at = (at + keyControlGroup) & mask {
		matches, empties := keyControlMasks(controls[at:at+keyControlGroup], fingerprint)
		if empties != 0 {
			matches &= empties ^ (empties - 1)
		}
		for matches != 0 {
			offset := bits.TrailingZeros16(matches)
			matches &= matches - 1
			position := (at + uint64(offset)) & mask
			if slots[2*position] != hash {
				continue
			}
			index := int(slots[2*position+1] - 1)
			key := keys.key(index)
			if stringKey {
				if isNull && key[0] == 0 || !isNull && key[0] != 0 && string(key[9:]) == text {
					return index, true
				}
			} else if bytes.Equal(key, encoded) {
				return index, true
			}
		}
		if empties != 0 {
			return 0, false
		}
	}
}
