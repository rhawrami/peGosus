package plan

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestSpillSortDifferentialBudgetAndOwnership(t *testing.T) {
	for mode := range 8 {
		t.Run(fmt.Sprintf("mode=%d", mode), func(t *testing.T) {
			const rows = 2049
			a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
			random := rand.New(rand.NewPCG(771, uint64(mode+8)))
			ids := store.MakeVector(a, rows, dtype.Int64T(), false)
			number := store.MakeVector(a, rows, dtype.Float64T(), true)
			flag := store.MakeVector(a, rows, dtype.BoolT(), true)
			texts, valid := make([][]byte, rows), make([]bool, rows)
			samples := []string{"", "a", "a\x00", "a\x00\x00", "abcdefghijklmno long payload a", "abcdefghijklmno long payload b", "ø雪\x00tail"}
			for row := range rows {
				ids.I64s()[row] = int64(row)
				number.F64s()[row] = float64(random.IntN(15) - 7)
				switch row % 29 {
				case 0:
					number.F64s()[row] = math.NaN()
				case 1:
					number.F64s()[row] = math.Inf(-1)
				case 2:
					number.F64s()[row] = math.Inf(1)
				case 3:
					number.F64s()[row] = math.Copysign(0, -1)
				}
				if row%17 == 0 {
					number.Validity().Clear(row)
				}
				if row%3 != 0 {
					flag.Bools()[row>>3] |= 1 << uint(row&7)
				}
				if row%19 == 0 {
					flag.Validity().Clear(row)
				}
				texts[row] = []byte(samples[random.IntN(len(samples))])
				valid[row] = row%13 != 0
			}
			batch := store.MakeBatch([]store.Vector{ids, number, store.MakeStringVector(a, texts, valid), flag})
			if mode&1 != 0 {
				mask := store.MakeBitMap(a, rows)
				for row := range rows {
					if row%7 != 0 {
						mask.Set(row)
					}
				}
				batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
			} else if mode&2 != 0 {
				var selected []uint32
				for row := range rows {
					if row%5 != 0 {
						selected = append(selected, uint32(row))
					}
				}
				batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, rows, selected)))
			}
			table := store.MakeTable([]*store.Batch{batch})
			batch.Release()
			scan := MakeScan(table, MakeSchemaWithNullability([]string{"id", "number", "text", "flag"}, []dtype.Type{dtype.Int64T(), dtype.Float64T(), dtype.StringT(), dtype.BoolT()}, []bool{false, true, true, true}))
			key := MakeOrderKey(MakeColumn("number"))
			if mode&1 != 0 {
				key = key.Desc()
			}
			if mode&2 != 0 {
				key = key.NullsFirst()
			}
			textKey := MakeOrderKey(MakeColumn("text"))
			if mode&4 != 0 {
				textKey = MakeOrderKey(MakeColumn("text").Concat("\x00computed"))
				scan = scan.Filter(MakeColumn("flag"))
			}
			query := scan.OrderBy(key, textKey)
			if mode == 7 {
				query = query.Limit(700, 3)
			}
			p, err := MakePhysicalPlan(query)
			if err != nil {
				t.Fatal(err)
			}
			var want, got []int64
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 1}, func(output *store.Batch) bool {
				mask, _ := output.Selection().AsBitMap()
				for row := range output.Len() {
					if mask == nil || mask.IsSet(row) {
						want = append(want, output.VectorAt(0).I64s()[row])
					}
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				t.Fatal(result.Code(), result.Err())
			}
			directory := t.TempDir()
			scope := mem.MakeAllocationScope(a, 128<<10)
			var held *store.Batch
			var heldString string
			observed := false
			result = p.executeWithScope(context.Background(), a, ExecutionOptions{MemoryBudget: scope.Limit(), SpillDirectory: directory, Workers: 4}, func(output *store.Batch) bool {
				files, _ := os.ReadDir(directory)
				observed = observed || len(files) != 0
				mask, _ := output.Selection().AsBitMap()
				for row := range output.Len() {
					if mask == nil || mask.IsSet(row) {
						id := output.VectorAt(0).I64s()[row]
						got = append(got, id)
						if math.Float64bits(output.VectorAt(1).F64s()[row]) != math.Float64bits(table.BatchAt(0).VectorAt(1).F64s()[id]) && (table.BatchAt(0).VectorAt(1).Validity() == nil || table.BatchAt(0).VectorAt(1).Validity().IsSet(int(id))) {
							t.Fatalf("float bits changed id=%d", id)
						}
						if valid[id] && output.VectorAt(2).StringAt(row).View() != string(texts[id]) {
							t.Fatal("payload changed")
						}
					}
				}
				if held == nil && output.Len() != 0 {
					held = output.Retain()
					heldString = output.VectorAt(2).StringAt(0).View()
				}
				return true
			}, scope)
			if result.Code() != ExecutionCompleted {
				t.Fatalf("spill result=%v error=%v live=%d peak=%d", result.Code(), result.Err(), scope.Live(), scope.Peak())
			}
			if !observed || !slices.Equal(got, want) {
				t.Fatalf("external output mismatch: observed=%v got=%d want=%d", observed, len(got), len(want))
			}
			if scope.Peak() > scope.Limit() {
				t.Fatal("budget exceeded")
			}
			files, _ := os.ReadDir(directory)
			if len(files) != 0 {
				t.Fatal("temporary files retained")
			}
			p.Release()
			table.Release()
			if held != nil {
				if held.VectorAt(2).StringAt(0).View() != heldString {
					t.Fatal("borrowed output backing")
				}
				held.Release()
			}
			if scope.Live() != 0 {
				t.Fatalf("scope leak %d", scope.Live())
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatal(usage)
			}
		})
	}
}

func TestSpillSortStopCancellationEmptyAndErrors(t *testing.T) {
	for _, rows := range []int{0, 1, 4097} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
		vector := store.MakeVector(a, rows, dtype.Int64T(), false)
		for row := range rows {
			vector.I64s()[row] = int64(rows - row)
		}
		batch := store.MakeBatch([]store.Vector{vector})
		table := store.MakeTable([]*store.Batch{batch})
		batch.Release()
		p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"n"}, []dtype.Type{dtype.Int64T()})).OrderBy(MakeOrderKey(MakeColumn("n"))))
		if err != nil {
			t.Fatal(err)
		}
		for mode := range 4 {
			directory := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			called := 0
			if mode == 1 {
				cancel()
			}
			result := p.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 64 << 10, Workers: 1, SpillDirectory: directory}, func(b *store.Batch) bool {
				called++
				if mode == 2 {
					cancel()
					return true
				}
				return false
			})
			cancel()
			expected := ExecutionStopped
			if rows == 0 {
				expected = ExecutionCompleted
			}
			if mode == 1 || mode == 2 && rows != 0 {
				expected = ExecutionCancelled
			}
			if result.Code() != expected {
				t.Fatalf("rows=%d mode=%d got=%v err=%v calls=%d", rows, mode, result.Code(), result.Err(), called)
			}
			files, _ := os.ReadDir(directory)
			if len(files) != 0 {
				t.Fatal("files leaked")
			}
		}
		p.Release()
		table.Release()
		if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatal(usage)
		}
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	vector := store.MakeVector(a, 2, dtype.Int64T(), false)
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	p, _ := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"n"}, []dtype.Type{dtype.Int64T()})).OrderBy(MakeOrderKey(MakeColumn("n"))))
	for _, options := range []ExecutionOptions{{MemoryBudget: 32, SpillDirectory: t.TempDir()}, {MemoryBudget: 1 << 20, SpillDirectory: filepath.Join(t.TempDir(), "missing")}} {
		result := p.ExecuteWithOptions(context.Background(), a, options, func(*store.Batch) bool { return true })
		if result.Code() != ExecutionResourceExhausted && result.Code() != ExecutionFailed {
			t.Fatal(result.Code())
		}
	}
	p.Release()
	table.Release()
}

func TestSpillRecordMalformedAndTypeRoundTrip(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	scope := mem.MakeAllocationScope(a, 64<<10)
	allocator := mem.MakeAllocatorWithScope(a, scope)
	schema := MakeSchema([]string{"i32", "i64", "f32", "f64", "date", "time", "bool", "text"}, []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT(), dtype.BoolT(), dtype.StringT()})
	for _, bad := range []uint64{0, 8, 1 << 40} {
		path := filepath.Join(t.TempDir(), "bad")
		var header [8]byte
		binary.LittleEndian.PutUint64(header[:], bad)
		if err := os.WriteFile(path, header[:], 0600); err != nil {
			t.Fatal(err)
		}
		file, _ := os.Open(path)
		reader := makeSpillStream(context.Background(), allocator, file, scope.Limit())
		batch, err := reader.readRow(allocator, schema, scope.Limit())
		if err == nil || batch != nil {
			t.Fatal("accepted invalid frame")
		}
		reader.release()
	}
	for _, body := range []int{0, 3, -1} {
		path := filepath.Join(t.TempDir(), "truncated")
		data := make([]byte, 8+max(0, body))
		binary.LittleEndian.PutUint64(data, uint64(schema.Len()*9))
		if body == -1 {
			data = data[:5]
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		file, _ := os.Open(path)
		reader := makeSpillStream(context.Background(), allocator, file, scope.Limit())
		batch, err := reader.readRow(allocator, schema, scope.Limit())
		if err != io.ErrUnexpectedEOF || batch != nil {
			t.Fatalf("truncated body=%d: %v", body, err)
		}
		reader.release()
	}
	types := make([]dtype.Type, schema.Len())
	for i := range types {
		types[i] = schema.FieldAt(i).Type()
	}
	vectors := make([]store.Vector, len(types))
	for i, typ := range types {
		if typ.ID() == dtype.STRT {
			vectors[i] = store.MakeStringVector(a, [][]byte{[]byte("binary\x00long string payload"), nil}, []bool{true, false})
		} else {
			vectors[i] = store.MakeVector(a, 2, typ, true)
			vectors[i].Validity().Clear(1)
			if typ.ID() == dtype.BOOLT {
				vectors[i].Bools()[0] = 1
			} else if typ.SlotBits() == 32 {
				binary.NativeEndian.PutUint32(vectors[i].Data().AsBytes(), 0xff800001)
			} else {
				binary.NativeEndian.PutUint64(vectors[i].Data().AsBytes(), 0xfff0000000000001)
			}
		}
	}
	input := store.MakeBatch(vectors)
	file, _ := os.CreateTemp(t.TempDir(), "records")
	path := file.Name()
	writer := makeSpillStream(context.Background(), allocator, file, scope.Limit())
	for row := range 2 {
		if err := writer.writeRow(input, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.flush(); err != nil {
		t.Fatal(err)
	}
	writer.release()
	file, _ = os.Open(path)
	reader := makeSpillStream(context.Background(), allocator, file, scope.Limit())
	for row := range 2 {
		output, err := reader.readRow(allocator, schema, scope.Limit())
		if err != nil {
			t.Fatal(err)
		}
		for column := range types {
			x, y := scalarAt(input.VectorAt(column), row), scalarAt(output.VectorAt(column), 0)
			if x.IsNull() != y.IsNull() || !x.IsNull() && (x.bits != y.bits || x.text != y.text) {
				t.Fatalf("value changed row=%d col=%d", row, column)
			}
		}
		output.Release()
	}
	input.Release()
	reader.release()
	if scope.Live() != 0 {
		t.Fatal("scope leak")
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal(usage)
	}
}
