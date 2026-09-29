package plan

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalIntegerReductionKernelDifferential(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	rng := rand.New(rand.NewPCG(17, 23))
	for _, typ := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.DateT(), dtype.TimestampTZT()} {
		for _, length := range []int{0, 1, 7, 9, 71} {
			for _, selected := range []bool{false, true} {
				for _, allNull := range []bool{false, true} {
					vector := store.MakeVector(a, length, typ, true)
					for i := range length {
						if allNull || i%7 == 0 {
							vector.Validity().Clear(i)
						}
						if typ.ID() == dtype.INT32T || typ.ID() == dtype.DATET {
							vector.I32s()[i] = int32(rng.Uint32())
							if i%13 == 0 {
								vector.I32s()[i] = math.MinInt32
							}
							if i%11 == 0 {
								vector.I32s()[i] = math.MaxInt32
							}
						} else {
							vector.I64s()[i] = int64(rng.Uint64())
							if i%13 == 0 {
								vector.I64s()[i] = math.MinInt64
							}
							if i%11 == 0 {
								vector.I64s()[i] = math.MaxInt64
							}
						}
					}
					var mask *store.BitMap
					if selected {
						mask = store.MakeBitMapTemp(a, length)
						for i := range length {
							if i%3 != 0 {
								mask.Set(i)
							}
						}
					}
					for _, kind := range []AggregateKind{AggregateSum, AggregateMin, AggregateMax} {
						var got, want aggregateValue
						if !accumulateIntegerKernel(a, &vector, mask, kind, &got) {
							if kind == AggregateSum && typ.ID() != dtype.INT32T && typ.ID() != dtype.INT64T {
								continue
							}
							t.Fatalf("%s length %d kind %d: not dispatched", typ, length, kind)
						}
						for row := range length {
							if mask != nil && !mask.IsSet(row) {
								continue
							}
							if !want.add(a, kind, scalarAt(&vector, row)) {
								t.Fatal("scalar reduction failed")
							}
						}
						matches := got.count == want.count
						if got.count != 0 {
							matches = matches && got.value.Type().ID() == want.value.Type().ID()
							if kind != AggregateSum && (typ.ID() == dtype.INT32T || typ.ID() == dtype.DATET) {
								matches = matches && got.value.i32() == want.value.i32()
							} else {
								matches = matches && got.value.i64() == want.value.i64()
							}
						}
						if !matches {
							t.Fatalf("%s length %d selected %t allNull %t kind %d: got %+v want %+v", typ, length, selected, allNull, kind, got, want)
						}
					}
					if mask != nil {
						mask.Release()
					}
					vector.Release()
				}
			}
		}
	}
}

func TestPhysicalFloatReductionKernelDifferential(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	for _, typ := range []dtype.Type{dtype.Float32T(), dtype.Float64T()} {
		for _, length := range []int{0, 1, 7, 8, 9, 33, 128} {
			for _, mode := range []int{0, 1, 2, 3} {
				vector := store.MakeVector(a, length, typ, true)
				var selection *store.BitMap
				if mode%2 == 1 {
					selection = store.MakeBitMapTemp(a, length)
				}
				for row := range length {
					value := float64(row%13-6) / 2
					if mode == 2 {
						value = math.Copysign(0, -1)
					}
					if mode == 3 || mode == 0 && row%7 == 0 {
						value = math.NaN()
					}
					if row%11 == 0 && mode == 0 {
						vector.Validity().Clear(row)
					}
					if selection != nil && row%3 != 0 {
						selection.Set(row)
					}
					if typ.ID() == dtype.FLOAT32T {
						vector.F32s()[row] = float32(value)
					} else {
						vector.F64s()[row] = value
					}
				}
				for _, kind := range []AggregateKind{AggregateSum, AggregateAvg, AggregateMin, AggregateMax} {
					var got, want aggregateValue
					if !accumulateFloatKernel(a, &vector, selection, kind, &got) {
						t.Fatalf("%s length %d mode %d kind %d: missing dispatch", typ, length, mode, kind)
					}
					for row := range length {
						if selection != nil && !selection.IsSet(row) {
							continue
						}
						if !want.add(a, kind, scalarAt(&vector, row)) {
							t.Fatal("scalar reduction failed")
						}
					}
					if got.count != want.count {
						t.Fatalf("%s length %d mode %d kind %d: count %d want %d", typ, length, mode, kind, got.count, want.count)
					}
					if got.count == 0 {
						continue
					}
					if kind == AggregateSum || kind == AggregateAvg {
						if got.value.f64() != want.value.f64() {
							t.Fatalf("%s length %d mode %d kind %d: sum %v want %v", typ, length, mode, kind, got.value.f64(), want.value.f64())
						}
						if mode == 2 && (math.Signbit(got.value.f64()) || math.Signbit(want.value.f64())) {
							t.Fatal("SUM/AVG over negative zeros must use a positive zero accumulator")
						}
					} else if typ.ID() == dtype.FLOAT32T {
						if math.Float32bits(got.value.f32()) != math.Float32bits(want.value.f32()) {
							t.Fatalf("%s length %d mode %d kind %d: extremum %v want %v", typ, length, mode, kind, got.value.f32(), want.value.f32())
						}
					} else if math.Float64bits(got.value.f64()) != math.Float64bits(want.value.f64()) {
						t.Fatalf("%s length %d mode %d kind %d: extremum %v want %v", typ, length, mode, kind, got.value.f64(), want.value.f64())
					}
				}
				if selection != nil {
					selection.Release()
				}
				vector.Release()
			}
		}
	}
}
