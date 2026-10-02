package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCompactMultiSortDifferentialAndOwnership(t *testing.T) {
	for _, length := range []int{0, 1, 9, 129, 1025} {
		for mode := range 8 {
			t.Run(fmt.Sprintf("n=%d/mode=%d", length, mode), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
				schema := MakeSchema([]string{"n", "s", "id", "payload"}, []dtype.Type{dtype.Float64T(), dtype.StringT(), dtype.Int64T(), dtype.StringT()})
				rng := rand.New(rand.NewPCG(uint64(length+1), uint64(mode+17)))
				texts := []string{"", "a", "a\x00", "a\x00\x00", "abcdefghijklm shared long prefix a", "abcdefghijklm shared long prefix b", "ø雪\x00tail", "z"}
				type record struct {
					id           int64
					n            float64
					s            string
					nullN, nullS bool
				}
				var expected []record
				var batches []*store.Batch
				for start := 0; start < length || start == 0; start += 17 {
					n := min(17, length-start)
					num := store.MakeVector(a, n, dtype.Float64T(), true)
					ids := store.MakeVector(a, n, dtype.Int64T(), false)
					text, valid, payload := make([][]byte, n), make([]bool, n), make([][]byte, n)
					dictIDs := a.AllocSeg(n * 4)
					dictValid := store.MakeBitMap(a, n)
					selection := store.MakeBitMap(a, n)
					var offsets []uint32
					for row := range n {
						id := start + row
						v := float64(rng.IntN(7) - 3)
						switch id % 19 {
						case 0:
							v = math.NaN()
						case 1:
							v = math.Inf(-1)
						case 2:
							v = math.Inf(1)
						case 3:
							v = math.Copysign(0, -1)
						}
						num.F64s()[row] = v
						ids.I64s()[row] = int64(id)
						nullN := id%11 == 0 || mode == 7
						nullS := id%13 == 0 || mode == 7
						if nullN {
							num.Validity().Clear(row)
						}
						entry := rng.IntN(len(texts))
						text[row] = []byte(texts[entry])
						valid[row] = !nullS
						dictIDs.AsU32T()[row] = uint32(entry)
						if !nullS {
							dictValid.Set(row)
						}
						payload[row] = []byte(fmt.Sprintf("retained owned payload row %d", id))
						if id%5 != 2 {
							selection.Set(row)
							offsets = append(offsets, uint32(row))
							expected = append(expected, record{int64(id), v, texts[entry], nullN, nullS})
						}
					}
					str := store.MakeStringVector(a, text, valid)
					if start%2 != 0 {
						str.Release()
						dictionary := make([][]byte, len(texts))
						for i := range texts {
							dictionary[i] = []byte(texts[i])
						}
						str = store.MakeDictionaryStringVectorFromOwnedSegments(store.MakeStringVector(a, dictionary, nil), n, dictIDs, dictValid)
					} else {
						dictIDs.Dec()
						dictValid.Release()
					}
					batch := store.MakeBatch([]store.Vector{num, str, ids, store.MakeStringVector(a, payload, nil)})
					if start%2 == 0 {
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
					} else {
						selection.Release()
						batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
					}
					batches = append(batches, batch)
				}
				table := store.MakeTable(batches)
				nk, sk := MakeOrderKey(MakeColumn("n")), MakeOrderKey(MakeColumn("s"))
				descN, descS := mode&1 != 0, mode&2 != 0
				firstN, firstS := mode&2 != 0, mode&4 != 0
				if descN {
					nk = nk.Desc()
				}
				if descS {
					sk = sk.Desc()
				}
				if firstN {
					nk = nk.NullsFirst()
				}
				if firstS {
					sk = sk.NullsFirst()
				}
				stringFirst := mode&4 != 0
				orders := []OrderKey{nk, sk}
				if stringFirst {
					orders = []OrderKey{sk, nk}
				}
				// A computed tertiary key also exercises expression lifetimes.
				if mode == 7 {
					orders = append(orders, MakeOrderKey(MakeLiteral(int64(0))))
				} else {
					orders = append(orders, MakeOrderKey(MakeColumn("id").Neg()).Desc())
				}
				p, err := MakePhysicalPlan(MakeScan(table, schema).OrderBy(orders...))
				if err != nil {
					t.Fatal(err)
				}
				step := p.steps[0]
				state := makeCompactSortState(step)
				if state == nil {
					t.Fatal("multi-key sort not compact")
				}
				for _, batch := range batches {
					if !state.add(a, batch, step, 1<<24) {
						t.Fatal("add failed")
					}
					batch.Release()
				}
				p.Release()
				table.Release()
				compareN := func(l, r record) int {
					if l.nullN != r.nullN {
						if l.nullN == firstN {
							return -1
						}
						return 1
					}
					if l.nullN {
						return 0
					}
					cmp := 0
					if math.IsNaN(l.n) {
						if !math.IsNaN(r.n) {
							cmp = 1
						}
					} else if math.IsNaN(r.n) {
						cmp = -1
					} else if l.n < r.n {
						cmp = -1
					} else if l.n > r.n {
						cmp = 1
					}
					if descN {
						cmp = -cmp
					}
					return cmp
				}
				compareS := func(l, r record) int {
					if l.nullS != r.nullS {
						if l.nullS == firstS {
							return -1
						}
						return 1
					}
					if l.nullS {
						return 0
					}
					cmp := 0
					if l.s < r.s {
						cmp = -1
					} else if l.s > r.s {
						cmp = 1
					}
					if descS {
						cmp = -cmp
					}
					return cmp
				}
				sort.SliceStable(expected, func(i, j int) bool {
					l, r := expected[i], expected[j]
					n, s := compareN(l, r), compareS(l, r)
					if stringFirst {
						n, s = s, n
					}
					if n != 0 {
						return n < 0
					}
					if s != 0 {
						return s < 0
					}
					return l.id < r.id
				})
				out := state.finish(a, step)
				state.release()
				if len(expected) == 0 {
					if out != nil {
						t.Fatal("empty output")
					}
				} else {
					if out == nil || out.Len() != len(expected) {
						t.Fatal("wrong output domain")
					}
					for row, want := range expected {
						if out.VectorAt(2).I64s()[row] != want.id || out.VectorAt(0).Validity().IsSet(row) == want.nullN || out.VectorAt(1).Validity().IsSet(row) == want.nullS {
							t.Fatalf("row %d expected %+v, got id %d", row, want, out.VectorAt(2).I64s()[row])
						}
						if !want.nullN && math.Float64bits(out.VectorAt(0).F64s()[row]) != math.Float64bits(want.n) {
							t.Fatal("numeric payload changed")
						}
						if !want.nullS && out.VectorAt(1).StringAt(row).View() != want.s {
							t.Fatal("string payload changed")
						}
						if out.VectorAt(3).StringAt(row).View() != fmt.Sprintf("retained owned payload row %d", want.id) {
							t.Fatal("output backing not retained")
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

func TestCompactSortComputedStringsAndScopedFailure(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	str := store.MakeStringVector(a, [][]byte{[]byte("UPPER long string"), []byte("z"), []byte("a")}, nil)
	batch := store.MakeBatch([]store.Vector{str})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s"}, []dtype.Type{dtype.StringT()})).OrderBy(MakeOrderKey(MakeColumn("s").Lower())))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[0]
	for _, limit := range []int64{64, 192, 384, 8192} {
		scope := mem.MakeAllocationScope(a, limit)
		scoped := mem.MakeAllocatorWithScope(a, scope)
		state := makeCompactSortState(step)
		state.scope = scope
		ok := state.add(scoped, batch, step, limit)
		if limit < 8192 {
			if ok {
				t.Fatal("tiny budget accepted")
			}
		} else {
			if !ok {
				t.Fatal("adequate budget rejected")
			}
			out := state.finish(scoped, step)
			if out == nil {
				t.Fatal("computed string sort failed")
			}
			for i, want := range []string{"a", "UPPER long string", "z"} {
				if out.VectorAt(0).StringAt(i).View() != want {
					t.Fatal("computed sort lost backing")
				}
			}
			out.Release()
		}
		state.release()
		if scope.Live() != 0 {
			t.Fatalf("scope retained %d bytes", scope.Live())
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	state := makeCompactSortState(step)
	state.ctx = ctx
	if !state.add(a, batch, step, 8192) {
		t.Fatal("add")
	}
	cancel()
	if state.finish(a, step) != nil {
		t.Fatal("cancelled sort emitted output")
	}
	state.release()
	called := 0
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { called++; return false })
	if result.Code() != ExecutionStopped || called != 1 {
		t.Fatalf("sink termination %v/%d", result.Code(), called)
	}
	p.Release()
	table.Release()
	batch.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("sort scope leaked: %+v", usage)
	}
}

type sortCancelContext struct {
	context.Context
	cancel context.CancelFunc
	calls  int
}

func (ctx *sortCancelContext) Err() error {
	ctx.calls++
	if ctx.calls == 25 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestCompactSortCancellationDuringMerge(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	texts := make([][]byte, 257)
	for i := range texts {
		texts[i] = []byte(fmt.Sprintf("descending long string %05d", 257-i))
	}
	batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil)})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s"}, []dtype.Type{dtype.StringT()})).OrderBy(MakeOrderKey(MakeColumn("s"))))
	if err != nil {
		t.Fatal(err)
	}
	scope := mem.MakeAllocationScope(a, 1<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	state := makeCompactSortState(p.steps[0])
	state.scope = scope
	if !state.add(scoped, batch, p.steps[0], 1<<20) {
		t.Fatal("add")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probe := &sortCancelContext{Context: ctx, cancel: cancel}
	state.ctx = probe
	if out := state.finish(scoped, p.steps[0]); out != nil {
		out.Release()
		t.Fatal("cancelled merge emitted output")
	}
	if probe.calls != 25 {
		t.Fatalf("merge did not stop promptly: %d polls", probe.calls)
	}
	state.release()
	if scope.Live() != 0 {
		t.Fatalf("cancelled merge leaked %d bytes", scope.Live())
	}
	p.Release()
	table.Release()
	batch.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("cancelled merge leaked %+v", usage)
	}
}
