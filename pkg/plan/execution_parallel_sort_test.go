package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelSortDifferentialAndOwnership(t *testing.T) {
	for _, test := range []struct{ rows, workers, mode, batchRows int }{
		{0, 4, 0, 1024}, {1, 4, 0, 1024}, {32767, 4, 0, 1024},
		{32768, 2, 1, 32768}, {32769, 3, 2, 4093}, {65537, 5, 3, 2047},
		{65537, 7, 4, 65537}, {131073, 7, 5, 4093},
	} {
		t.Run(fmt.Sprintf("rows=%d/workers=%d/mode=%d", test.rows, test.workers, test.mode), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			schema := MakeSchema([]string{"n", "s", "id", "payload"}, []dtype.Type{dtype.Float64T(), dtype.StringT(), dtype.Int64T(), dtype.StringT()})
			type record struct {
				id           int64
				n            float64
				s            string
				nullN, nullS bool
			}
			rng := rand.New(rand.NewPCG(uint64(test.rows+1), uint64(test.mode+1)))
			texts := []string{"", "\x00", "a", "a\x00", "a\x00\x00", "0123456789abcde shared suffix a", "0123456789abcde shared suffix b", "ø雪\x00tail", "\xff"}
			var expected []record
			var batches []*store.Batch
			for start := 0; start < test.rows || start == 0; start += test.batchRows {
				length := min(test.batchRows, test.rows-start)
				numbers := store.MakeVector(a, length, dtype.Float64T(), true)
				ids := store.MakeVector(a, length, dtype.Int64T(), false)
				stringsIn, payloads, valid := make([][]byte, length), make([][]byte, length), make([]bool, length)
				dictionaryIDs := make([]uint32, length)
				selection := store.MakeBitMap(a, length)
				var offsets []uint32
				for row := range length {
					id := start + row
					n := float64(rng.IntN(19) - 9)
					switch id % 23 {
					case 0:
						n = math.Float64frombits(0x7ff8000000000000 | uint64(id%128))
					case 1:
						n = math.Inf(-1)
					case 2:
						n = math.Inf(1)
					case 3:
						n = math.Copysign(0, -1)
					}
					entry := rng.IntN(len(texts))
					text := texts[entry]
					dictionaryIDs[row] = uint32(entry)
					nullN, nullS := id%17 == 0, id%19 == 0
					if test.mode == 4 {
						n, text, nullN, nullS = 0, "all equal long retained text", false, false
					} else if test.mode == 5 {
						nullN, nullS = true, true
					}
					numbers.F64s()[row], ids.I64s()[row] = n, int64(id)
					if nullN {
						numbers.Validity().Clear(row)
					}
					stringsIn[row], valid[row] = []byte(text), !nullS
					payloads[row] = []byte(fmt.Sprintf("retained payload for original row %d", id))
					if test.mode%2 == 0 || id%7 != 0 {
						selection.Set(row)
						offsets = append(offsets, uint32(row))
						expected = append(expected, record{int64(id), n, text, nullN, nullS})
					}
				}
				textVector := store.MakeStringVector(a, stringsIn, valid)
				if start%2 != 0 && test.mode < 4 {
					textVector.Release()
					values := make([][]byte, len(texts))
					for i := range texts {
						values[i] = []byte(texts[i])
					}
					indices := a.AllocSeg(length * 4)
					copy(indices.AsU32T(), dictionaryIDs)
					validity := store.MakeBitMap(a, length)
					for row := range valid {
						if valid[row] {
							validity.Set(row)
						}
					}
					textVector = store.MakeDictionaryStringVectorFromOwnedSegments(store.MakeStringVector(a, values, nil), length, indices, validity)
				}
				batch := store.MakeBatch([]store.Vector{numbers, textVector, ids, store.MakeStringVector(a, payloads, nil)})
				if start%2 == 0 {
					batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
				} else {
					selection.Release()
					batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, length, offsets)))
				}
				batches = append(batches, batch)
			}
			descN, descS, firstN, firstS := test.mode&1 != 0, test.mode&2 != 0, test.mode&2 != 0, test.mode&1 != 0
			nk, sk := MakeOrderKey(MakeColumn("n")), MakeOrderKey(MakeColumn("s").Replace("a", "a"))
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
			orders := []OrderKey{nk, sk}
			if test.mode&1 != 0 {
				orders = []OrderKey{sk, nk}
			}
			table := store.MakeTable(batches)
			p, err := MakePhysicalPlan(MakeScan(table, schema).OrderBy(orders...))
			if err != nil {
				t.Fatal(err)
			}
			step := p.steps[0]
			scope := mem.MakeAllocationScope(a, 256<<20)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			state := makeCompactSortState(step)
			state.scope, state.ctx, state.workers = scope, context.Background(), test.workers
			for _, batch := range batches {
				if !state.add(scoped, batch, step, scope.Limit()) {
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
				n, str := compareN(expected[i], expected[j]), compareS(expected[i], expected[j])
				if test.mode&1 != 0 {
					n, str = str, n
				}
				if n != 0 {
					return n < 0
				}
				return str < 0
			})
			out := state.finish(scoped, step)
			state.release()
			if len(expected) == 0 {
				if out != nil {
					out.Release()
					t.Fatal("empty output")
				}
			} else {
				if out == nil || out.Len() != len(expected) {
					t.Fatal("wrong output domain")
				}
				for row, want := range expected {
					if out.VectorAt(2).I64s()[row] != want.id || out.VectorAt(0).Validity().IsSet(row) == want.nullN || out.VectorAt(1).Validity().IsSet(row) == want.nullS {
						t.Fatalf("row %d expected %+v", row, want)
					}
					if !want.nullN && math.Float64bits(out.VectorAt(0).F64s()[row]) != math.Float64bits(want.n) {
						t.Fatal("numeric bits changed")
					}
					if !want.nullS && out.VectorAt(1).StringAt(row).View() != want.s {
						t.Fatal("string changed")
					}
					if out.VectorAt(3).StringAt(row).View() != fmt.Sprintf("retained payload for original row %d", want.id) {
						t.Fatal("output ownership")
					}
				}
				out.Release()
			}
			if scope.Live() != 0 {
				t.Fatalf("scope leak %d", scope.Live())
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("allocator leak %+v", usage)
			}
		})
	}
}

func TestParallelSortMergePath(t *testing.T) {
	rng := rand.New(rand.NewPCG(72, 38))
	for round := range 200 {
		left, right := make([]uint64, rng.IntN(100)), make([]uint64, rng.IntN(100))
		for i := range left {
			left[i] = uint64(rng.IntN(10))
		}
		for i := range right {
			right[i] = uint64(rng.IntN(10))
		}
		sort.Slice(left, func(i, j int) bool { return left[i] < left[j] })
		sort.Slice(right, func(i, j int) bool { return right[i] < right[j] })
		less := func(a, b uint64) bool { return a < b }
		l, r := 0, 0
		for diagonal := 0; diagonal <= len(left)+len(right); diagonal++ {
			pl, pr := parallelSortMergePath(left, right, diagonal, less)
			if pl != l || pr != r {
				t.Fatalf("round %d diagonal %d got %d,%d want %d,%d", round, diagonal, pl, pr, l, r)
			}
			if l < len(left) && (r == len(right) || !less(right[r], left[l])) {
				l++
			} else {
				r++
			}
		}
	}
}

type parallelSortCancelContext struct {
	context.Context
	cancel context.CancelFunc
	calls  atomic.Int64
}

func (ctx *parallelSortCancelContext) Err() error {
	if ctx.calls.Add(1) == 20 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestParallelSortCancellationAndBudget(t *testing.T) {
	for _, stringKey := range []bool{false, true} {
		t.Run(fmt.Sprintf("string=%t", stringKey), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			const rows = 65537
			keyType := dtype.Int64T()
			key := store.MakeVector(a, rows, keyType, false)
			if stringKey {
				key.Release()
				keyType = dtype.StringT()
				texts := make([][]byte, rows)
				for i := range texts {
					texts[i] = []byte(fmt.Sprintf("key %08d", rows-i))
				}
				key = store.MakeStringVector(a, texts, nil)
			} else {
				for i := range rows {
					key.I64s()[i] = int64(rows - i)
				}
			}
			batch := store.MakeBatch([]store.Vector{key})
			table := store.MakeTable([]*store.Batch{batch})
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{keyType})).OrderBy(MakeOrderKey(MakeColumn("key"))))
			if err != nil {
				t.Fatal(err)
			}
			step := p.steps[0]
			scope := mem.MakeAllocationScope(a, 16<<20)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			state := makeCompactSortState(step)
			state.scope, state.workers = scope, 4
			if !state.add(scoped, batch, step, scope.Limit()) {
				t.Fatal("add")
			}
			before := scope.Live()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			probe := &parallelSortCancelContext{Context: ctx, cancel: cancel}
			state.ctx = probe
			if out := state.finish(scoped, step); out != nil {
				out.Release()
				t.Fatal("cancelled sort emitted output")
			}
			if probe.calls.Load() < 20 || scope.Live() != before {
				t.Fatalf("cancellation leaked or did not poll: %d/%d", probe.calls.Load(), scope.Live())
			}
			state.release()
			if scope.Live() != 0 {
				t.Fatal("scope retained state")
			}
			for _, budget := range []int64{64, 1 << 20} {
				calls := 0
				result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: budget, Workers: 4}, func(*store.Batch) bool { calls++; return true })
				if result.Code() != ExecutionResourceExhausted || calls != 0 {
					t.Fatalf("budget=%d result=%v calls=%d", budget, result.Code(), calls)
				}
			}
			calls := 0
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16 << 20, Workers: 4}, func(*store.Batch) bool { calls++; return false })
			if result.Code() != ExecutionStopped || calls != 1 {
				t.Fatalf("sink stop %v/%d", result.Code(), calls)
			}
			p.Release()
			table.Release()
			batch.Release()
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("allocator leak %+v", usage)
			}
		})
	}
}

func TestParallelSortOptionalPrefixBudget(t *testing.T) {
	for _, cacheRows := range []int{0, 16384, 65536} {
		t.Run(fmt.Sprintf("cacheRows=%d", cacheRows), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			const rows = 65536
			texts := make([][]byte, rows)
			for i := range texts {
				texts[i] = []byte(fmt.Sprintf("common shared prefix %05d", (rows-i)%257))
			}
			batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil)})
			table := store.MakeTable([]*store.Batch{batch})
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.StringT()})).OrderBy(MakeOrderKey(MakeColumn("key"))))
			if err != nil {
				t.Fatal(err)
			}
			step := p.steps[0]
			scope := mem.MakeAllocationScope(a, 8<<20)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			state := makeCompactSortState(step)
			state.scope, state.ctx, state.workers = scope, context.Background(), 4
			if !state.add(scoped, batch, step, scope.Limit()) {
				t.Fatal("add")
			}
			temp := scoped.AllocSegTemp(rows * 8)
			if temp == nil {
				t.Fatal("merge scratch")
			}
			filler, ok := scope.TryAllocSeg(int(scope.Limit()-scope.Live()) - cacheRows*16)
			if !ok {
				t.Fatal("reserve remaining budget")
			}
			live := scope.Live()
			items, _, sorted := state.sortParallel(scoped, step, state.items.AsU64T()[:rows], temp.AsU64T())
			if !sorted || scope.Exhausted() || scope.Live() != live || scope.Peak() > scope.Limit() {
				t.Fatal("optional cache budget rejected or leaked")
			}
			for i := 1; i < len(items); i++ {
				previous, current := batch.VectorAt(0).StringAt(int(items[i-1])).View(), batch.VectorAt(0).StringAt(int(items[i])).View()
				if previous > current || previous == current && items[i-1] > items[i] {
					t.Fatalf("ordering or stable ties at %d", i)
				}
			}
			filler.Dec()
			temp.Dec()
			state.release()
			p.Release()
			table.Release()
			batch.Release()
			if scope.Live() != 0 {
				t.Fatalf("scope leak %d", scope.Live())
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("allocator leak %+v", usage)
			}
		})
	}
}

func TestParallelSortPrefixMergeDifferential(t *testing.T) {
	for mode := range 8 {
		t.Run(fmt.Sprintf("mode=%d", mode), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			const rows = 512
			rng := rand.New(rand.NewPCG(771, uint64(mode+1)))
			texts := make([][]byte, rows)
			valid := make([]bool, rows)
			candidates := []string{"", "\x00", "\x00\x00", "a", "a\x00", "a\x00\x00", "abcdefghijklmno", "abcdefghijklmno\x00", "abcdefghijklmno\x00a", "abcdefghijklmno\xff", "abcdefghijklmnp", "\xff", "ø雪"}
			numbers := store.MakeVector(a, rows, dtype.Int32T(), true)
			for i := range texts {
				if i%2 == 0 {
					texts[i] = []byte(candidates[rng.IntN(len(candidates))])
				} else {
					texts[i] = make([]byte, rng.IntN(48))
					for j := range texts[i] {
						texts[i][j] = byte(rng.IntN(256))
					}
				}
				valid[i] = i%17 != 0
				numbers.I32s()[i] = int32(rng.IntN(7) - 3)
				if i%19 == 0 {
					numbers.Validity().Clear(i)
				}
			}
			batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, valid), numbers})
			table := store.MakeTable([]*store.Batch{batch})
			sk, nk := MakeOrderKey(MakeColumn("s")), MakeOrderKey(MakeColumn("n"))
			descS, firstS, descN := mode&1 != 0, mode&2 != 0, mode&4 != 0
			if descS {
				sk = sk.Desc()
			}
			if firstS {
				sk = sk.NullsFirst()
			}
			if descN {
				nk = nk.Desc()
			}
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s", "n"}, []dtype.Type{dtype.StringT(), dtype.Int32T()})).OrderBy(sk, nk))
			if err != nil {
				t.Fatal(err)
			}
			step := p.steps[0]
			scope := mem.MakeAllocationScope(a, 1<<20)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			state := makeCompactSortState(step)
			state.scope = scope
			if !state.add(scoped, batch, step, scope.Limit()) {
				t.Fatal("add")
			}
			cache := state.stringPrefixes(scoped, &state.keys[0], step.order[0])
			if cache == nil {
				t.Fatal("prefix cache")
			}
			before := scope.Live()
			for left := range rows {
				for right := range rows {
					cmp := 0
					if valid[left] != valid[right] {
						if !valid[left] == firstS {
							cmp = -1
						} else {
							cmp = 1
						}
					} else if valid[left] {
						lt, rt := string(texts[left]), string(texts[right])
						if lt < rt {
							cmp = -1
						} else if lt > rt {
							cmp = 1
						}
						if descS {
							cmp = -cmp
						}
					}
					if cmp == 0 {
						lv, rv := numbers.Validity().IsSet(left), numbers.Validity().IsSet(right)
						if lv != rv {
							if lv {
								cmp = -1
							} else {
								cmp = 1
							}
						} else if lv {
							if numbers.I32s()[left] < numbers.I32s()[right] {
								cmp = -1
							} else if numbers.I32s()[left] > numbers.I32s()[right] {
								cmp = 1
							}
							if descN {
								cmp = -cmp
							}
						}
					}
					want := cmp < 0 || cmp == 0 && left < right
					if got := state.lessItem(step, cache.AsU64T(), uint64(left), uint64(right)); got != want {
						t.Fatalf("prefix ordering mismatch left=%d right=%d", left, right)
					}
				}
			}
			if scope.Live() != before {
				t.Fatal("comparison changed reservations")
			}
			cache.Dec()
			state.release()
			p.Release()
			table.Release()
			batch.Release()
			if scope.Live() != 0 {
				t.Fatal("scope leak")
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("allocator leak %+v", usage)
			}
		})
	}
}
