package plan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestSpillScanBatchesOwnRetainedRecords(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	schema := MakeSchemaWithNullability([]string{"s", "i", "f"}, []dtype.Type{dtype.StringT(), dtype.Int32T(), dtype.Float64T()}, []bool{true, true, true})
	const rows = 301
	text := make([][]byte, rows)
	valid := make([]bool, rows)
	ints := store.MakeVector(a, rows, dtype.Int32T(), true)
	floats := store.MakeVector(a, rows, dtype.Float64T(), true)
	for row := range rows {
		text[row] = []byte(fmt.Sprintf("binary\x00%04d:%s", row, strings.Repeat("x", row%31)))
		if row == 151 {
			text[row] = []byte(strings.Repeat("large\x00", 400))
		}
		valid[row] = row%17 != 0
		ints.I32s()[row] = int32(row - 151)
		floats.F64s()[row] = math.Float64frombits([]uint64{0x7ff8000000000042, 0x8000000000000000, 0x7ff0000000000000, 0xfff0000000000000, 0x3ff0000000000000}[row%5])
		if !valid[row] {
			ints.Validity().Clear(row)
			floats.Validity().Clear(row)
		}
	}
	input := store.MakeBatch([]store.Vector{store.MakeStringVector(a, text, valid), ints, floats})
	paths := make([]string, 3)
	for i := range paths {
		paths[i] = filepath.Join(t.TempDir(), "records")
		file, err := os.Create(paths[i])
		if err != nil {
			t.Fatal(err)
		}
		writer := makeSpillStream(context.Background(), a, file, 128<<10)
		start, end := 0, 0
		if i == 0 {
			end = 131
		} else if i == 2 {
			start, end = 131, rows
		}
		for row := start; row < end; row++ {
			if err := writer.writeRow(input, row); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.flush(); err != nil {
			t.Fatal(err)
		}
		writer.release()
	}
	input.Release()
	scope := mem.MakeAllocationScope(a, 128<<10)
	view := mem.MakeAllocatorWithScope(a, scope)
	cursor := scanCursor{source: &scanSource{schema: schema, spillPaths: paths, spillBudget: scope.Limit()}}
	var retained []*store.Batch
	for {
		batch, done, err := cursor.nextSpill(context.Background(), view)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		if batch.Len() > spillBatchRows || batch.Len() == 0 {
			t.Fatalf("batch length %d", batch.Len())
		}
		retained = append(retained, batch.Retain())
		batch.Release()
	}
	cursor.close()
	if len(retained) >= rows/4 {
		t.Fatalf("scan did not batch records: %d batches", len(retained))
	}
	overwrite := a.AllocSegTemp(16 << 10)
	for i := range overwrite.AsBytes() {
		overwrite.AsBytes()[i] = 0xcc
	}
	overwrite.Dec()
	row := 0
	for _, batch := range retained {
		for at := range batch.Len() {
			for col := range batch.NVectors() {
				if batch.VectorAt(col).Validity().IsSet(at) != valid[row] {
					t.Fatalf("row %d column %d validity changed", row, col)
				}
			}
			if valid[row] && (batch.VectorAt(0).StringAt(at).View() != string(text[row]) || batch.VectorAt(1).I32s()[at] != int32(row-151) || math.Float64bits(batch.VectorAt(2).F64s()[at]) != []uint64{0x7ff8000000000042, 0x8000000000000000, 0x7ff0000000000000, 0xfff0000000000000, 0x3ff0000000000000}[row%5]) {
				t.Fatalf("row %d retained payload changed", row)
			}
			row++
		}
		batch.Release()
	}
	if row != rows || scope.Live() != 0 || scope.Peak() > scope.Limit() {
		t.Fatalf("rows=%d live=%d peak=%d", row, scope.Live(), scope.Peak())
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestSpillScanEmptySchemaAndTruncatedRecord(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
		path := filepath.Join(t.TempDir(), "records")
		data := make([]byte, 8*19)
		if truncated {
			data = append(data, 0)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		scope := mem.MakeAllocationScope(a, 8192)
		view := mem.MakeAllocatorWithScope(a, scope)
		cursor := scanCursor{source: &scanSource{schema: MakeSchema(nil, nil), spillPaths: []string{path}, spillBudget: scope.Limit()}}
		batch, done, err := cursor.nextSpill(context.Background(), view)
		if truncated {
			if err != io.ErrUnexpectedEOF || batch != nil || done {
				t.Fatalf("truncation batch=%v done=%t error=%v", batch, done, err)
			}
		} else {
			if err != nil || done || batch == nil || batch.Len() != 19 || batch.NVectors() != 0 {
				t.Fatalf("empty schema batch=%v done=%t error=%v", batch, done, err)
			}
			batch.Release()
		}
		cursor.close()
		if scope.Live() != 0 {
			t.Fatalf("live=%d", scope.Live())
		}
	}
}

func TestSpillScanRetainedOutputBudgetFailure(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	schema := MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})
	input := store.MakeBatch([]store.Vector{store.MakeVector(a, 2048, dtype.Int64T(), false)})
	path := filepath.Join(t.TempDir(), "records")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := makeSpillStream(context.Background(), a, file, 8192)
	for row := range input.Len() {
		if err := writer.writeRow(input, row); err != nil {
			t.Fatal(err)
		}
	}
	input.Release()
	if err := writer.flush(); err != nil {
		t.Fatal(err)
	}
	writer.release()
	p := PhysicalPlan{source: &scanSource{schema: schema, spillPaths: []string{path}, spillBudget: 8192}, schema: schema}
	scope := mem.MakeAllocationScope(a, 8192)
	var retained []*store.Batch
	result := p.executeWithScope(context.Background(), a, ExecutionOptions{Workers: 1, MemoryBudget: scope.Limit()}, func(batch *store.Batch) bool {
		retained = append(retained, batch.Retain())
		return true
	}, scope)
	if result.Code() != ExecutionResourceExhausted || !errors.Is(result.Err(), errSpillBudget) || !scope.Exhausted() || len(retained) == 0 {
		t.Fatalf("code=%v cause=%v exhausted=%t retained=%d", result.Code(), result.Err(), scope.Exhausted(), len(retained))
	}
	for _, batch := range retained {
		batch.Release()
	}
	if scope.Live() != 0 || scope.Peak() > scope.Limit() {
		t.Fatalf("live=%d peak=%d", scope.Live(), scope.Peak())
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestSpillScanOversizedRecordAndAggregate(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	text := strings.Repeat("wide\x00", 6000)
	schema := MakeSchema([]string{"s"}, []dtype.Type{dtype.StringT()})
	input := store.MakeBatch([]store.Vector{store.MakeStringVector(a, [][]byte{[]byte(text), []byte("z")}, nil)})
	path := filepath.Join(t.TempDir(), "records")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := makeSpillStream(context.Background(), a, file, 128<<10)
	for row := range input.Len() {
		if err := writer.writeRow(input, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.flush(); err != nil {
		t.Fatal(err)
	}
	writer.release()
	scope := mem.MakeAllocationScope(a, 128<<10)
	view := mem.MakeAllocatorWithScope(a, scope)
	cursor := scanCursor{source: &scanSource{schema: schema, spillPaths: []string{path}, spillBudget: scope.Limit()}}
	var retained []*store.Batch
	for {
		batch, done, err := cursor.nextSpill(context.Background(), view)
		if err != nil {
			cursor.close()
			t.Fatal("record fits owned admission but not scan batching", err)
		}
		if done {
			break
		}
		retained = append(retained, batch.Retain())
		batch.Release()
	}
	cursor.close()
	if len(retained) != 2 || retained[0].Len() != 1 || retained[0].VectorAt(0).StringAt(0).View() != text || retained[1].VectorAt(0).StringAt(0).View() != "z" {
		t.Fatal("oversized record ownership changed")
	}
	for _, batch := range retained {
		batch.Release()
	}
	if scope.Live() != 0 || scope.Exhausted() || scope.Peak() > scope.Limit() {
		t.Fatal("record budget accounting", scope.Live(), scope.Peak(), scope.Exhausted())
	}
	table := store.MakeTable([]*store.Batch{input})
	input.Release()
	p, failure := MakePhysicalPlan(MakeScan(table, schema).Aggregate(MakeMin(MakeColumn("s"))))
	if failure != nil {
		t.Fatal(failure)
	}
	scope = mem.MakeAllocationScope(a, 128<<10)
	var result *store.Batch
	status := p.executeWithScope(context.Background(), a, ExecutionOptions{MemoryBudget: scope.Limit(), Workers: 1, SpillDirectory: t.TempDir()}, func(batch *store.Batch) bool {
		result = batch.Retain()
		return true
	}, scope)
	p.Release()
	table.Release()
	if status.Code() != ExecutionCompleted || result == nil || result.Len() != 1 || result.VectorAt(0).StringAt(0).View() != text {
		t.Fatalf("large MIN code=%v cause=%v result=%v", status.Code(), status.Err(), result)
	}
	result.Release()
	if scope.Live() != 0 || scope.Exhausted() || scope.Peak() > scope.Limit() {
		t.Fatal("aggregate budget accounting", scope.Live(), scope.Peak(), scope.Exhausted())
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}
