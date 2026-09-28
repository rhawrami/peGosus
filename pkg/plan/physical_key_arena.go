package plan

import "github.com/rhawrami/peGosus/pkg/mem"

type keyArena struct {
	data    *mem.Segment
	refs    *mem.Segment
	scope   *mem.AllocationScope
	used    int
	entries int
}

func (k *keyArena) release() {
	if k.data != nil {
		k.data.Dec()
	}
	if k.refs != nil {
		k.refs.Dec()
	}
	*k = keyArena{}
}

func (k *keyArena) bytes() int64 {
	var n int64
	if k.data != nil {
		n += int64(k.data.Len())
	}
	if k.refs != nil {
		n += int64(k.refs.Len())
	}
	return n
}

func (k *keyArena) key(index int) []byte {
	refs := k.refs.AsU64T()
	on, length := int(refs[2*index]), int(refs[2*index+1])
	return k.data.AsBytes()[on : on+length]
}

func (k *keyArena) append(a *mem.Allocator, value []byte, budget int64) bool {
	maxInt := int(^uint(0) >> 1)
	if len(value) > maxInt-k.used {
		return false
	}
	dataCap, refsCap := 0, 0
	if k.data != nil {
		dataCap = k.data.Len()
		refsCap = k.refs.Len() / 16
	}
	newDataCap, newRefsCap := dataCap, refsCap
	if newDataCap == 0 {
		newDataCap = 64
	}
	for newDataCap < k.used+len(value) {
		if newDataCap > maxInt/2 {
			return false
		}
		newDataCap *= 2
	}
	if newRefsCap == 0 {
		newRefsCap = 4
	}
	if k.entries == newRefsCap {
		if newRefsCap > maxInt/32 {
			return false
		}
		newRefsCap *= 2
	}
	if newRefsCap > maxInt/16 || int64(newDataCap)+int64(dataCap)+int64(refsCap)*16 > budget && newDataCap != dataCap || int64(newDataCap)+int64(newRefsCap+refsCap)*16 > budget && newRefsCap != refsCap || int64(newDataCap)+int64(newRefsCap)*16 > budget {
		return false
	}
	if newDataCap != dataCap {
		data, ok := allocOperatorSegment(a, k.scope, newDataCap)
		if !ok {
			return false
		}
		if k.data != nil {
			copy(data.AsBytes(), k.data.AsBytes()[:k.used])
			k.data.Dec()
		}
		k.data = data
	}
	if newRefsCap != refsCap {
		refs, ok := allocOperatorSegment(a, k.scope, newRefsCap*16)
		if !ok {
			return false
		}
		if k.refs != nil {
			copy(refs.AsBytes(), k.refs.AsBytes()[:k.entries*16])
			k.refs.Dec()
		}
		k.refs = refs
	}
	copy(k.data.AsBytes()[k.used:], value)
	references := k.refs.AsU64T()
	references[2*k.entries], references[2*k.entries+1] = uint64(k.used), uint64(len(value))
	k.entries++
	k.used += len(value)
	return true
}
