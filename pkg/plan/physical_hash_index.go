package plan

import "github.com/rhawrami/peGosus/pkg/mem"

type u64Index struct {
	slots *mem.Segment
	used  int
	scope *mem.AllocationScope
}

func (h *u64Index) release() {
	if h.slots != nil {
		h.slots.Dec()
	}
	*h = u64Index{}
}

func (h *u64Index) bytes() int64 {
	if h.slots == nil {
		return 0
	}
	return int64(h.slots.Len())
}

func (h *u64Index) get(key uint64) (int, bool) {
	if h.slots == nil {
		return 0, false
	}
	slots := h.slots.AsU64T()
	capacity := len(slots) / 2
	mask := uint64(capacity - 1)
	for at := hashU64(key) & mask; ; at = (at + 1) & mask {
		id := slots[capacity+int(at)]
		if id == 0 {
			return 0, false
		}
		if slots[at] == key {
			return int(id - 1), true
		}
	}
}

func (h *u64Index) put(a *mem.Allocator, key uint64, id int, budget int64) bool {
	if id < 0 {
		return false
	}
	if _, found := h.get(key); found {
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
		var newSlots *mem.Segment
		if h.scope != nil {
			var ok bool
			newSlots, ok = h.scope.AllocSeg(next * 16)
			if !ok {
				return false
			}
		} else {
			newSlots = a.AllocSeg(next * 16)
		}
		entries := newSlots.AsU64T()
		clear(entries[next:])
		if h.slots != nil {
			old := h.slots.AsU64T()
			for i := range capacity {
				if old[capacity+i] == 0 {
					continue
				}
				k := old[i]
				at := hashU64(k) & uint64(next-1)
				for entries[next+int(at)] != 0 {
					at = (at + 1) & uint64(next-1)
				}
				entries[at] = k
				entries[next+int(at)] = old[capacity+i]
			}
			h.slots.Dec()
		}
		h.slots = newSlots
		capacity = next
	}
	slots := h.slots.AsU64T()
	at := hashU64(key) & uint64(capacity-1)
	for slots[capacity+int(at)] != 0 {
		at = (at + 1) & uint64(capacity-1)
	}
	slots[at] = key
	slots[capacity+int(at)] = uint64(id) + 1
	h.used++
	return true
}

func hashU64(key uint64) uint64 {
	key ^= key >> 30
	key *= 0xbf58476d1ce4e5b9
	key ^= key >> 27
	key *= 0x94d049bb133111eb
	return key ^ (key >> 31)
}
