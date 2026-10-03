package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelDistinctCanonicalOwnershipAndTails(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, cardinality := range []int{7, 16384} {
		for _, sorted := range []bool{false, true} {
			t.Run(fmt.Sprintf("keys=%d/sorted=%t", cardinality, sorted), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				batches := make([]*store.Batch, 8)
				want := make(map[string]bool)
				for chunk := range batches {
					nums := store.MakeVector(a, 2049, dtype.Float64T(), true)
					texts := make([][]byte, 2049)
					selectRows := store.MakeBitMap(a, 2049)
					offsets := []uint32{}
					for row := range 2049 {
						id := chunk*2049 + row
						key := id % cardinality
						nums.F64s()[row] = float64(key)
						if key%19 == 0 {
							nums.Validity().Clear(row)
						} else if key%17 == 0 {
							nums.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(id%256))
						} else if key%13 == 0 {
							nums.F64s()[row] = math.Copysign(0, -1)
						}
						text := fmt.Sprintf("long distinct key \x00 shared binary %08d", key)
						texts[row] = []byte(text)
						if id%5 == 0 {
							continue
						}
						selectRows.Set(row)
						offsets = append(offsets, uint32(row))
						name := "null"
						if nums.Validity().IsSet(row) {
							if math.IsNaN(nums.F64s()[row]) {
								name = "nan"
							} else {
								name = fmt.Sprint(nums.F64s()[row])
							}
						}
						want[name+"/"+text] = true
					}
					batch := store.MakeBatch([]store.Vector{nums, store.MakeStringVector(a, texts, nil)})
					if chunk%2 == 0 {
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selectRows))
					} else {
						selectRows.Release()
						batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 2049, offsets)))
					}
					batches[chunk] = batch
				}
				table := store.MakeTable(batches)
				for _, batch := range batches {
					batch.Release()
				}
				schema := MakeSchemaWithNullability([]string{"x", "s"}, []dtype.Type{dtype.Float64T(), dtype.StringT()}, []bool{true, false})
				logical := MakeScan(table, schema).Distinct()
				if sorted {
					logical = logical.OrderBy(MakeOrderKey(MakeColumn("s")), MakeOrderKey(MakeColumn("x"))).Limit(17)
				}
				p, err := MakePhysicalPlan(logical)
				if err != nil {
					t.Fatal(err)
				}
				original := append([]physicalStep(nil), p.steps...)
				var output []*store.Batch
				result, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: 4}, func(batch *store.Batch) bool { output = append(output, batch.Retain()); return true })
				if !parallel || result.Code() != ExecutionCompleted {
					t.Fatalf("parallel=%t result=%v/%v", parallel, result.Code(), result.Err())
				}
				for i := range p.steps {
					if p.steps[i].operation != original[i].operation || (p.steps[i].schema.Len() != original[i].schema.Len() || p.steps[i].schema.Len() > 0 && p.steps[i].schema.FieldAt(0).ID() != original[i].schema.FieldAt(0).ID()) {
						t.Fatal("mutated plan")
					}
				}
				p.Release()
				table.Release()
				seen := make(map[string]bool)
				prior := ""
				for _, batch := range output {
					for row := range batch.Len() {
						name := "null"
						v := batch.VectorAt(0)
						if v.Validity() == nil || v.Validity().IsSet(row) {
							if math.IsNaN(v.F64s()[row]) {
								name = "nan"
							} else {
								name = fmt.Sprint(v.F64s()[row])
							}
						}
						text := batch.VectorAt(1).StringAt(row).View()
						key := name + "/" + text
						if !want[key] || seen[key] {
							t.Errorf("unexpected/duplicate retained key %q", key)
						}
						seen[key] = true
						if sorted && text < prior {
							t.Error("unordered output")
						}
						prior = text
					}
					batch.Release()
				}
				expected := len(want)
				if sorted {
					expected = min(expected, 17)
				}
				if len(seen) != expected {
					t.Fatalf("got %d keys want %d", len(seen), expected)
				}
				usage := a.Usage()
				if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked allocator payload: %+v", usage)
				}
			})
		}
	}
}

func TestParallelDistinctGlobalAggregates(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, input := range []string{"normal", "null", "empty"} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		batches := make([]*store.Batch, 4)
		for chunk := range batches {
			n := 257
			if input == "empty" {
				n = 0
			}
			v := store.MakeVector(a, n, dtype.Float64T(), true)
			for row := range n {
				id := chunk*n + row
				v.F64s()[row] = float64(id%11 - 5)
				if id%13 == 0 {
					v.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(id%256))
				}
				if id%17 == 0 {
					v.F64s()[row] = math.Copysign(0, -1)
				}
				if input == "null" || id%19 == 0 {
					v.Validity().Clear(row)
				}
			}
			batches[chunk] = store.MakeBatch([]store.Vector{v})
		}
		table := store.MakeTable(batches)
		for _, b := range batches {
			b.Release()
		}
		schema := MakeSchemaWithNullability([]string{"x"}, []dtype.Type{dtype.Float64T()}, []bool{true})
		for _, aggregate := range []Aggregate{MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeMin(MakeColumn("x")).Distinct(), MakeMax(MakeColumn("x")).Distinct(), MakeAvg(MakeColumn("x")).Distinct()} {
			p, err := MakePhysicalPlan(MakeScan(table, schema).Aggregate(aggregate))
			if err != nil {
				t.Fatal(err)
			}
			var expected Scalar
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 1}, func(b *store.Batch) bool { expected = scalarAt(b.VectorAt(0), 0); return true })
			if result.Code() != ExecutionCompleted {
				t.Fatal(result.Code())
			}
			called := false
			result, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(b *store.Batch) bool {
				called = true
				actual := scalarAt(b.VectorAt(0), 0)
				if actual.IsNull() != expected.IsNull() || actual.Type() != expected.Type() || actual.bits != expected.bits {
					t.Errorf("%s aggregate=%d actual=%+v expected=%+v", input, aggregate.kind, actual, expected)
				}
				return true
			})
			if input == "empty" {
				if parallel {
					t.Fatal("empty source should use fallback")
				}
			} else if !parallel || !called || result.Code() != ExecutionCompleted {
				t.Fatalf("%s: %t %t %v", input, parallel, called, result.Code())
			}
			p.Release()
		}
		table.Release()
		usage := a.Usage()
		if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatalf("leaked %+v", usage)
		}
	}
}

func TestParallelDistinctResourceStopCancellationAndFallback(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	for chunk := range batches {
		texts := make([][]byte, 4096)
		for row := range texts {
			texts[row] = []byte(fmt.Sprintf("unique long budget and cancellation key %08d", chunk*4096+row))
		}
		batches[chunk] = store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil)})
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	schema := MakeSchemaWithNullability([]string{"s"}, []dtype.Type{dtype.StringT()}, []bool{false})
	p, err := MakePhysicalPlan(MakeScan(table, schema).Distinct())
	if err != nil {
		t.Fatal(err)
	}
	baseline := a.Usage()
	result, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 2 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("budget failure reached sink"); return true })
	if !parallel || result.Code() != ExecutionResourceExhausted {
		t.Fatalf("budget %t/%v", parallel, result.Code())
	}
	calls := 0
	result, parallel = p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: 4}, func(*store.Batch) bool { calls++; return false })
	if !parallel || result.Code() != ExecutionStopped || calls != 1 {
		t.Fatalf("stop %t/%v calls=%d", parallel, result.Code(), calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, parallel = p.executeParallelDistinct(ctx, a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("cancelled query reached sink"); return true })
	if !parallel || result.Code() != ExecutionCancelled {
		t.Fatalf("cancel %t/%v", parallel, result.Code())
	}
	ctx, cancel = context.WithCancel(context.Background())
	result, parallel = p.executeParallelDistinct(ctx, a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: 4}, func(*store.Batch) bool { cancel(); return true })
	if !parallel || result.Code() != ExecutionCancelled {
		t.Fatalf("sink cancellation %t/%v", parallel, result.Code())
	}
	usage := a.Usage()
	if usage.GeneralUsed != baseline.GeneralUsed || usage.ScratchUsed != baseline.ScratchUsed {
		t.Fatalf("temporary payload leaked %+v baseline %+v", usage, baseline)
	}
	p.Release()
	for _, logical := range []LogicalPlan{
		MakeScan(table, schema).Aggregate(MakeCountDistinct(MakeColumn("s")), MakeCountStar()),
		MakeScan(table, schema).Distinct().Aggregate(MakeCountDistinct(MakeColumn("s"))),
	} {
		p, err := MakePhysicalPlan(logical)
		if err != nil {
			t.Fatal(err)
		}
		if _, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("unsupported tail executed"); return true }); parallel {
			t.Fatal("unsupported shape did not fall back")
		}
		p.Release()
	}
	table.Release()
	usage = a.Usage()
	if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestParallelDistinctDictionaryRandomized(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, allNull := range []bool{false, true} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		rng := rand.New(rand.NewPCG(753, 191))
		texts := [][]byte{[]byte(""), []byte("\x00"), []byte("ø\x00東京"), []byte("long owned dictionary payload key"), []byte("long owned dictionary payload key"), []byte("\xff\xfe binary")}
		want := make(map[string]bool)
		batches := make([]*store.Batch, 7)
		for chunk := range batches {
			dictionary := store.MakeStringVector(a, texts, nil)
			indices := a.AllocSeg(513 * 4)
			valid := store.MakeBitMap(a, 513)
			selected := store.MakeBitMap(a, 513)
			for row := range 513 {
				id := rng.IntN(len(texts))
				indices.AsU32T()[row] = uint32(id)
				isNull := allNull || rng.IntN(7) == 0
				if !isNull {
					valid.Set(row)
				} else {
					indices.AsU32T()[row] = math.MaxUint32
				}
				if rng.IntN(5) != 0 {
					selected.Set(row)
					if isNull {
						want["null"] = true
					} else {
						want["value/"+string(texts[id])] = true
					}
				}
			}
			v := store.MakeDictionaryStringVectorFromOwnedSegments(dictionary, 513, indices, valid)
			batches[chunk] = store.MakeBatch([]store.Vector{v})
			batches[chunk].SetSelection(store.MakeRowSelectionFromBitMap(selected))
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"s"}, []dtype.Type{dtype.StringT()}, []bool{true})).Distinct())
		if err != nil {
			t.Fatal(err)
		}
		var retained *store.Batch
		result, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(batch *store.Batch) bool {
			if retained != nil {
				t.Error("unexpected multiple low-cardinality outputs")
			}
			retained = batch.Retain()
			return true
		})
		p.Release()
		table.Release()
		if !parallel || result.Code() != ExecutionCompleted || retained == nil {
			t.Fatalf("dictionary %t/%v", parallel, result.Code())
		}
		seen := make(map[string]bool)
		for row := range retained.Len() {
			key := "null"
			v := retained.VectorAt(0)
			if v.Validity() == nil || v.Validity().IsSet(row) {
				key = "value/" + v.StringAt(row).View()
			}
			if !want[key] || seen[key] {
				t.Errorf("unexpected key %q", key)
			}
			seen[key] = true
		}
		retained.Release()
		if len(seen) != len(want) {
			t.Fatalf("got %d keys want %d", len(seen), len(want))
		}
		usage := a.Usage()
		if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatalf("leaked %+v", usage)
		}
	}
}

func TestParallelDistinctTerminalAggregate(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, mode := range []string{"normal", "all-null", "filtered-empty", "empty"} {
		t.Run(mode, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			type referenceKey struct {
				x               int32
				text            string
				xNull, textNull bool
			}
			unique := make(map[referenceKey]bool)
			batches := make([]*store.Batch, 8)
			for chunk := range batches {
				n := 513
				if mode == "empty" {
					n = 0
				}
				ints := store.MakeVector(a, n, dtype.Int32T(), true)
				strings := make([][]byte, n)
				valid := make([]bool, n)
				selection := store.MakeBitMap(a, n)
				for row := range n {
					id := chunk*n + row
					ints.I32s()[row] = int32(id%17 - 8)
					if mode == "all-null" || id%11 == 0 {
						ints.Validity().Clear(row)
					}
					strings[row] = []byte(fmt.Sprintf("retained distinct reducer \x00\xff key %02d", id%7))
					valid[row] = mode != "all-null" && id%13 != 0
					if id%5 != 0 {
						selection.Set(row)
						if mode != "filtered-empty" {
							key := referenceKey{x: ints.I32s()[row], text: string(strings[row]), xNull: !ints.Validity().IsSet(row), textNull: !valid[row]}
							if key.xNull {
								key.x = 0
							}
							if key.textNull {
								key.text = ""
							}
							unique[key] = true
						}
					}
				}
				batches[chunk] = store.MakeBatch([]store.Vector{ints, store.MakeStringVector(a, strings, valid)})
				batches[chunk].SetSelection(store.MakeRowSelectionFromBitMap(selection))
			}
			table := store.MakeTable(batches)
			for _, batch := range batches {
				batch.Release()
			}
			scan := MakeScan(table, MakeSchemaWithNullability([]string{"x", "s"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true}))
			if mode == "filtered-empty" {
				scan = scan.Filter(MakeColumn("x").Lt(-1000))
			}
			query := scan.Distinct().Aggregate(MakeCountStar().Alias("rows"), MakeCount(MakeColumn("x")), MakeSum(MakeColumn("x")), MakeAvg(MakeColumn("x")), MakeMin(MakeColumn("s")), MakeMax(MakeColumn("s")), MakeCountStar().Alias("rows_again"), MakeSum(MakeColumn("x").Add(3)))
			p, err := MakePhysicalPlan(query)
			if err != nil {
				t.Fatal(err)
			}
			expected := make([]Scalar, 8)
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 1}, func(batch *store.Batch) bool {
				for c := range expected {
					expected[c] = scalarAt(batch.VectorAt(c), 0)
					if expected[c].text != "" {
						expected[c].text = string(append([]byte(nil), expected[c].text...))
					}
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				t.Fatalf("serial %v/%v", result.Code(), result.Err())
			}
			var nonNull, sum, expressionSum int64
			var minText, maxText string
			stringsSeen := false
			for key := range unique {
				if !key.xNull {
					nonNull++
					sum += int64(key.x)
					expressionSum += int64(key.x + 3)
				}
				if !key.textNull {
					if !stringsSeen || key.text < minText {
						minText = key.text
					}
					if !stringsSeen || key.text > maxText {
						maxText = key.text
					}
					stringsSeen = true
				}
			}
			want := []Scalar{MakeI64Scalar(int64(len(unique))), MakeI64Scalar(nonNull), MakeI64Scalar(sum), MakeF64Scalar(0), MakeStringScalar(minText), MakeStringScalar(maxText), MakeI64Scalar(int64(len(unique))), MakeI64Scalar(expressionSum)}
			if nonNull == 0 {
				want[2], want[3], want[7] = MakeNullScalar(dtype.Int64T()), MakeNullScalar(dtype.Float64T()), MakeNullScalar(dtype.Int64T())
			} else {
				want[3] = MakeF64Scalar(float64(sum) / float64(nonNull))
			}
			if !stringsSeen {
				want[4], want[5] = MakeNullScalar(dtype.StringT()), MakeNullScalar(dtype.StringT())
			}
			for i, value := range expected {
				if value.IsNull() != want[i].IsNull() || !value.IsNull() && (value.bits != want[i].bits || value.text != want[i].text) {
					t.Errorf("scalar reference column=%d serial=%+v want=%+v", i, value, want[i])
				}
			}
			var retained *store.Batch
			result, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(batch *store.Batch) bool { retained = batch.Retain(); return true })
			if mode == "empty" {
				if parallel {
					t.Fatal("empty source unexpectedly parallel")
				}
				result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(batch *store.Batch) bool { retained = batch.Retain(); return true })
			} else if !parallel {
				t.Fatal("terminal DISTINCT reduction fell back")
			}
			p.Release()
			table.Release()
			if result.Code() != ExecutionCompleted || retained == nil || retained.Len() != 1 {
				t.Fatalf("parallel reduction %v/%v", result.Code(), result.Err())
			}
			for column, want := range expected {
				got := scalarAt(retained.VectorAt(column), 0)
				if got.IsNull() != want.IsNull() || got.Type() != want.Type() || !got.IsNull() && (got.bits != want.bits || got.text != want.text) {
					t.Errorf("column=%d got=%+v want=%+v", column, got, want)
				}
			}
			retained.Release()
			usage := a.Usage()
			if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("leaked %+v", usage)
			}
		})
	}
}

func TestParallelDistinctSingleStringReductionOwnership(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, kind := range []AggregateKind{AggregateMin, AggregateMax} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		texts := [][]byte{[]byte("z long retained string key \xff"), []byte("a long retained string key \x00"), []byte("a long retained string key \x00"), nil}
		batches := make([]*store.Batch, 4)
		for i := range batches {
			batches[i] = store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, []bool{true, true, true, false})})
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		aggregate := MakeMin(MakeColumn("s")).Distinct()
		want := string(texts[1])
		if kind == AggregateMax {
			aggregate = MakeMax(MakeColumn("s")).Distinct()
			want = string(texts[0])
		}
		p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"s"}, []dtype.Type{dtype.StringT()}, []bool{true})).Aggregate(aggregate))
		if err != nil {
			t.Fatal(err)
		}
		var retained *store.Batch
		result, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(batch *store.Batch) bool { retained = batch.Retain(); return true })
		p.Release()
		table.Release()
		if !parallel || result.Code() != ExecutionCompleted || retained == nil {
			t.Fatalf("string reduction %t/%v", parallel, result.Code())
		}
		if got := retained.VectorAt(0).StringAt(0).View(); got != want {
			t.Errorf("retained reduction got=%q want=%q", got, want)
		}
		retained.Release()
		usage := a.Usage()
		if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatalf("leaked %+v", usage)
		}
	}
}
