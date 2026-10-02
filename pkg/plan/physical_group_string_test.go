package plan

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestStringGroupDifferentialAndOwnership(t *testing.T) {
	for _, length := range []int{0, 1, 65, 16384} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
			type identity struct {
				text string
				null bool
			}
			type totals struct{ count, nonNull, sum int64 }
			want := make(map[identity]totals)
			rng := rand.New(rand.NewPCG(87, uint64(length)))
			var batches []*store.Batch
			for start := 0; start < length || start == 0; start += 257 {
				n := min(257, length-start)
				texts, valid := make([][]byte, n), make([]bool, n)
				values := store.MakeVector(a, n, dtype.Int64T(), true)
				selection := store.MakeBitMap(a, n)
				var offsets []uint32
				for row := range n {
					at, id := start+row, rng.IntN(4096)
					text := fmt.Sprintf("shared prefix for long key ø %04d", id)
					if id%11 == 0 {
						text = fmt.Sprintf("short%04d", id)
					}
					if at%19 == 0 {
						text = ""
					} else if at%23 == 0 {
						text = "\x00ø\x00"
					}
					texts[row], valid[row] = []byte(text), at%113 != 0
					values.I64s()[row] = []int64{math.MaxInt64, math.MinInt64, -1, 0, 1, 37}[at%6]
					if id%13 == 0 {
						values.Validity().Clear(row)
					}
					if at%7 == 3 {
						continue
					}
					selection.Set(row)
					offsets = append(offsets, uint32(row))
					key := identity{text: text}
					if !valid[row] {
						key = identity{null: true}
					}
					total := want[key]
					total.count++
					if values.Validity().IsSet(row) {
						total.nonNull++
						total.sum += values.I64s()[row]
					}
					want[key] = total
				}
				batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, valid), values})
				if start%2 == 0 {
					batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
				} else {
					selection.Release()
					batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
				}
				batches = append(batches, batch)
			}
			table := store.MakeTable(batches)
			for _, batch := range batches {
				batch.Release()
			}
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{dtype.StringT(), dtype.Int64T()}, []bool{true, true})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("value")), MakeSum(MakeColumn("value"))))
			if err != nil {
				t.Fatal(err)
			}
			var retained []*store.Batch
			for _, workers := range []int{1, 4, 8} {
				result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16 << 20, Workers: workers}, func(batch *store.Batch) bool {
					retained = append(retained, batch.Retain())
					return true
				})
				if result.Code() != ExecutionCompleted {
					t.Fatalf("workers %d: %v/%v", workers, result.Code(), result.Err())
				}
			}
			p.Release()
			table.Release()
			for _, output := range retained {
				seen := make(map[identity]bool)
				for row := range output.Len() {
					key := identity{}
					if !output.VectorAt(0).Validity().IsSet(row) {
						key.null = true
					} else {
						key.text = output.VectorAt(0).Strings()[row].View()
					}
					total, exists := want[key]
					if !exists || seen[key] || output.VectorAt(1).I64s()[row] != total.count || output.VectorAt(2).I64s()[row] != total.nonNull || output.VectorAt(3).Validity().IsSet(row) != (total.nonNull != 0) || total.nonNull != 0 && output.VectorAt(3).I64s()[row] != total.sum {
						t.Fatalf("group %+v differs from reference %+v", key, total)
					}
					seen[key] = true
				}
				if len(seen) != len(want) {
					t.Fatalf("got %d groups, want %d", len(seen), len(want))
				}
				output.Release()
			}
			if used := a.Usage(); used.GeneralUsed != 0 || used.ScratchUsed != 0 {
				t.Fatalf("leaked string grouping storage: %+v", used)
			}
		})
	}
}

func TestStringGroupBudgetAndTermination(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	texts := make([][]byte, 1024)
	for row := range texts {
		texts[row] = []byte(fmt.Sprintf("owned long grouping key %04d", row))
	}
	batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil)})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.StringT()})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	for budget := int64(0); budget < 8192; budget += 64 {
		scope := mem.MakeAllocationScope(a, budget)
		state := makeGroupState(p.steps[0], scope)
		if state.add(a, batch, p.steps[0], budget) {
			t.Fatalf("grouping admitted budget %d", budget)
		}
		state.release()
		if scope.Live() != 0 || scope.Peak() > budget {
			t.Fatalf("scope escaped budget %d: live %d, peak %d", budget, scope.Live(), scope.Peak())
		}
	}
	baseline := a.Usage()
	for _, workers := range []int{1, 4} {
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192, Workers: workers}, func(*store.Batch) bool { t.Fatal("under-budget sink"); return true })
		if result.Code() != ExecutionResourceExhausted {
			t.Fatalf("resource result: %v/%v", result.Code(), result.Err())
		}
		result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20, Workers: workers}, func(*store.Batch) bool { return false })
		if result.Code() != ExecutionStopped {
			t.Fatalf("stop result: %v", result.Code())
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result = p.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 1 << 20, Workers: workers}, func(*store.Batch) bool { t.Fatal("cancelled sink"); return true })
		if result.Code() != ExecutionCancelled {
			t.Fatalf("cancel result: %v", result.Code())
		}
		if used := a.Usage(); used.GeneralUsed != baseline.GeneralUsed || used.ScratchUsed != baseline.ScratchUsed {
			t.Fatalf("failed execution leaked storage: %+v, baseline %+v", used, baseline)
		}
	}
	p.Release()
	table.Release()
	batch.Release()
}

func TestStringGroupMergeThenAdd(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	var batches []*store.Batch
	for part := range 2 {
		texts := make([][]byte, 4096)
		for row := range texts {
			texts[row] = []byte(fmt.Sprintf("overlapping long group key ø %04d", row+part*2048))
		}
		batches = append(batches, store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil)}))
	}
	table := store.MakeTable(batches)
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.StringT()})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[0]
	scope := mem.MakeAllocationScope(a, 8<<20)
	view := mem.MakeAllocatorWithScope(a, scope)
	first, second := makeGroupState(step, scope), makeGroupState(step, scope)
	if !first.add(view, batches[0], step, math.MaxInt64) || !second.add(view, batches[1], step, math.MaxInt64) {
		t.Fatal("string state build failed")
	}
	states := []*groupState{makeGroupState(step, scope), makeGroupState(step, scope)}
	for i := range states {
		if !states[i].add(view, batches[i], step, math.MaxInt64) {
			t.Fatal("sharded string state build failed")
		}
	}
	seen := make(map[int]bool)
	result, sharded := p.executeGroupedShards(context.Background(), view, scope, step, states, 8<<20, 8192, func(output *store.Batch) bool {
		for row := range output.Len() {
			var id int
			_, err := fmt.Sscanf(output.VectorAt(0).Strings()[row].View(), "overlapping long group key ø %04d", &id)
			want := int64(1)
			if id >= 2048 && id < 4096 {
				want = 2
			}
			if err != nil || id < 0 || id >= 6144 || seen[id] || output.VectorAt(1).I64s()[row] != want {
				t.Fatalf("incorrect sharded string group %d/%v", id, err)
			}
			seen[id] = true
		}
		return true
	})
	for _, state := range states {
		if state != nil {
			state.release()
		}
	}
	if !sharded || result.Code() != ExecutionCompleted || len(seen) != 6144 {
		t.Fatalf("sharded merge: %v/%v, %d groups", sharded, result.Code(), len(seen))
	}
	if !first.merge(view, second, step, math.MaxInt64) {
		t.Fatal("string state build/merge failed")
	}
	second.release()
	if !first.add(view, batches[0], step, math.MaxInt64) {
		t.Fatal("direct lookup failed after encoded merge")
	}
	for _, batch := range batches {
		batch.Release()
	}
	table.Release()
	p.Release()
	output := first.finish(view, step)
	first.release()
	if output == nil || output.Len() != 6144 {
		t.Fatal("incorrect merged group count")
	}
	for row := range output.Len() {
		id := row
		want := int64(2)
		if id >= 2048 {
			want = 3
		}
		if id >= 4096 {
			want = 1
		}
		if output.VectorAt(0).Strings()[row].View() != fmt.Sprintf("overlapping long group key ø %04d", id) || output.VectorAt(1).I64s()[row] != want {
			t.Fatalf("merged/appended group %d changed", id)
		}
	}
	output.Release()
	if scope.Live() != 0 {
		t.Fatalf("merge leaked %d scoped bytes", scope.Live())
	}
	if used := a.Usage(); used.GeneralUsed != 0 || used.ScratchUsed != 0 {
		t.Fatalf("merge leaked storage: %+v", used)
	}
}

func TestStringKeyIndexCollisionResolution(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	h := keyIndex{stringKeys: true}
	keys := keyArena{}
	defer h.release()
	defer keys.release()
	for id, text := range []string{"wrong string", "", "wanted string"} {
		key := make([]byte, 9+len(text))
		key[0] = 1
		binary.LittleEndian.PutUint64(key[1:], uint64(len(text)))
		copy(key[9:], text)
		if !keys.append(a, key, math.MaxInt64) || !h.put(a, key, id, math.MaxInt64) {
			t.Fatal("index setup failed")
		}
	}
	if !keys.append(a, []byte{0}, math.MaxInt64) || !h.put(a, []byte{0}, 3, math.MaxInt64) {
		t.Fatal("null key setup failed")
	}
	// Force equal hashes so lookup must compare the owned key bytes.
	slots := h.slots.AsU64T()
	clear(slots)
	hash := h.hash(keys.key(2))
	at := hash & uint64(len(slots)/2-1)
	for _, id := range []int{0, 1, 3, 2} {
		slots[2*at], slots[2*at+1] = hash, uint64(id+1)
		at = (at + 1) & uint64(len(slots)/2-1)
	}
	if id, ok := h.lookupString("wanted string", false, &keys); !ok || id != 2 {
		t.Fatalf("hash collision returned %d/%v", id, ok)
	}
	if _, ok := h.lookupString("absent", false, &keys); ok {
		t.Fatal("absent key matched collision")
	}
	for _, isNull := range []bool{false, true} {
		clear(slots)
		hash := h.hash(keys.key(1))
		if isNull {
			hash = h.hash(keys.key(3))
		}
		at := hash & uint64(len(slots)/2-1)
		for _, id := range []int{0, 1, 3} {
			slots[2*at], slots[2*at+1] = hash, uint64(id+1)
			at = (at + 1) & uint64(len(slots)/2-1)
		}
		want := 1
		if isNull {
			want = 3
		}
		if id, ok := h.lookupString("", isNull, &keys); !ok || id != want {
			t.Fatalf("null/empty collision returned %d/%v, want %d", id, ok, want)
		}
	}
}
