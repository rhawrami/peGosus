package plan

import (
	"math"
	"sort"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type sortedRow struct {
	record   distinctRow
	keys     []Scalar
	text     []*mem.Segment
	sequence int64
	charged  int64
}

type sortState struct {
	rows    []sortedRow
	charged int64
	next    int64
	order   []physicalOrderKey
}

func (row *sortedRow) release() {
	for _, text := range row.record.text {
		if text != nil {
			text.Dec()
		}
	}
	for _, text := range row.text {
		if text != nil {
			text.Dec()
		}
	}
}

func (s *sortState) release() {
	for i := range s.rows {
		s.rows[i].release()
	}
}

func (s *sortState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
	if step.hasTopN && step.topN == 0 {
		return true
	}
	s.order = step.order
	vectors := make([]*store.Vector, len(step.order))
	computed := make([]physicalExprValues, len(step.order))
	defer func() {
		for i := range computed {
			computed[i].release()
		}
	}()
	for i, key := range step.order {
		var ok bool
		computed[i], ok = executePhysicalExprProgram(a, batch, key.program)
		if !ok {
			return false
		}
		vectors[i] = &computed[i].vectors[key.program.roots[0]]
	}
	selection := batch.Selection().MakeBitMapTemp(a)
	if selection != nil {
		defer selection.Release()
	}
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		keys := make([]Scalar, len(vectors))
		for i, vector := range vectors {
			keys[i] = scalarAt(vector, row)
		}
		candidate := sortedRow{keys: keys, sequence: s.next}
		s.next++
		if step.hasTopN && int64(len(s.rows)) >= step.topN && s.compare(candidate, s.rows[0]) >= 0 {
			continue
		}
		charge := int64((batch.NVectors() + len(vectors)) * 64)
		for column := range batch.NVectors() {
			v := batch.VectorAt(column)
			if v.TypeID() == dtype.STRT && (v.Validity() == nil || v.Validity().IsSet(row)) {
				charge = saturatingAdd(charge, int64(len(v.Strings()[row].View())))
			}
		}
		for _, vector := range vectors {
			if vector.TypeID() == dtype.STRT && (vector.Validity() == nil || vector.Validity().IsSet(row)) {
				charge = saturatingAdd(charge, int64(len(vector.Strings()[row].View())))
			}
		}
		evict := int64(0)
		if step.hasTopN && int64(len(s.rows)) >= step.topN {
			evict = s.rows[0].charged
		}
		if charge > budget-s.charged+evict {
			return false
		}
		entry := sortedRow{record: distinctRow{values: make([]Scalar, batch.NVectors()), text: make([]*mem.Segment, batch.NVectors())}, keys: make([]Scalar, len(vectors)), text: make([]*mem.Segment, len(vectors)), sequence: candidate.sequence, charged: charge}
		for column := range entry.record.values {
			entry.record.values[column], entry.record.text[column] = ownScalar(a, scalarAt(batch.VectorAt(column), row))
		}
		for i, vector := range vectors {
			entry.keys[i], entry.text[i] = ownScalar(a, scalarAt(vector, row))
		}
		if evict != 0 {
			s.rows[0].release()
			s.rows[0] = entry
			s.charged += charge - evict
			s.siftDown(0)
		} else {
			s.rows = append(s.rows, entry)
			s.charged += charge
			if step.hasTopN {
				s.siftUp(len(s.rows) - 1)
			}
		}
	}
	return true
}

func (s *sortState) compare(x, y sortedRow) int {
	for k, key := range s.order {
		a, b := x.keys[k], y.keys[k]
		if a.IsNull() || b.IsNull() {
			if a.IsNull() != b.IsNull() {
				if a.IsNull() == key.nullsFirst {
					return -1
				}
				return 1
			}
			continue
		}
		cmp := compareOrdered(a, b)
		if cmp != 0 {
			if key.descending {
				return -cmp
			}
			return cmp
		}
	}
	if x.sequence < y.sequence {
		return -1
	}
	if x.sequence > y.sequence {
		return 1
	}
	return 0
}

func (s *sortState) siftUp(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if s.compare(s.rows[index], s.rows[parent]) <= 0 {
			break
		}
		s.rows[index], s.rows[parent] = s.rows[parent], s.rows[index]
		index = parent
	}
}

func (s *sortState) siftDown(index int) {
	for {
		child := 2*index + 1
		if child >= len(s.rows) {
			return
		}
		if child+1 < len(s.rows) && s.compare(s.rows[child+1], s.rows[child]) > 0 {
			child++
		}
		if s.compare(s.rows[child], s.rows[index]) <= 0 {
			return
		}
		s.rows[index], s.rows[child] = s.rows[child], s.rows[index]
		index = child
	}
}

func compareOrdered(x, y Scalar) int {
	switch x.Type().ID() {
	case dtype.INT32T, dtype.DATET:
		if x.i32() < y.i32() {
			return -1
		}
		if x.i32() > y.i32() {
			return 1
		}
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		if x.i64() < y.i64() {
			return -1
		}
		if x.i64() > y.i64() {
			return 1
		}
	case dtype.FLOAT32T:
		a, b := x.f32(), y.f32()
		if math.IsNaN(float64(a)) {
			if !math.IsNaN(float64(b)) {
				return 1
			}
			return 0
		}
		if math.IsNaN(float64(b)) {
			return -1
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
	case dtype.FLOAT64T:
		a, b := x.f64(), y.f64()
		if math.IsNaN(a) {
			if !math.IsNaN(b) {
				return 1
			}
			return 0
		}
		if math.IsNaN(b) {
			return -1
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
	case dtype.STRT:
		return compareStrings(x.text, y.text)
	}
	return 0
}

func (s *sortState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	sort.SliceStable(s.rows, func(i, j int) bool {
		return s.compare(s.rows[i], s.rows[j]) < 0
	})
	rows := make([]distinctRow, len(s.rows))
	for i, row := range s.rows {
		rows[i] = row.record
	}
	return makeDistinctBatch(a, step.schema, rows)
}
