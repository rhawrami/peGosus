package peg_test

import (
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/peg"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestEngineMemoryUsageRetainedResultAndReuse(t *testing.T) {
	source := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	vector := store.MakeVector(source, 300000, dtype.Int64T(), false)
	for i := range vector.I64s() {
		vector.I64s()[i] = int64(i)
	}
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	defer table.Release()
	batch.Release()
	engine := peg.MakeEngine(peg.EngineOptions{Workers: 1})
	prepared, err := engine.Prepare(engine.ScanTable(table, []string{"id"}).Select(peg.C("id").Add(1).Alias("next")))
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Release()
	before := engine.MemoryUsage()
	result, err := prepared.Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Release()
	retained, ok := result.BatchAt(0)
	if !ok || result.NumRows() != 300000 {
		t.Fatal("missing computed output")
	}
	retained = retained.Retain()
	defer retained.Release()
	held := engine.MemoryUsage()
	if held.GeneralUsed+held.ScratchUsed <= before.GeneralUsed+before.ScratchUsed {
		t.Fatalf("retained output missing from used capacity: before=%+v held=%+v", before, held)
	}
	result.Release()
	stillHeld := engine.MemoryUsage()
	if stillHeld.GeneralUsed+stillHeld.ScratchUsed != held.GeneralUsed+held.ScratchUsed {
		t.Fatalf("result release freed retained storage: held=%+v after=%+v", held, stillHeld)
	}
	column, err := retained.Column("next")
	if err != nil || column.Int64s()[0] != 1 || column.Int64s()[299999] != 300000 {
		t.Fatalf("retained values invalid: %v", err)
	}
	retained.Release()
	released := engine.MemoryUsage()
	if released.GeneralUsed != before.GeneralUsed || released.ScratchUsed != before.ScratchUsed {
		t.Fatalf("release leaked segment capacity: before=%+v after=%+v", before, released)
	}
	for range 3 {
		next, err := prepared.Exec()
		if err != nil {
			t.Fatal(err)
		}
		next.Release()
		usage := engine.MemoryUsage()
		if usage.GeneralCapacity != released.GeneralCapacity || usage.ScratchCapacity != released.ScratchCapacity || usage.GeneralUsed != before.GeneralUsed || usage.ScratchUsed != before.ScratchUsed {
			t.Fatalf("warm execution failed to reuse released slabs: released=%+v after=%+v", released, usage)
		}
	}
}
