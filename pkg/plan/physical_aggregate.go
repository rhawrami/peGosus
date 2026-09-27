package plan

import (
	"math"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type physicalAggregateExpr struct {
	kind     AggregateKind
	program  physicalExprProgram
	distinct bool
}

type aggregateValue struct {
	value Scalar
	count int64
	text  *mem.Segment
}

func (s *aggregateValue) release() {
	if s.text != nil {
		s.text.Dec()
		s.text = nil
	}
}

func (s *aggregateValue) add(a *mem.Allocator, kind AggregateKind, value Scalar) {
	if kind == AggregateCountStar || kind == AggregateCount {
		if kind == AggregateCountStar || !value.IsNull() {
			s.count++
		}
		return
	}
	if value.IsNull() {
		return
	}
	if kind == AggregateMin || kind == AggregateMax {
		if value.Type().ID() == dtype.FLOAT32T && math.IsNaN(float64(value.f32())) || value.Type().ID() == dtype.FLOAT64T && math.IsNaN(value.f64()) {
			return
		}
		if s.count == 0 || aggregateLess(value, s.value, kind == AggregateMax) {
			s.release()
			if value.Type().ID() == dtype.STRT {
				s.text = a.AllocSeg(len(value.text))
				copy(s.text.AsBytes(), value.text)
				value.text = unsafe.String(unsafe.SliceData(s.text.AsBytes()), s.text.Len())
			}
			s.value = value
		}
		s.count++
		return
	}
	if s.count == 0 {
		if kind == AggregateAvg || value.Type().ID() == dtype.FLOAT32T {
			s.value = MakeF64Scalar(0)
		} else if value.Type().ID() == dtype.INT32T {
			s.value = MakeI64Scalar(0)
		} else {
			s.value = value
		}
	}
	if kind == AggregateSum || kind == AggregateAvg {
		switch value.Type().ID() {
		case dtype.INT32T:
			if kind == AggregateAvg {
				s.value = MakeF64Scalar(s.value.f64() + float64(value.i32()))
			} else {
				s.value = MakeI64Scalar(s.value.i64() + int64(value.i32()))
			}
		case dtype.INT64T:
			if kind == AggregateAvg {
				s.value = MakeF64Scalar(s.value.f64() + float64(value.i64()))
			} else if s.count == 0 {
				s.value = value
			} else {
				s.value = MakeI64Scalar(s.value.i64() + value.i64())
			}
		case dtype.FLOAT32T:
			s.value = MakeF64Scalar(s.value.f64() + float64(value.f32()))
		case dtype.FLOAT64T:
			if s.count == 0 && kind == AggregateSum {
				s.value = value
			} else {
				s.value = MakeF64Scalar(s.value.f64() + value.f64())
			}
		}
		s.count++
	}
}

func aggregateLess(x, y Scalar, max bool) bool {
	switch x.Type().ID() {
	case dtype.INT32T, dtype.DATET:
		if max {
			return x.i32() > y.i32()
		}
		return x.i32() < y.i32()
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		if max {
			return x.i64() > y.i64()
		}
		return x.i64() < y.i64()
	case dtype.FLOAT32T:
		a, b := x.f32(), y.f32()
		if a == 0 && b == 0 {
			if max {
				return !math.Signbit(float64(a)) && math.Signbit(float64(b))
			}
			return math.Signbit(float64(a)) && !math.Signbit(float64(b))
		}
		if max {
			return a > b
		}
		return a < b
	case dtype.FLOAT64T:
		a, b := x.f64(), y.f64()
		if a == 0 && b == 0 {
			if max {
				return !math.Signbit(a) && math.Signbit(b)
			}
			return math.Signbit(a) && !math.Signbit(b)
		}
		if max {
			return a > b
		}
		return a < b
	case dtype.STRT:
		if max {
			return x.text > y.text
		}
		return x.text < y.text
	}
	return false
}

func scalarAt(v *store.Vector, row int) Scalar {
	if v.Validity() != nil && !v.Validity().IsSet(row) {
		return MakeNullScalar(v.Type())
	}
	switch v.TypeID() {
	case dtype.INT32T:
		return MakeI32Scalar(v.I32s()[row])
	case dtype.INT64T:
		return MakeI64Scalar(v.I64s()[row])
	case dtype.FLOAT32T:
		return MakeF32Scalar(v.F32s()[row])
	case dtype.FLOAT64T:
		return MakeF64Scalar(v.F64s()[row])
	case dtype.DATET:
		return MakeDateScalar(v.I32s()[row])
	case dtype.TIMESTAMPTZT:
		return MakeTimestampTZScalar(v.I64s()[row])
	case dtype.BOOLT:
		return MakeBoolScalar(v.Bools()[row>>3]&(1<<(row&7)) != 0)
	case dtype.STRT:
		return MakeStringScalar(v.Strings()[row].View())
	}
	return Scalar{}
}

func accumulateAggregates(a *mem.Allocator, batch *store.Batch, step physicalStep, merged []aggregateValue, uniques []*distinctState, budget int64) bool {
	local := make([]aggregateValue, len(step.aggregates))
	defer func() {
		for i := range local {
			local[i].release()
		}
	}()
	selection := batch.Selection().MakeBitMapTemp(a)
	if selection != nil {
		defer selection.Release()
	}
	for i, aggregate := range step.aggregates {
		var vector *store.Vector
		var values physicalExprValues
		var one *store.Batch
		if aggregate.kind != AggregateCountStar {
			var ok bool
			values, ok = executePhysicalExprProgram(a, batch, aggregate.program)
			if !ok {
				return false
			}
			vector = &values.vectors[aggregate.program.roots[0]]
			if aggregate.distinct {
				one = store.MakeBatchRetained([]store.Vector{*vector})
			}
		}
		for row := range batch.Len() {
			if selection != nil && !selection.IsSet(row) {
				continue
			}
			if aggregate.distinct {
				if vector.Validity() != nil && !vector.Validity().IsSet(row) {
					continue
				}
				before := len(uniques[i].rows)
				_, ok := uniques[i].add(a, one, row, budget+uniques[i].charged)
				if !ok {
					one.Release()
					values.release()
					return false
				}
				if len(uniques[i].rows) == before {
					continue
				}
			}
			if vector == nil {
				local[i].add(a, aggregate.kind, Scalar{})
			} else {
				local[i].add(a, aggregate.kind, scalarAt(vector, row))
			}
		}
		if one != nil {
			one.Release()
		}
		values.release()
	}
	for i, aggregate := range step.aggregates {
		mergeAggregate(&merged[i], &local[i], aggregate.kind)
	}
	return true
}

func mergeAggregate(dst, src *aggregateValue, kind AggregateKind) {
	if src.count == 0 {
		return
	}
	if kind == AggregateCount || kind == AggregateCountStar {
		dst.count += src.count
	} else if kind == AggregateMin || kind == AggregateMax {
		if dst.count == 0 || aggregateLess(src.value, dst.value, kind == AggregateMax) {
			dst.release()
			dst.value = src.value
			dst.text = src.text
			src.text = nil
		}
		dst.count += src.count
	} else {
		if dst.count == 0 {
			dst.value = src.value
		} else if dst.value.Type().ID() == dtype.INT64T {
			dst.value = MakeI64Scalar(dst.value.i64() + src.value.i64())
		} else {
			dst.value = MakeF64Scalar(dst.value.f64() + src.value.f64())
		}
		dst.count += src.count
	}
	src.release()
}

func finalizeAggregates(a *mem.Allocator, step physicalStep, merged []aggregateValue) *store.Batch {
	vectors := make([]store.Vector, len(merged))
	for i, state := range merged {
		field := step.schema.FieldAt(i)
		if field.Type().ID() == dtype.STRT {
			var valid []bool
			if state.count == 0 {
				valid = []bool{false}
			}
			vectors[i] = store.MakeStringVector(a, [][]byte{borrowedStringBytes(state.value.text)}, valid)
		} else {
			vectors[i] = store.MakeVector(a, 1, field.Type(), field.Nullable())
			if vectors[i].Kind() == store.VectorInvalid {
				break
			}
			if state.count == 0 && field.Nullable() {
				vectors[i].Validity().Clear(0)
				continue
			}
			switch field.Type().ID() {
			case dtype.INT64T:
				if step.aggregates[i].kind == AggregateCount || step.aggregates[i].kind == AggregateCountStar {
					vectors[i].I64s()[0] = state.count
				} else {
					vectors[i].I64s()[0] = state.value.i64()
				}
			case dtype.FLOAT64T:
				if step.aggregates[i].kind == AggregateAvg {
					vectors[i].F64s()[0] = state.value.f64() / float64(state.count)
				} else {
					vectors[i].F64s()[0] = state.value.f64()
				}
			case dtype.INT32T, dtype.DATET:
				vectors[i].I32s()[0] = state.value.i32()
			case dtype.FLOAT32T:
				vectors[i].F32s()[0] = state.value.f32()
			}
		}
	}
	batch := store.MakeBatch(vectors)
	if batch == nil {
		for i := range vectors {
			vectors[i].Release()
		}
	}
	return batch
}
