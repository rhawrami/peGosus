package plan

import (
	"github.com/rhawrami/peGosus/pkg/mem"
)

func (p *PhysicalPlan) sampleGroupCardinality(a *mem.Allocator, aggregateAt int) bool {
	if p.source.table == nil {
		return false
	}
	for _, step := range p.steps[:aggregateAt] {
		if step.operation != physicalPushedFilter {
			return false
		}
	}
	var offsets []int
	for _, program := range p.steps[aggregateAt].groupKeys {
		if len(program.roots) != 1 {
			return false
		}
		root := program.nodes[program.roots[0]]
		if root.kind != exprColumn || root.column < 0 || root.column >= p.source.table.NColumns() {
			return false
		}
		offsets = append(offsets, root.column)
	}
	if len(offsets) == 0 {
		return false
	}
	seen := make(map[string]struct{}, 256)
	var buffer *mem.Segment
	defer func() {
		if buffer != nil {
			buffer.Dec()
		}
	}()
	samples := 0
	for part := range 4 {
		batchIndex := (p.source.table.NBatches() - 1) * part / 3
		batch := p.source.table.BatchAt(batchIndex).Project(offsets)
		if batch == nil {
			return false
		}
		visit := func(row int) bool {
			length := encodedRowKeyLength(batch, row)
			if length <= 0 {
				return false
			}
			if buffer == nil || buffer.Len() < length {
				capacity := max(64, length)
				if buffer != nil && buffer.Len() <= int(^uint(0)>>1)/2 {
					capacity = max(capacity, buffer.Len()*2)
				}
				next := a.AllocSegTemp(capacity)
				if next == nil {
					return false
				}
				if buffer != nil {
					buffer.Dec()
				}
				buffer = next
			}
			n := encodeRowKey(buffer.AsBytes()[:length], batch, row)
			seen[string(buffer.AsBytes()[:n])] = struct{}{}
			samples++
			return true
		}
		selection := batch.Selection()
		if offsets, ok := selection.AsSelVec(); ok {
			rows := offsets.Offsets()
			for i := 0; i < len(rows); i += max(1, len(rows)/64) {
				if !visit(int(rows[i])) {
					batch.Release()
					return false
				}
			}
		} else {
			bitmap, _ := selection.AsBitMap()
			for row := 0; row < batch.Len(); row += max(1, batch.Len()/64) {
				if bitmap != nil && !bitmap.IsSet(row) {
					continue
				}
				if !visit(row) {
					batch.Release()
					return false
				}
			}
		}
		batch.Release()
	}
	return samples >= 64 && len(seen)*4 <= samples
}
