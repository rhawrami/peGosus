package plan

import (
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkLowCardinalityStringGroupLookup(b *testing.B) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	stringsIn := make([][]byte, 4096)
	values := store.MakeVector(a, len(stringsIn), dtype.Int32T(), false)
	for row := range stringsIn {
		stringsIn[row] = []byte([]string{"north", "south", "east", "west", "ø", ""}[row%6])
		values.I32s()[row] = int32(row)
	}
	batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, stringsIn, nil), values})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{false, false})).GroupBy([]Expr{MakeColumn("key")}, MakeSum(MakeColumn("value"))))
	if err != nil {
		b.Fatal(err)
	}
	defer p.Release()
	defer table.Release()
	defer batch.Release()
	for _, fast := range []bool{true, false} {
		name := "encoded"
		if fast {
			name = "cached"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(stringsIn)))
			b.ResetTimer()
			for range b.N {
				g := makeGroupState(p.steps[0], nil)
				if !fast {
					g.stringGroups = nil
				}
				if !g.add(a, batch, p.steps[0], math.MaxInt64) || len(g.values) != 6 {
					b.Fatal("incorrect groups")
				}
				g.release()
			}
		})
	}
}
