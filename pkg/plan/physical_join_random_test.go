package plan

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalJoinRandomizedDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(277, 73))
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	type row struct {
		key           int32
		score         int32
		valid, active bool
	}
	leftRows, rightRows := make([]row, 37), make([]row, 29)
	makeTable := func(rows []row) *store.Table {
		var batches []*store.Batch
		for start := 0; start < len(rows); start += 8 {
			end := min(start+8, len(rows))
			keys := store.MakeVector(a, end-start, dtype.Int32T(), true)
			ids := store.MakeVector(a, end-start, dtype.Int32T(), false)
			scores := store.MakeVector(a, end-start, dtype.Int32T(), false)
			selection := store.MakeBitMap(a, end-start)
			for i, input := range rows[start:end] {
				keys.I32s()[i], ids.I32s()[i], scores.I32s()[i] = input.key, int32(start+i), input.score
				if !input.valid {
					keys.Validity().Clear(i)
				}
				if input.active {
					selection.Set(i)
				}
			}
			b := store.MakeBatch([]store.Vector{keys, ids, scores})
			b.SetSelection(store.MakeRowSelectionFromBitMap(selection))
			batches = append(batches, b)
		}
		table := store.MakeTable(batches)
		for _, b := range batches {
			b.Release()
		}
		return table
	}
	for _, rows := range [][]row{leftRows, rightRows} {
		for i := range rows {
			rows[i] = row{key: int32(rng.IntN(5)) - 2, score: int32(rng.IntN(10)), valid: rng.IntN(5) != 0, active: rng.IntN(4) != 0}
		}
	}
	leftTable, rightTable := makeTable(leftRows), makeTable(rightRows)
	schema := MakeSchema([]string{"key", "id", "score"}, []dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.Int32T()})
	left := MakeScan(leftTable, schema).As("l")
	right := MakeScan(rightTable, schema).As("r")
	for _, kind := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		physical, err := MakePhysicalPlan(left.Join(right, kind, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")}, MakeColumn("l.score").Gt(MakeColumn("r.score"))))
		if err != nil {
			t.Fatal(err)
		}
		got := make(map[string]int)
		result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20}, func(b *store.Batch) bool {
			for row := range b.Len() {
				if b.Selection() != nil {
					mask := b.Selection().MakeBitMapTemp(a)
					selected := mask.IsSet(row)
					mask.Release()
					if !selected {
						continue
					}
				}
				leftID := "NULL"
				if b.VectorAt(1).Validity() == nil || b.VectorAt(1).Validity().IsSet(row) {
					leftID = fmt.Sprint(b.VectorAt(1).I32s()[row])
				}
				rightID := "NULL"
				if kind != JoinSemi && kind != JoinAnti && (b.VectorAt(4).Validity() == nil || b.VectorAt(4).Validity().IsSet(row)) {
					rightID = fmt.Sprint(b.VectorAt(4).I32s()[row])
				}
				got[leftID+"/"+rightID]++
			}
			return true
		})
		physical.Release()
		if result.Code() != ExecutionCompleted {
			t.Fatalf("join %d: result %v", kind, result.Code())
		}
		want := make(map[string]int)
		matchedRight := make([]bool, len(rightRows))
		for i, l := range leftRows {
			if !l.active {
				continue
			}
			matched := false
			for j, r := range rightRows {
				if !r.active || !l.valid || !r.valid || l.key != r.key || l.score <= r.score {
					continue
				}
				matched, matchedRight[j] = true, true
				if kind == JoinInner || kind == JoinLeft || kind == JoinFull {
					want[fmt.Sprintf("%d/%d", i, j)]++
				}
			}
			if kind == JoinSemi && matched || kind == JoinAnti && !matched {
				want[fmt.Sprintf("%d/NULL", i)]++
			}
			if !matched && (kind == JoinLeft || kind == JoinFull) {
				want[fmt.Sprintf("%d/NULL", i)]++
			}
		}
		if kind == JoinFull {
			for j, r := range rightRows {
				if r.active && !matchedRight[j] {
					want[fmt.Sprintf("NULL/%d", j)]++
				}
			}
		}
		if len(got) != len(want) {
			t.Fatalf("join %d: got %v, want %v", kind, got, want)
		}
		for pair, n := range want {
			if got[pair] != n {
				t.Fatalf("join %d pair %s: got %d, want %d", kind, pair, got[pair], n)
			}
		}
	}
	leftTable.Release()
	rightTable.Release()
}
