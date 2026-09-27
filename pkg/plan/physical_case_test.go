package plan

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalCaseRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(267, 101))
	for _, length := range []int{0, 1, 7, 8, 9, 17, 129} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
		condition := makePhysicalBoolVector(a, make([]int8, length))
		left := store.MakeVector(a, length, dtype.Float64T(), true)
		right := store.MakeVector(a, length, dtype.Float64T(), true)
		selected := make([]bool, length)
		want := make([]float64, length)
		valid := make([]bool, length)
		mask := store.MakeBitMap(a, length)
		for row := range length {
			state := int8(rng.IntN(3)) - 1
			if state < 0 {
				condition.Validity().Clear(row)
			} else if state > 0 {
				condition.Bools()[row>>3] |= 1 << (row & 7)
			}
			left.F64s()[row], right.F64s()[row] = rng.Float64()*100, rng.Float64()*100
			if rng.IntN(7) == 0 {
				left.F64s()[row] = math.NaN()
			}
			if rng.IntN(7) == 0 {
				right.F64s()[row] = math.Inf(1)
			}
			if rng.IntN(5) == 0 {
				left.Validity().Clear(row)
			}
			if rng.IntN(5) == 0 {
				right.Validity().Clear(row)
			}
			if rng.IntN(3) != 0 {
				mask.Set(row)
				selected[row] = true
			}
			if state == 1 {
				want[row] = left.F64s()[row] + 3
				valid[row] = left.Validity().IsSet(row)
			} else {
				want[row] = 4 - right.F64s()[row]
				valid[row] = right.Validity().IsSet(row)
			}
		}
		batch := store.MakeBatch([]store.Vector{condition, left, right})
		batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
		table := store.MakeTable([]*store.Batch{batch})
		batch.Release()
		schema := MakeSchema([]string{"condition", "left", "right"}, []dtype.Type{dtype.BoolT(), dtype.Float64T(), dtype.Float64T()})
		physical, err := MakePhysicalPlan(MakeScan(table, schema).Project(
			MakeCase(MakeColumn("condition"), MakeColumn("left").Add(3), MakeLiteral(4).Sub(MakeColumn("right"))),
		))
		if err != nil {
			t.Fatalf("length %d: lower CASE: %v", length, err)
		}
		table.Release()
		if !physical.Execute(a, func(output *store.Batch) bool {
			if output.Len() != length {
				t.Fatalf("length %d: physical domain changed", length)
			}
			for row := range length {
				v := output.VectorAt(0)
				bitmap, ok := output.Selection().AsBitMap()
				if !ok || bitmap.IsSet(row) != selected[row] {
					t.Fatalf("length %d row %d: selection changed", length, row)
				}
				if v.Validity().IsSet(row) != valid[row] {
					t.Fatalf("length %d row %d: wrong CASE validity", length, row)
				}
				if valid[row] && v.F64s()[row] != want[row] && !(math.IsNaN(v.F64s()[row]) && math.IsNaN(want[row])) {
					t.Fatalf("length %d row %d: got %v, expected %v", length, row, v.F64s()[row], want[row])
				}
			}
			return true
		}) {
			t.Fatalf("length %d: CASE execution failed", length)
		}
		physical.Release()
	}
}
