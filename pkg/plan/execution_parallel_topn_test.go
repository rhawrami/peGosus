package plan

import (
	"context"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelTopNRandomized(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	type record struct {
		first                 float64
		second                int64
		id                    int32
		nullFirst, nullSecond bool
	}
	var records []record
	var batches []*store.Batch
	rng := rand.New(rand.NewPCG(151, 311))
	values := []float64{math.Inf(-1), math.Inf(1), math.NaN(), math.Float64frombits(0x7ff8000000000009), math.Copysign(0, -1), 0, -2, 2}
	for chunk := range 12 {
		length := rng.IntN(133)
		first := store.MakeVector(a, length, dtype.Float64T(), true)
		second := store.MakeVector(a, length, dtype.Int64T(), true)
		ids := store.MakeVector(a, length, dtype.Int32T(), false)
		flags := store.MakeVector(a, length, dtype.BoolT(), false)
		mask := store.MakeBitMap(a, length)
		var offsets []uint32
		for row := range length {
			id := int32(chunk*133 + row)
			first.F64s()[row] = values[rng.IntN(len(values))]
			second.I64s()[row] = int64(rng.IntN(11) - 5)
			ids.I32s()[row] = id
			if id%11 == 0 {
				first.Validity().Clear(row)
			}
			if id%13 == 0 {
				second.Validity().Clear(row)
			}
			if id%2 == 0 {
				flags.Bools()[row>>3] |= 1 << uint(row&7)
			}
			if id%7 == 0 {
				continue
			}
			offsets = append(offsets, uint32(row))
			mask.Set(row)
			if id < 100 {
				continue
			}
			records = append(records, record{first.F64s()[row], second.I64s()[row], id, !first.Validity().IsSet(row), !second.Validity().IsSet(row)})
		}
		batch := store.MakeBatch([]store.Vector{first, second, ids, flags})
		if chunk%2 == 0 {
			batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
		} else {
			mask.Release()
			batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, length, offsets)))
		}
		batches = append(batches, batch)
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	defer table.Release()
	schema := MakeSchemaWithNullability([]string{"first", "second", "id", "flag"}, []dtype.Type{dtype.Float64T(), dtype.Int64T(), dtype.Int32T(), dtype.BoolT()}, []bool{true, true, false, false})
	scan := MakeScan(table, schema).Filter(MakeColumn("id").Ge(100)).Project(MakeColumn("first"), MakeColumn("second"), MakeColumn("id"), MakeColumn("flag"))
	for mode := range 16 {
		desc1, desc2, nulls1, nulls2 := mode&1 != 0, mode&2 != 0, mode&4 != 0, mode&8 != 0
		for keys := 1; keys <= 2; keys++ {
			ordered := append([]record(nil), records...)
			sort.SliceStable(ordered, func(i, j int) bool {
				x, y := ordered[i], ordered[j]
				if x.nullFirst != y.nullFirst {
					return x.nullFirst == nulls1
				}
				if !x.nullFirst {
					cmp := 0
					if math.IsNaN(x.first) {
						if !math.IsNaN(y.first) {
							cmp = 1
						}
					} else if math.IsNaN(y.first) {
						cmp = -1
					} else if x.first < y.first {
						cmp = -1
					} else if x.first > y.first {
						cmp = 1
					}
					if cmp != 0 {
						if desc1 {
							return cmp > 0
						}
						return cmp < 0
					}
				}
				if keys == 2 {
					if x.nullSecond != y.nullSecond {
						return x.nullSecond == nulls2
					}
					if !x.nullSecond && x.second != y.second {
						if desc2 {
							return x.second > y.second
						}
						return x.second < y.second
					}
				}
				return x.id < y.id
			})
			key1, key2 := MakeOrderKey(MakeColumn("first")), MakeOrderKey(MakeColumn("second").Add(int64(0)))
			if desc1 {
				key1 = key1.Desc()
			}
			if desc2 {
				key2 = key2.Desc()
			}
			if nulls1 {
				key1 = key1.NullsFirst()
			}
			if nulls2 {
				key2 = key2.NullsFirst()
			}
			order := []OrderKey{key1}
			if keys == 2 {
				order = append(order, key2)
			}
			for _, window := range [][2]int64{{0, 0}, {1, 0}, {17, 9}, {45, 0}, {2000, 0}, {7, 2000}} {
				p, err := MakePhysicalPlan(scan.OrderBy(order...).Limit(window[0], window[1]))
				if err != nil {
					t.Fatal(err)
				}
				want := make([]int32, 0)
				begin, end := min(int(window[1]), len(ordered)), min(int(window[0]+window[1]), len(ordered))
				for _, r := range ordered[begin:end] {
					want = append(want, r.id)
				}
				for _, workers := range []int{1, 2, 4, 8} {
					var got []int32
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: workers}, func(batch *store.Batch) bool {
						mask := batch.Selection().MakeBitMapTemp(a)
						if mask != nil {
							defer mask.Release()
						}
						for row := range batch.Len() {
							if mask == nil || mask.IsSet(row) {
								got = append(got, batch.VectorAt(2).I32s()[row])
								expected := records[slices.IndexFunc(records, func(r record) bool { return r.id == batch.VectorAt(2).I32s()[row] })]
								if math.Float64bits(batch.VectorAt(0).F64s()[row]) != math.Float64bits(expected.first) && !expected.nullFirst {
									t.Error("floating payload bits changed")
								}
							}
						}
						return true
					})
					if result.Code() != ExecutionCompleted || !slices.Equal(got, want) {
						t.Fatalf("mode=%d keys=%d workers=%d window=%v result=%v/%v got=%v want=%v", mode, keys, workers, window, result.Code(), result.Err(), got, want)
					}
				}
				p.Release()
			}
		}
	}
}

func TestParallelTopNLifecycle(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	var batches []*store.Batch
	for chunk := range 8 {
		v := store.MakeVector(a, 8192, dtype.Int32T(), true)
		for row := range v.I32s() {
			v.I32s()[row] = int32(chunk*8192 + row)
		}
		batches = append(batches, store.MakeBatch([]store.Vector{v}))
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	scan := MakeScan(table, MakeSchemaWithNullability([]string{"id"}, []dtype.Type{dtype.Int32T()}, []bool{true}))
	p, err := MakePhysicalPlan(scan.OrderBy(MakeOrderKey(MakeColumn("id"))).Limit(13, 5))
	if err != nil {
		t.Fatal(err)
	}
	opts := ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}
	var retained *store.Batch
	result, used := p.executeParallel(context.Background(), a, opts, func(b *store.Batch) bool { retained = b.Retain(); return true })
	if !used || result.Code() != ExecutionCompleted || retained == nil {
		t.Fatalf("parallel=%t result=%v/%v", used, result.Code(), result.Err())
	}
	retained.Release()
	for _, filter := range []Expr{MakeColumn("id").Lt(0), MakeColumn("id").IsNull()} {
		empty, err := MakePhysicalPlan(scan.Filter(filter).OrderBy(MakeOrderKey(MakeColumn("id"))).Limit(13))
		if err != nil {
			t.Fatal(err)
		}
		result := empty.ExecuteWithOptions(context.Background(), a, opts, func(*store.Batch) bool { t.Error("empty input reached sink"); return true })
		empty.Release()
		if result.Code() != ExecutionCompleted {
			t.Fatal(result.Code())
		}
	}
	calls := 0
	result = p.ExecuteWithOptions(context.Background(), a, opts, func(*store.Batch) bool { calls++; return false })
	if result.Code() != ExecutionStopped || calls != 1 {
		t.Fatalf("sink stop=%v calls=%d", result.Code(), calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = p.ExecuteWithOptions(ctx, a, opts, func(*store.Batch) bool { t.Error("cancelled sink"); return true })
	if result.Code() != ExecutionCancelled {
		t.Fatal(result.Code())
	}
	ctx, cancel = context.WithCancel(context.Background())
	result = p.ExecuteWithOptions(ctx, a, opts, func(*store.Batch) bool { cancel(); return true })
	cancel()
	if result.Code() != ExecutionCancelled {
		t.Fatal(result.Code())
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := p.ExecuteWithOptions(context.Background(), a, opts, func(b *store.Batch) bool {
				if b.ActiveLen() != 13 {
					t.Error("concurrent result count")
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				t.Error(result.Code(), result.Err())
			}
		}()
	}
	wg.Wait()
	large, err := MakePhysicalPlan(scan.OrderBy(MakeOrderKey(MakeColumn("id"))).Limit(65536))
	if err != nil {
		t.Fatal(err)
	}
	result, used = large.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 2 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("over-budget reached sink"); return true })
	large.Release()
	if !used || result.Code() != ExecutionResourceExhausted {
		t.Fatalf("shared heap budget %t/%v", used, result.Code())
	}
	result = p.ExecuteWithOptions(context.Background(), a, opts, func(b *store.Batch) bool { retained = b.Retain(); return true })
	p.Release()
	table.Release()
	if result.Code() != ExecutionCompleted || retained.ActiveLen() != 13 || retained.VectorAt(0).I32s()[5] != 5 {
		t.Fatal("retained result lost")
	}
	retained.Release()
}

func TestParallelTopNParquet(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two GOMAXPROCS")
	}
	pairedScanData(t)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(names))
	for i := range nullable {
		nullable[i] = true
	}
	schema := MakeSchemaWithNullability(names, pairedScanTypes(), nullable)
	rng := rand.New(rand.NewPCG(71, 113))
	for _, name := range []string{"scan_plain.parquet", "scan_snappy.parquet"} {
		path := filepath.Join(externalIOData, "paired", name)
		for trial := range 12 {
			threshold := rng.IntN(15000)
			count, offset := int64(rng.IntN(89)), int64(rng.IntN(17))
			order := MakeOrderKey(MakeColumn("score"))
			if trial%2 == 0 {
				order = order.Desc()
			}
			if trial%3 == 0 {
				order = order.NullsFirst()
			}
			orders := []OrderKey{order}
			if trial%2 != 0 {
				orders = append(orders, MakeOrderKey(MakeColumn("seq")).Desc())
			}
			p, err := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 131}).Filter(MakeColumn("id").Ge(threshold)).Project(MakeColumn("score"), MakeColumn("seq"), MakeColumn("id")).OrderBy(orders...).Limit(count, offset))
			if err != nil {
				t.Fatal(err)
			}
			var want []int32
			for _, workers := range []int{1, 2, 4, 8} {
				var got []int32
				sink := func(b *store.Batch) bool {
					mask := b.Selection().MakeBitMapTemp(a)
					if mask != nil {
						defer mask.Release()
					}
					for row := range b.Len() {
						if mask == nil || mask.IsSet(row) {
							got = append(got, b.VectorAt(2).I32s()[row])
						}
					}
					return true
				}
				var result ExecutionResult
				if workers == 1 {
					result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 1}, sink)
					want = got
				} else {
					var used bool
					result, used = p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: workers}, sink)
					if !used {
						t.Fatal("Parquet TopN did not enter parallel execution")
					}
				}
				if result.Code() != ExecutionCompleted || !slices.Equal(got, want) {
					t.Fatalf("%s trial=%d workers=%d result=%v/%v got=%v want=%v", name, trial, workers, result.Code(), result.Err(), got, want)
				}
			}
			p.Release()
		}
	}
	data, err := os.ReadFile(filepath.Join(externalIOData, "paired", "scan_snappy.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	data[4] = 0xff
	bad := filepath.Join(t.TempDir(), "corrupt.parquet")
	if err := os.WriteFile(bad, data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := MakePhysicalPlan(MakeParquetScan(bad, schema, parquet.ParquetOptions{}).Project(MakeColumn("id")).OrderBy(MakeOrderKey(MakeColumn("id"))).Limit(5))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("failed blocking source reached sink"); return true })
	if result.Code() != ExecutionSourceFailure {
		t.Fatalf("corrupt source result %v/%v", result.Code(), result.Err())
	}
}

func TestParallelTopNAllNullAndDispatch(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two GOMAXPROCS")
	}
	for _, keyType := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT()} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		var batches []*store.Batch
		for chunk := range 8 {
			key := store.MakeVector(a, 2048, keyType, true)
			key.Validity().ClearAll()
			ids := store.MakeVector(a, 2048, dtype.Int32T(), false)
			for row := range ids.I32s() {
				ids.I32s()[row] = int32(chunk*2048 + row)
			}
			batches = append(batches, store.MakeBatch([]store.Vector{key, ids}))
		}
		table := store.MakeTable(batches)
		for _, b := range batches {
			b.Release()
		}
		p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key", "id"}, []dtype.Type{keyType, dtype.Int32T()}, []bool{true, false})).OrderBy(MakeOrderKey(MakeColumn("key")).Desc().NullsFirst()).Limit(19, 7))
		if err != nil {
			t.Fatal(err)
		}
		for _, workers := range []int{0, 2, 4} {
			var got []int32
			result, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: workers}, func(b *store.Batch) bool {
				mask := b.Selection().MakeBitMapTemp(a)
				if mask != nil {
					defer mask.Release()
				}
				for row := range b.Len() {
					if mask == nil || mask.IsSet(row) {
						if b.VectorAt(0).Validity().IsSet(row) {
							t.Error("null validity lost")
						}
						got = append(got, b.VectorAt(1).I32s()[row])
					}
				}
				return true
			})
			if !used || result.Code() != ExecutionCompleted || len(got) != 19 {
				t.Fatalf("%v workers=%d used=%t code=%v len=%d", keyType, workers, used, result.Code(), len(got))
			}
			for i, id := range got {
				if id != int32(i+7) {
					t.Fatalf("%v workers=%d unstable null tie at %d: %d", keyType, workers, i, id)
				}
			}
		}
		p.Release()
		table.Release()
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	var batches []*store.Batch
	for range 2 {
		batches = append(batches, store.MakeBatch([]store.Vector{store.MakeVector(a, 3, dtype.Int32T(), false), store.MakeStringVector(a, [][]byte{[]byte("b"), []byte("a"), []byte("c")}, nil)}))
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	defer table.Release()
	scan := MakeScan(table, MakeSchema([]string{"id", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}))
	for _, query := range []LogicalPlan{
		scan.OrderBy(MakeOrderKey(MakeColumn("id"))).Limit(2),
		scan.OrderBy(MakeOrderKey(MakeColumn("text"))).Limit(2),
		scan.Project(MakeColumn("id")).OrderBy(MakeOrderKey(MakeColumn("id")), MakeOrderKey(MakeColumn("id")), MakeOrderKey(MakeColumn("id"))).Limit(2),
	} {
		p, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		if _, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("unsupported shape reached parallel sink"); return true }); used {
			t.Fatal("unsupported shape accepted")
		}
		rows := 0
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(b *store.Batch) bool { rows += b.ActiveLen(); return true })
		p.Release()
		if result.Code() != ExecutionCompleted || rows != 2 {
			t.Fatalf("fallback code=%v rows=%d", result.Code(), rows)
		}
	}
}
