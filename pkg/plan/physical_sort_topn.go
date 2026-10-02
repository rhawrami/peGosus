package plan

import (
	"math"
	"sort"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const compactTopNItemBytes = int(unsafe.Sizeof(compactTopNItem{}))

func makeCompactTopNState(step physicalStep) *compactTopNState {
	if !step.hasTopN || len(step.order) == 0 || len(step.order) > 2 || step.topN > int64(int(^uint(0)>>1)) {
		return nil
	}
	for _, order := range step.order {
		keyType := order.program.nodes[order.program.roots[0]].dType.ID()
		switch keyType {
		case dtype.INT32T, dtype.INT64T, dtype.FLOAT32T, dtype.FLOAT64T, dtype.DATET, dtype.TIMESTAMPTZT:
		default:
			return nil
		}
	}
	for _, field := range step.schema.fields {
		if !field.Type().IsFixedSize() {
			return nil
		}
	}
	state := &compactTopNState{limit: int(step.topN), keyCount: len(step.order)}
	for i, order := range step.order {
		state.nullsFirst[i] = order.nullsFirst
	}
	return state
}

type compactTopNItem struct {
	key      uint64
	key2     uint64
	row      uint64
	null     uint64
	null2    uint64
	sequence uint64
}

type compactTopNState struct {
	items      *mem.Segment
	scope      *mem.AllocationScope
	rows       packedRows
	limit      int
	nullsFirst [2]bool
	keyCount   int
	length     int
	capacity   int
	next       uint64
}

func (s *compactTopNState) release() {
	if s.items != nil {
		s.items.Dec()
	}
	s.rows.release()
	*s = compactTopNState{}
}

func (s *compactTopNState) charged() int64 {
	return saturatingAdd(int64(s.capacity)*int64(compactTopNItemBytes), s.rows.bytes())
}

func (s *compactTopNState) slice() []compactTopNItem {
	if s.items == nil {
		return nil
	}
	return unsafe.Slice((*compactTopNItem)(unsafe.Pointer(unsafe.SliceData(s.items.AsBytes()))), s.capacity)
}

func (s *compactTopNState) grow(a *mem.Allocator, budget int64) bool {
	capacity := min(max(16, s.capacity*2), s.limit)
	if capacity <= s.capacity || capacity > int(^uint(0)>>1)/compactTopNItemBytes || int64(capacity)*int64(compactTopNItemBytes) > budget-s.charged() {
		return false
	}
	items, ok := allocOperatorSegment(a, s.scope, capacity*compactTopNItemBytes)
	if !ok {
		return false
	}
	if s.items != nil {
		copy(items.AsBytes(), s.items.AsBytes()[:s.length*compactTopNItemBytes])
		s.items.Dec()
	}
	s.items, s.capacity = items, capacity
	return true
}

func (s *compactTopNState) compare(left, right compactTopNItem) int {
	for i := range s.keyCount {
		l, r := left.key, right.key
		ln, rn := left.null, right.null
		if i == 1 {
			l, r = left.key2, right.key2
			ln, rn = left.null2, right.null2
		}
		if ln != rn {
			if (ln != 0) == s.nullsFirst[i] {
				return -1
			}
			return 1
		}
		if ln != 0 {
			continue
		}
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	}
	if left.sequence < right.sequence {
		return -1
	}
	if left.sequence > right.sequence {
		return 1
	}
	return 0
}

func (s *compactTopNState) siftUp(index int) {
	items := s.slice()
	for index > 0 {
		parent := (index - 1) / 2
		if s.compare(items[index], items[parent]) <= 0 {
			return
		}
		items[index], items[parent] = items[parent], items[index]
		index = parent
	}
}

func (s *compactTopNState) siftDown(index int) {
	items := s.slice()
	for {
		child := 2*index + 1
		if child >= s.length {
			return
		}
		if child+1 < s.length && s.compare(items[child+1], items[child]) > 0 {
			child++
		}
		if s.compare(items[child], items[index]) <= 0 {
			return
		}
		items[index], items[child] = items[child], items[index]
		index = child
	}
}

func (s *compactTopNState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
	if s.limit == 0 {
		return true
	}
	var values [2]physicalExprValues
	var keys [2]*store.Vector
	defer func() {
		for i := range s.keyCount {
			values[i].release()
		}
	}()
	for i := range s.keyCount {
		var ok bool
		values[i], ok = executePhysicalExprProgram(a, batch, step.order[i].program)
		if !ok {
			return false
		}
		keys[i] = &values[i].vectors[step.order[i].program.roots[0]]
	}
	selection := batch.Selection().RetainBitMap(a)
	if batch.Selection() != nil && selection == nil {
		return false
	}
	if selection != nil {
		defer selection.Release()
	}
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		encoded, null := encodeNumericOrderKey(keys[0], row, step.order[0].descending)
		item := compactTopNItem{key: encoded, null: null, sequence: s.next}
		if s.keyCount == 2 {
			item.key2, item.null2 = encodeNumericOrderKey(keys[1], row, step.order[1].descending)
		}
		s.next++
		if s.length == s.limit {
			worst := s.slice()[0]
			if s.compare(item, worst) >= 0 {
				continue
			}
			item.row = worst.row
			if !s.rows.setFixed(batch, row, int(item.row)) {
				return false
			}
			s.slice()[0] = item
			s.siftDown(0)
			continue
		}
		if s.length == s.capacity && !s.grow(a, budget) {
			return false
		}
		item.row = uint64(s.rows.length)
		if !s.rows.append(a, batch, row, budget-int64(s.capacity)*int64(compactTopNItemBytes)) {
			return false
		}
		s.slice()[s.length] = item
		s.length++
		s.siftUp(s.length - 1)
	}
	return true
}

func (s *compactTopNState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	if s.length == 0 {
		return nil
	}
	items := s.slice()[:s.length]
	sort.Slice(items, func(i, j int) bool { return s.compare(items[i], items[j]) < 0 })
	rows := &packedRows{}
	defer rows.release()
	values := make([]Scalar, len(s.rows.types))
	for _, item := range items {
		for column := range values {
			values[column] = s.rows.at(int(item.row), column)
		}
		if !rows.appendScalars(a, values, math.MaxInt64) {
			return nil
		}
	}
	return rows.makeBatch(a, step.schema)
}
