package plan

import (
	"bytes"
	"hash/maphash"

	"github.com/rhawrami/peGosus/pkg/mem"
)

type keyIndex struct {
	slots      *mem.Segment
	scope      *mem.AllocationScope
	seed       maphash.Seed
	used       int
	stringKeys bool
}

func (h *keyIndex) release() {
	if h.slots != nil {
		h.slots.Dec()
	}
	*h = keyIndex{}
}

func (h *keyIndex) bytes() int64 {
	if h.slots == nil {
		return 0
	}
	return int64(h.slots.Len())
}

func (h *keyIndex) lookup(key []byte, keys *keyArena) (int, bool) {
	if h.slots == nil {
		return 0, false
	}
	slots := h.slots.AsU64T()
	mask := uint64(len(slots)/2 - 1)
	hash := h.hash(key)
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
	}
	slots := h.slots.AsU64T()
	hash := h.hash(key)
	at := hash & uint64(capacity-1)
	for slots[2*at+1] != 0 {
		at = (at + 1) & uint64(capacity-1)
	}
	slots[2*at], slots[2*at+1] = hash, uint64(index)+1
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
