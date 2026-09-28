package plan

import (
	"bytes"
	"hash/maphash"

	"github.com/rhawrami/peGosus/pkg/mem"
)

type keyIndex struct {
	slots *mem.Segment
	scope *mem.AllocationScope
	seed  maphash.Seed
	used  int
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
	hash := maphash.Bytes(h.seed, key)
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
	hash := maphash.Bytes(h.seed, key)
	at := hash & uint64(capacity-1)
	for slots[2*at+1] != 0 {
		at = (at + 1) & uint64(capacity-1)
	}
	slots[2*at], slots[2*at+1] = hash, uint64(index)+1
	h.used++
	return true
}
