package plan

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCompactStringRadixDifferentialAndOwnership(t *testing.T) {
	rng := rand.New(rand.NewPCG(8172, 513))
	texts := []string{"", "\x00", "\x00\x00", "a", "a\x00", "a\x00\x00", "ø雪", "\xff", "\xff\x00"}
	for _, length := range []int{7, 8, 12, 14, 15, 16, 17, 31, 32} {
		prefix := strings.Repeat("a", length)
		texts = append(texts, prefix, prefix+"\x00", prefix+"\x00b", prefix+"b", prefix+"\xff")
	}
	for range 128 {
		text := make([]byte, rng.IntN(48))
		for i := range text {
			text[i] = byte(rng.IntN(256))
		}
		texts = append(texts, string(text))
	}
	for _, length := range []int{0, 1, 255, 256, 257, 1025} {
		for mode := range 8 {
			t.Run(fmt.Sprintf("n=%d/mode=%d", length, mode), func(t *testing.T) {
				rng := rand.New(rand.NewPCG(uint64(length)+8172, uint64(mode)+513))
				a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
				type record struct {
					text string
					id   int32
					null bool
				}
				var want []record
				var batches []*store.Batch
				for start := 0; start < length || start == 0; start += 67 {
					n := min(67, length-start)
					ids := store.MakeVector(a, n, dtype.Int32T(), false)
					input, valid := make([][]byte, n), make([]bool, n)
					indices := a.AllocSeg(n * 4)
					validity := store.MakeBitMap(a, n)
					mask := store.MakeBitMap(a, n)
					var offsets []uint32
					for row := range n {
						id, at := start+row, rng.IntN(len(texts))
						ids.I32s()[row] = int32(id)
						input[row], valid[row] = []byte(texts[at]), id%11 != 0 && mode != 7
						indices.AsU32T()[row] = uint32(at)
						if valid[row] {
							validity.Set(row)
						}
						if mode&2 == 0 || id%5 != 2 {
							mask.Set(row)
							offsets = append(offsets, uint32(row))
							want = append(want, record{texts[at], int32(id), !valid[row]})
						}
					}
					var text store.Vector
					if start%2 == 0 {
						text = store.MakeStringVector(a, input, valid)
						indices.Dec()
						validity.Release()
					} else {
						dictionary := make([][]byte, len(texts))
						for i := range texts {
							dictionary[i] = []byte(texts[i])
						}
						text = store.MakeDictionaryStringVectorFromOwnedSegments(store.MakeStringVector(a, dictionary, nil), n, indices, validity)
					}
					batch := store.MakeBatch([]store.Vector{text, ids})
					if start%2 == 0 {
						batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
					} else {
						mask.Release()
						batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
					}
					batches = append(batches, batch)
				}
				descending, nullsFirst := mode&1 != 0, mode&2 != 0
				sort.SliceStable(want, func(i, j int) bool {
					if want[i].null != want[j].null {
						return want[i].null == nullsFirst
					}
					if want[i].null {
						return false
					}
					if descending {
						return want[i].text > want[j].text
					}
					return want[i].text < want[j].text
				})
				table := store.MakeTable(batches)
				key := MakeColumn("s")
				if mode&4 != 0 {
					key = key.Replace("a", "a")
				}
				order := MakeOrderKey(key)
				if descending {
					order = order.Desc()
				}
				if nullsFirst {
					order = order.NullsFirst()
				}
				p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s", "id"}, []dtype.Type{dtype.StringT(), dtype.Int32T()})).OrderBy(order))
				if err != nil {
					t.Fatal(err)
				}
				var out *store.Batch
				result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 24}, func(batch *store.Batch) bool { out = batch.Retain(); return true })
				p.Release()
				table.Release()
				for _, batch := range batches {
					batch.Release()
				}
				if result.Code() != ExecutionCompleted || (out == nil) != (len(want) == 0) {
					t.Fatalf("result=%v output=%v want=%d rows", result.Code(), out, len(want))
				}
				if out != nil {
					if out.Len() != len(want) {
						t.Fatalf("got %d rows, want %d", out.Len(), len(want))
					}
					for i, row := range want {
						v := out.VectorAt(0)
						null := v.Validity() != nil && !v.Validity().IsSet(i)
						if out.VectorAt(1).I32s()[i] != row.id || null != row.null || !null && v.StringAt(i).View() != row.text {
							t.Fatalf("row %d: got id=%d/null=%t/text=%q, want %+v", i, out.VectorAt(1).I32s()[i], null, v.StringAt(i).View(), row)
						}
					}
					out.Release()
				}
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("sort leaked: %+v", usage)
				}
			})
		}
	}
}

func TestCompactStringRadixCancellationAndRelease(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 22}, []int{1 << 22})
	texts := make([][]byte, 131073)
	for i := range texts {
		texts[i] = []byte(fmt.Sprintf("%013d", len(texts)-i))
	}
	batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil)})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s"}, []dtype.Type{dtype.StringT()})).OrderBy(MakeOrderKey(MakeColumn("s"))))
	if err != nil {
		t.Fatal(err)
	}
	scope := mem.MakeAllocationScope(a, 1<<26)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	state := makeCompactSortState(p.steps[0])
	state.scope = scope
	if !state.add(scoped, batch, p.steps[0], 1<<26) {
		t.Fatal("add failed")
	}
	prior := scope.Live()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probe := &sortCancelContext{Context: ctx, cancel: cancel}
	state.ctx = probe
	if out := state.finish(scoped, p.steps[0]); out != nil {
		out.Release()
		t.Fatal("cancelled radix sort emitted output")
	}
	if probe.calls != 25 || scope.Peak() <= prior+int64(len(texts)*8) || scope.Live() != prior {
		t.Fatalf("polls=%d peak=%d live=%d prior=%d", probe.calls, scope.Peak(), scope.Live(), prior)
	}
	state.release()
	p.Release()
	table.Release()
	batch.Release()
	if scope.Live() != 0 {
		t.Fatalf("scope leaked %d bytes", scope.Live())
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("cancelled sort leaked: %+v", usage)
	}
}

func TestCompactStringRadixOptionalCacheBudget(t *testing.T) {
	const length = 512
	for mode := range 4 {
		for _, cache := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/cache=%t", mode, cache), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
				texts, valid := make([][]byte, length), make([]bool, length)
				want := make([]int, length)
				for i := range texts {
					texts[i], valid[i], want[i] = []byte(fmt.Sprintf("%013d\x00suffix", length-i)), i%7 != 0, i
				}
				batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, valid)})
				table := store.MakeTable([]*store.Batch{batch})
				descending, nullsFirst := mode&1 != 0, mode&2 != 0
				order := MakeOrderKey(MakeColumn("s"))
				if descending {
					order = order.Desc()
				}
				if nullsFirst {
					order = order.NullsFirst()
				}
				p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s"}, []dtype.Type{dtype.StringT()})).OrderBy(order))
				if err != nil {
					t.Fatal(err)
				}
				limit := int64(length*24 + 1024)
				if cache {
					limit += length * 16
				}
				scope := mem.MakeAllocationScope(a, limit)
				scoped := mem.MakeAllocatorWithScope(a, scope)
				state := makeCompactSortState(p.steps[0])
				state.scope = scope
				if !state.add(scoped, batch, p.steps[0], 1<<20) {
					t.Fatal("add failed")
				}
				temp := scoped.AllocSegTemp(length * 8)
				if temp == nil {
					t.Fatal("scratch failed")
				}
				prior := scope.Live()
				source, _, ok := state.sortStrings(scoped, &state.keys[0], p.steps[0].order[0], state.items.AsU64T()[:length], temp.AsU64T())
				if !ok || scope.Exhausted() || scope.Live() != prior || (scope.Peak() > prior) != cache {
					t.Fatalf("sort=%t exhausted=%t live=%d peak=%d prior=%d", ok, scope.Exhausted(), scope.Live(), scope.Peak(), prior)
				}
				sort.SliceStable(want, func(i, j int) bool {
					l, r := want[i], want[j]
					if valid[l] != valid[r] {
						return !valid[l] == nullsFirst
					}
					if !valid[l] {
						return false
					}
					if descending {
						return string(texts[l]) > string(texts[r])
					}
					return string(texts[l]) < string(texts[r])
				})
				for i, id := range source {
					if int(id) != want[i] {
						t.Fatalf("row %d: got %d, want %d", i, id, want[i])
					}
				}
				temp.Dec()
				state.release()
				p.Release()
				table.Release()
				batch.Release()
				if scope.Live() != 0 {
					t.Fatalf("scope leaked %d bytes", scope.Live())
				}
			})
		}
	}
}
