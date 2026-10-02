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
	dictionary := store.MakeStringVector(a, [][]byte{[]byte("north"), []byte("south"), []byte("east"), []byte("west"), []byte("ø"), []byte("")}, nil)
	indices := a.AllocSeg(len(stringsIn) * 4)
	for row := range stringsIn {
		indices.AsU32T()[row] = uint32(row % 6)
	}
	dictionaryBatch := store.MakeBatch([]store.Vector{store.MakeDictionaryStringVectorFromOwnedSegments(dictionary, len(stringsIn), indices, nil), values.Retain()})
	defer dictionaryBatch.Release()
	for _, mode := range []string{"typed", "dictionary", "direct", "encoded"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(stringsIn)))
			b.ResetTimer()
			for range b.N {
				g := makeGroupState(p.steps[0], nil)
				if mode == "direct" || mode == "encoded" {
					g.compact = nil
					g.keys.index.stringKeys = mode == "direct"
				}
				input := batch
				if mode == "dictionary" {
					input = dictionaryBatch
				}
				if !g.add(a, input, p.steps[0], math.MaxInt64) || g.groupCount() != 6 {
					b.Fatal("incorrect groups")
				}
				g.release()
			}
		})
	}
}
