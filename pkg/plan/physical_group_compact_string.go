package plan

import (
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (g *compactGroupState) prepareStringGroup(a *mem.Allocator, index int, budget int64) bool {
	if index < g.length {
		return true
	}
	if index != g.length || g.length == g.capacity && !g.grow(a, budget) {
		return false
	}
	clear(g.counts.AsI64T()[index*g.aggregateCount : (index+1)*g.aggregateCount])
	if g.sums != nil {
		clear(g.sums.AsU64T()[index*g.aggregateCount : (index+1)*g.aggregateCount])
	}
	g.length++
	return true
}

func (g *compactGroupState) stringGroupIDs(a *mem.Allocator, key *store.Vector, selection *store.BitMap, ids, mapping []int32, budget int64) bool {
	keyBatch := store.MakeBatchRetained([]store.Vector{*key})
	defer keyBatch.Release()
	var dictionaryBatch *store.Batch
	if dictionary := key.Dictionary(); dictionary != nil && len(mapping) != 0 {
		dictionaryBatch = store.MakeBatchRetained([]store.Vector{*dictionary})
		defer dictionaryBatch.Release()
		for i := range mapping {
			mapping[i] = -1
		}
	}
	indices := key.DictionaryIndices()
	nullIndex := -1
	for row := range ids {
		ids[row] = -1
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		isNull := key.Validity() != nil && !key.Validity().IsSet(row)
		index := -1
		source, sourceRow := keyBatch, row
		if isNull {
			index = nullIndex
		} else if dictionaryBatch != nil {
			source, sourceRow = dictionaryBatch, int(indices[row])
			index = int(mapping[sourceRow])
		}
		if index < 0 {
			var ok bool
			index, ok = g.strings.addString(a, source, sourceRow, budget-g.charged()+g.strings.charged)
			if !ok || !g.prepareStringGroup(a, index, budget) {
				return false
			}
			if isNull {
				nullIndex = index
			} else if dictionaryBatch != nil {
				mapping[sourceRow] = int32(index)
			}
		}
		ids[row] = int32(index)
	}
	return true
}
