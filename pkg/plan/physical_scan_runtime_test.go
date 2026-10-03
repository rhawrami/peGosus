package plan

import (
	"bytes"
	"context"
	"encoding/base64"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func runtimeFilterScan(t testing.TB, a *mem.Allocator, fixture ...string) LogicalPlan {
	t.Helper()
	input := "../io/parquet/testdata/duckdb-predicates.b64"
	if len(fixture) != 0 {
		input = fixture[0]
	}
	encoded, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	reader, failure := parquet.MakeParquetReader(bytes.NewReader(data), int64(len(data)), a, parquet.ParquetOptions{})
	if failure != nil {
		t.Fatal(failure)
	}
	names, types, nullable := make([]string, reader.ColumnCount()), make([]dtype.Type, reader.ColumnCount()), make([]bool, reader.ColumnCount())
	for i := range names {
		names[i], types[i], nullable[i] = reader.Column(i)
	}
	reader.Close()
	path := filepath.Join(t.TempDir(), "runtime.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return MakeParquetScan(path, MakeSchemaWithNullability(names, types, nullable), parquet.ParquetOptions{BatchSize: 317})
}

func TestRuntimeTopNUnfilledAndAllNull(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scan := runtimeFilterScan(t, a, "testdata/duckdb-runtime-topn.b64")
	for _, allNull := range []bool{false, true} {
		for _, nullsFirst := range []bool{false, true} {
			for _, limit := range []int64{9, 10000, 20000} {
				query := scan
				if allNull {
					query = query.Filter(MakeColumn("bucket").IsNull())
				}
				order := MakeOrderKey(MakeColumn("bucket"))
				if nullsFirst {
					order = order.NullsFirst()
				}
				p, err := MakePhysicalPlan(query.Project(MakeColumn("id"), MakeColumn("bucket")).OrderBy(order, MakeOrderKey(MakeColumn("id")).Desc()).Limit(limit, 3))
				if err != nil {
					t.Fatal(err)
				}
				var want []int32
				if allNull || nullsFirst {
					for id := 10240; id >= 8192; id-- {
						want = append(want, int32(id))
					}
				}
				if !allNull {
					for _, start := range []int{4095, 8191} {
						for id := start; id > start-4096; id-- {
							want = append(want, int32(id))
						}
					}
					if !nullsFirst {
						for id := 10240; id >= 8192; id-- {
							want = append(want, int32(id))
						}
					}
				}
				want = want[3:min(int64(len(want)), limit+3)]
				for _, workers := range []int{1, 4} {
					var got []int32
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: workers}, func(batch *store.Batch) bool {
						mask := batch.Selection().MakeBitMapTemp(a)
						if mask != nil {
							defer mask.Release()
						}
						for row, id := range batch.VectorAt(0).I32s() {
							if mask == nil || mask.IsSet(row) {
								got = append(got, id)
							}
						}
						return true
					})
					if result.Code() != ExecutionCompleted || !slices.Equal(got, want) {
						t.Fatalf("allNull=%t nullsFirst=%t limit=%d workers=%d result=%v/%v rows=%d want=%d", allNull, nullsFirst, limit, workers, result.Code(), result.Err(), len(got), len(want))
					}
				}
				p.Release()
			}
		}
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("runtime nulls query retained bytes: %+v", usage)
	}
}

// DuckDB 1.5.6, Snappy, six groups; primary-key ties span groups and the
// final groups contain only null primary keys.
func TestRuntimeTopNCompoundTiesNullsAndFilters(t *testing.T) {
	encoded, err := os.ReadFile("testdata/duckdb-runtime-topn.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ties.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	schema := MakeSchemaWithNullability([]string{"id", "bucket", "keep"}, []dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.BoolT()}, []bool{true, true, true})
	for _, descending := range []bool{false, true} {
		for _, nullsFirst := range []bool{false, true} {
			for _, filtered := range []bool{false, true} {
				var want []int32
				for id := range 10241 {
					if !filtered || id%5 != 0 {
						want = append(want, int32(id))
					}
				}
				sort.SliceStable(want, func(i, j int) bool {
					l, r := want[i], want[j]
					ln, rn := l >= 8192, r >= 8192
					if ln != rn {
						return ln == nullsFirst
					}
					if !ln && l/4096 != r/4096 {
						if descending {
							return l/4096 > r/4096
						}
						return l/4096 < r/4096
					}
					return l > r
				})
				want = want[3:12]
				query := MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 37})
				if filtered {
					query = query.Filter(MakeColumn("keep"))
				}
				order := MakeOrderKey(MakeColumn("bucket"))
				if descending {
					order = order.Desc()
				}
				if nullsFirst {
					order = order.NullsFirst()
				}
				p, err := MakePhysicalPlan(query.Project(MakeColumn("id"), MakeColumn("bucket").Cast(dtype.Int64T()).Alias("bucket")).OrderBy(order, MakeOrderKey(MakeColumn("id")).Desc()).Limit(9, 3))
				if err != nil {
					t.Fatal(err)
				}
				for _, workers := range []int{1, 2, 4} {
					var got []int32
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: workers}, func(batch *store.Batch) bool {
						mask := batch.Selection().MakeBitMapTemp(a)
						if mask != nil {
							defer mask.Release()
							for row, id := range batch.VectorAt(0).I32s() {
								if mask.IsSet(row) {
									got = append(got, id)
								}
							}
						} else {
							got = append(got, batch.VectorAt(0).I32s()...)
						}
						return true
					})
					if result.Code() != ExecutionCompleted || !slices.Equal(got, want) {
						t.Fatalf("desc=%t nullsFirst=%t filtered=%t workers=%d result=%v/%v got=%v want=%v", descending, nullsFirst, filtered, workers, result.Code(), result.Err(), got, want)
					}
				}
				p.Release()
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("runtime query leaked payload: %+v", usage)
				}
			}
		}
	}
}

func TestTopNRuntimePrunesUnreadGroups(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scan := runtimeFilterScan(t, a)
	keyName := scan.root.schema.FieldAt(0).Name()
	otherName := scan.root.schema.FieldAt(1).Name()
	p, err := MakePhysicalPlan(scan.Project(MakeColumn(otherName), MakeColumn(keyName)).OrderBy(MakeOrderKey(MakeColumn(keyName)), MakeOrderKey(MakeColumn(otherName)).Desc()).Limit(10, 3))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	filter, at := makeTopNRuntimeFilter(p.source, p.steps)
	if filter == nil || at < 0 {
		t.Fatal("direct reordered projection did not produce runtime filter")
	}
	scope := mem.MakeAllocationScope(a, 8<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	top := makeCompactTopNState(p.steps[at])
	top.scope, top.rows.scope = scope, scope
	cursor := scanCursor{source: p.source, runtime: filter}
	defer cursor.close()
	rows := 0
	for {
		batch, done, err := cursor.next(context.Background(), scoped)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		rows += batch.ActiveLen()
		for _, step := range p.steps[:at] {
			if step.operation == physicalProject {
				projected := executePhysicalProject(scoped, batch, step.program)
				batch.Release()
				if projected == nil {
					t.Fatal("projection")
				}
				batch = projected
			}
		}
		if !top.add(scoped, batch, p.steps[at], 8<<20) {
			t.Fatal("TopN add")
		}
		batch.Release()
		publishTopNRuntimeFilter(filter, top, p.steps[at])
	}
	output := top.finish(scoped, p.steps[at])
	if output == nil || output.Len() != 13 {
		t.Fatal("TopN output")
	}
	for i, key := range output.VectorAt(1).I32s() {
		if key != int32(i) {
			t.Fatalf("key=%d want=%d", key, i)
		}
	}
	if rows != 2048 || filter.PrunedRowGroups() != 2 {
		t.Fatalf("decoded rows=%d pruned groups=%d", rows, filter.PrunedRowGroups())
	}
	output.Release()
	top.release()
	cursor.close()
	if scope.Live() != 0 {
		t.Fatalf("runtime TopN leaked %d bytes", scope.Live())
	}
}

func TestTopNRuntimeBoundsDecodeAndEligibility(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scan := runtimeFilterScan(t, a)
	name := scan.root.schema.FieldAt(0).Name()
	for _, key := range []Expr{MakeColumn(name).Add(1), MakeColumn(name).Cast(dtype.Float64T())} {
		p, err := MakePhysicalPlan(scan.Project(key.Alias("key")).OrderBy(MakeOrderKey(MakeColumn("key"))).Limit(3))
		if err != nil {
			t.Fatal(err)
		}
		filter, _ := makeTopNRuntimeFilter(p.source, p.steps)
		p.Release()
		if filter != nil {
			t.Fatal("computed or floating key was pushed into runtime scan bounds")
		}
	}
	p, err := MakePhysicalPlan(scan.Limit(20).Project(MakeColumn(name)).OrderBy(MakeOrderKey(MakeColumn(name))).Limit(3))
	if err != nil {
		t.Fatal(err)
	}
	filter, _ := makeTopNRuntimeFilter(p.source, p.steps)
	p.Release()
	if filter != nil {
		t.Fatal("runtime scan filter crossed an earlier limit")
	}
	for _, typ := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.DateT(), dtype.TimestampTZT()} {
		for _, descending := range []bool{false, true} {
			for _, value := range []int64{math.MinInt32, -1, 0, 1, math.MaxInt32} {
				vector := store.MakeVector(a, 1, typ, false)
				if typ.ID() == dtype.INT32T || typ.ID() == dtype.DATET {
					vector.I32s()[0] = int32(value)
				} else {
					vector.I64s()[0] = value
				}
				encoded, _ := encodeNumericOrderKey(&vector, 0, descending)
				segment := a.AllocSeg(compactTopNItemBytes)
				top := &compactTopNState{items: segment, capacity: 1, limit: 1, length: 1}
				top.slice()[0].key = encoded
				step := physicalStep{order: []physicalOrderKey{{descending: descending, program: physicalExprProgram{nodes: []physicalExprNode{{dType: typ}}, roots: []int{0}}}}}
				f := parquet.MakeRuntimePruningFilter(0, false)
				publishTopNRuntimeFilter(f, top, step)
				cursor := scanCursor{source: &scanSource{parquetPath: scan.root.parquetPath, schema: scan.root.schema, projection: []int{0}}, runtime: f}
				rows := 0
				for {
					batch, done, err := cursor.next(context.Background(), a)
					if err != nil {
						t.Fatal(err)
					}
					if done {
						break
					}
					rows += batch.Len()
					batch.Release()
				}
				cursor.close()
				if value < 0 && !descending || value == math.MaxInt32 && descending {
					if rows != 0 || f.PrunedRowGroups() != 3 {
						t.Fatalf("type=%s desc=%t value=%d rows=%d pruned=%d", typ, descending, value, rows, f.PrunedRowGroups())
					}
				} else if value < 0 && descending || value == math.MaxInt32 && !descending {
					if f.PrunedRowGroups() != 0 {
						t.Fatal("wrong decoded signed bound")
					}
				} else if !descending && f.PrunedRowGroups() != 2 {
					t.Fatal("ascending bound rejected a primary-key tie group")
				}
				top.release()
				vector.Release()
			}
		}
	}
}

func TestJoinRuntimeBoundsAndCancellation(t *testing.T) {
	for _, kind := range []JoinKind{JoinInner, JoinSemi, JoinLeft, JoinFull, JoinAnti} {
		for variant := range 3 {
			empty, widening := variant == 1, variant == 2
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			scan := runtimeFilterScan(t, a)
			name := scan.root.schema.FieldAt(0).Name()
			keyType := dtype.Int32T()
			if widening {
				keyType = dtype.Int64T()
			}
			key := store.MakeVector(a, 5, keyType, true)
			if widening {
				copy(key.I64s(), []int64{2048, 2050, 2050, math.MinInt64, math.MaxInt64})
			} else {
				copy(key.I32s(), []int32{2048, 2050, 2050, math.MinInt32, math.MaxInt32})
			}
			key.Validity().Clear(3)
			selection := store.MakeBitMap(a, 5)
			if !empty {
				selection.Set(0)
				selection.Set(1)
				selection.Set(2)
				selection.Set(3)
			}
			batch := store.MakeBatch([]store.Vector{key})
			batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
			table := store.MakeTable([]*store.Batch{batch})
			batch.Release()
			right := MakeScan(table, MakeSchemaWithNullability([]string{"key"}, []dtype.Type{keyType}, []bool{true})).As("r")
			p, err := MakePhysicalPlan(scan.As("l").Join(right, kind, []Expr{MakeColumn("l." + name)}, []Expr{MakeColumn("r.key")}, MakeColumn("r.key").Lt(2050)))
			if err != nil {
				t.Fatal(err)
			}
			at := -1
			for i, step := range p.steps {
				if step.operation == physicalJoin {
					at = i
				}
			}
			state := &joinState{}
			result := state.build(context.Background(), a, p.steps[at].join, 8<<20)
			if result.Code() != ExecutionCompleted {
				t.Fatalf("build: %v/%v", result.Code(), result.Err())
			}
			filter, err := makeJoinRuntimeFilter(context.Background(), p.source, p.steps, at, state)
			if err != nil {
				t.Fatal(err)
			}
			if kind != JoinInner && kind != JoinSemi {
				if filter != nil {
					t.Fatal("preserved-side join received a scan runtime filter")
				}
			} else {
				if filter == nil {
					t.Fatal("eligible join did not receive a runtime filter")
				}
				cursor := scanCursor{source: p.source, runtime: filter}
				rows := 0
				for {
					batch, done, err := cursor.next(context.Background(), a)
					if err != nil {
						t.Fatal(err)
					}
					if done {
						break
					}
					rows += batch.Len()
					batch.Release()
				}
				cursor.close()
				wantRows, wantGroups := 2048, int64(2)
				if empty {
					wantRows, wantGroups = 0, 3
				}
				if rows != wantRows || filter.PrunedRowGroups() != wantGroups {
					t.Fatalf("kind=%d empty=%t decoded=%d pruned=%d", kind, empty, rows, filter.PrunedRowGroups())
				}
				if !empty {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					if filter, err := makeJoinRuntimeFilter(ctx, p.source, p.steps, at, state); filter != nil || err != context.Canceled {
						t.Fatalf("cancelled bound construction: filter=%v err=%v", filter, err)
					}
				}
			}
			state.release()
			p.Release()
			table.Release()
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("join runtime retained bytes: %+v", usage)
			}
		}
	}
}

func TestJoinTopNParallelEligibility(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	var batches []*store.Batch
	for i, keys := range [][]int32{{1, 1}, {2}, {3}} {
		key := store.MakeVector(a, len(keys), dtype.Int32T(), false)
		id := store.MakeVector(a, len(keys), dtype.Int32T(), false)
		copy(key.I32s(), keys)
		for row := range keys {
			id.I32s()[row] = int32(i*10 + row)
		}
		batches = append(batches, store.MakeBatch([]store.Vector{key, id}))
	}
	leftTable := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	key := store.MakeVector(a, 3, dtype.Int32T(), false)
	score := store.MakeVector(a, 3, dtype.Int32T(), false)
	copy(key.I32s(), []int32{1, 1, 4})
	copy(score.I32s(), []int32{5, 6, 99})
	rightBatch := store.MakeBatch([]store.Vector{key, score})
	rightTable := store.MakeTable([]*store.Batch{rightBatch})
	rightBatch.Release()
	left := MakeScan(leftTable, MakeSchema([]string{"key", "id"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()})).As("l")
	right := MakeScan(rightTable, MakeSchema([]string{"key", "score"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()})).As("r")
	for _, kind := range []JoinKind{JoinInner, JoinFull} {
		p, err := MakePhysicalPlan(left.Join(right, kind, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")}).OrderBy(MakeOrderKey(MakeColumn("r.score")).Desc()).Limit(4))
		if err != nil {
			t.Fatal(err)
		}
		if result, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(*store.Batch) bool { return true }); !used || result.Code() != ExecutionCompleted {
			t.Fatalf("join TopN parallel=%t result=%v", used, result.Code())
		}
		var want []Scalar
		for _, workers := range []int{1, 4} {
			var got []Scalar
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: workers}, func(batch *store.Batch) bool {
				mask := batch.Selection().MakeBitMapTemp(a)
				if mask != nil {
					defer mask.Release()
				}
				for row := range batch.Len() {
					if mask != nil && !mask.IsSet(row) {
						continue
					}
					for column := range batch.NVectors() {
						got = append(got, scalarAt(batch.VectorAt(column), row))
					}
				}
				return true
			})
			if workers == 1 {
				want = got
			}
			if result.Code() != ExecutionCompleted || len(got) != 16 || !slices.Equal(got, want) {
				t.Fatalf("join=%d workers=%d result=%v/%v got=%v want=%v", kind, workers, result.Code(), result.Err(), got, want)
			}
		}
		p.Release()
	}
	leftTable.Release()
	rightTable.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("fallback join TopN leaked payload: %+v", usage)
	}
}
