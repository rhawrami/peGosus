package plan

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalPackedSortDifferential(t *testing.T) {
	for _, length := range []int{0, 1, 7, 8, 9, 130} {
		for _, descending := range []bool{false, true} {
			for _, nullsFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("n=%d/desc=%t/nullsFirst=%t", length, descending, nullsFirst), func(t *testing.T) {
					a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
					rng := rand.New(rand.NewPCG(uint64(length)+71, 35))
					type record struct {
						number               int32
						text                 string
						id                   int32
						nullNumber, nullText bool
					}
					var records []record
					var batches []*store.Batch
					for start := 0; start < length || start == 0; start += 17 {
						n := min(17, length-start)
						integer := store.MakeVector(a, n, dtype.Int32T(), true)
						stringsIn := make([][]byte, n)
						valid := make([]bool, n)
						id := store.MakeVector(a, n, dtype.Int32T(), false)
						payload := store.MakeStringVector(a, func() [][]byte {
							out := make([][]byte, n)
							for row := range out {
								out[row] = []byte(fmt.Sprintf("owned payload for row %d", start+row))
							}
							return out
						}(), nil)
						mask := store.MakeBitMap(a, n)
						var offsets []uint32
						for row := range n {
							i := start + row
							integer.I32s()[row] = int32(rng.IntN(65536) - 30000)
							stringsIn[row] = make([]byte, rng.IntN(5))
							for j := range stringsIn[row] {
								stringsIn[row][j] = byte(rng.IntN(5))
							}
							valid[row] = i%11 != 0
							if i%13 == 0 {
								integer.Validity().Clear(row)
							}
							id.I32s()[row] = int32(i)
							if i%5 == 2 {
								continue
							}
							mask.Set(row)
							offsets = append(offsets, uint32(row))
							records = append(records, record{number: integer.I32s()[row], text: string(stringsIn[row]), id: int32(i), nullNumber: !integer.Validity().IsSet(row), nullText: !valid[row]})
						}
						text := store.MakeStringVector(a, stringsIn, valid)
						batch := store.MakeBatch([]store.Vector{integer, text, id, payload})
						if start%2 == 0 {
							batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
						} else {
							mask.Release()
							batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
						}
						batches = append(batches, batch)
					}
					table := store.MakeTable(batches)
					sort.SliceStable(records, func(i, j int) bool {
						l, r := records[i], records[j]
						if l.nullNumber != r.nullNumber {
							return l.nullNumber == nullsFirst
						}
						if !l.nullNumber && l.number != r.number {
							if descending {
								return l.number > r.number
							}
							return l.number < r.number
						}
						if l.nullText != r.nullText {
							return l.nullText == nullsFirst
						}
						if !l.nullText && l.text != r.text {
							if descending {
								return l.text > r.text
							}
							return l.text < r.text
						}
						return false
					})
					order1, order2 := MakeOrderKey(MakeColumn("number")), MakeOrderKey(MakeColumn("text"))
					if descending {
						order1, order2 = order1.Desc(), order2.Desc()
					}
					if nullsFirst {
						order1, order2 = order1.NullsFirst(), order2.NullsFirst()
					}
					plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"number", "text", "id", "payload"}, []dtype.Type{dtype.Int32T(), dtype.StringT(), dtype.Int32T(), dtype.StringT()})).OrderBy(order1, order2))
					if err != nil {
						t.Fatal(err)
					}
					var oldOutput, newOutput *store.Batch
					for _, packed := range []bool{false, true} {
						result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20, TryPackedSort: packed}, func(output *store.Batch) bool {
							if packed {
								newOutput = output.Retain()
							} else {
								oldOutput = output.Retain()
							}
							return true
						})
						if result.Code() != ExecutionCompleted {
							t.Fatalf("packed=%t: result %v", packed, result.Code())
						}
					}
					if length == 0 {
						if oldOutput != nil || newOutput != nil {
							t.Fatal("empty sort emitted rows")
						}
					} else {
						if oldOutput == nil || newOutput == nil || oldOutput.Len() != len(records) || newOutput.Len() != len(records) {
							t.Fatalf("sort shapes: old=%v new=%v want=%d", oldOutput, newOutput, len(records))
						}
						for i, want := range records {
							for _, output := range []*store.Batch{oldOutput, newOutput} {
								if output.VectorAt(2).I32s()[i] != want.id || output.VectorAt(3).Strings()[i].View() != fmt.Sprintf("owned payload for row %d", want.id) || output.VectorAt(0).Validity().IsSet(i) == want.nullNumber || output.VectorAt(1).Validity().IsSet(i) == want.nullText {
									t.Fatalf("row %d: got id=%d expected=%d", i, output.VectorAt(2).I32s()[i], want.id)
								}
								if !want.nullNumber && output.VectorAt(0).I32s()[i] != want.number {
									t.Fatalf("row %d: incorrect integer", i)
								}
								if !want.nullText && output.VectorAt(1).Strings()[i].View() != want.text {
									t.Fatalf("row %d: incorrect binary string", i)
								}
							}
						}
					}
					plan.Release()
					table.Release()
					for _, batch := range batches {
						batch.Release()
					}
					if newOutput != nil {
						for i, row := range records {
							if got := newOutput.VectorAt(3).Strings()[i].View(); got != fmt.Sprintf("owned payload for row %d", row.id) {
								t.Fatalf("retained sorted output row %d lost its string backing: %q", i, got)
							}
						}
					}
					if oldOutput != nil {
						oldOutput.Release()
					}
					if newOutput != nil {
						newOutput.Release()
					}
				})
			}
		}
	}
}

func TestPhysicalPackedSortFallsBack(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	key := store.MakeVector(a, 2, dtype.Int32T(), false)
	copy(key.I32s(), []int32{2, 1})
	text := store.MakeStringVector(a, [][]byte{[]byte("longer than seven bytes"), []byte("short")}, nil)
	batch := store.MakeBatch([]store.Vector{key, text})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	scan := MakeScan(table, MakeSchema([]string{"number", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}))
	for _, query := range []LogicalPlan{
		scan.OrderBy(MakeOrderKey(MakeColumn("number")), MakeOrderKey(MakeColumn("text"))),
		scan.Filter(MakeColumn("number").Gt(0)).OrderBy(MakeOrderKey(MakeColumn("number")), MakeOrderKey(MakeColumn("text"))),
		scan.OrderBy(MakeOrderKey(MakeColumn("number")), MakeOrderKey(MakeColumn("text"))).Limit(1),
	} {
		plan, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		var outputs int
		result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096, TryPackedSort: true}, func(batch *store.Batch) bool { outputs += batch.ActiveLen(); return true })
		plan.Release()
		if result.Code() != ExecutionCompleted || outputs == 0 {
			t.Fatalf("packed fallback result %v, rows %d", result.Code(), outputs)
		}
	}
	table.Release()
}

func TestPhysicalPackedSortCancellationBudgetAndSink(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	ints := store.MakeVector(a, 32, dtype.Int32T(), false)
	stringsIn := make([][]byte, 32)
	for i := range ints.I32s() {
		ints.I32s()[i] = int32(32 - i)
		stringsIn[i] = []byte{byte(i%4 + 'a')}
	}
	text := store.MakeStringVector(a, stringsIn, nil)
	batch := store.MakeBatch([]store.Vector{ints, text})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"integer", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()})).OrderBy(MakeOrderKey(MakeColumn("integer")), MakeOrderKey(MakeColumn("text"))))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	options := ExecutionOptions{MemoryBudget: 8192, TryPackedSort: true}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := plan.ExecuteWithOptions(ctx, a, options, func(*store.Batch) bool { t.Fatal("cancelled sort reached sink"); return true })
	if result.Code() != ExecutionCancelled || !errors.Is(result.Err(), context.Canceled) {
		t.Fatalf("cancelled sort: %v", result.Code())
	}
	result = plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128, TryPackedSort: true}, func(*store.Batch) bool { t.Fatal("resource failure reached sink"); return true })
	if result.Code() != ExecutionResourceExhausted {
		t.Fatalf("under-budget packed sort: %v", result.Code())
	}
	called := 0
	result = plan.ExecuteWithOptions(context.Background(), a, options, func(batch *store.Batch) bool {
		called++
		if batch.Len() != 32 {
			t.Fatal("sorted batch has wrong domain")
		}
		return false
	})
	plan.Release()
	if result.Code() != ExecutionStopped || called != 1 {
		t.Fatalf("early sink result: %v after %d calls", result.Code(), called)
	}
}
