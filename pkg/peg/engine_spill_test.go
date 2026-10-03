package peg_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/peg"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestEngineSpilledStreamingAndRetainedBudget(t *testing.T) {
	const rows = 20000
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	vector := store.MakeVector(a, rows, dtype.Int64T(), false)
	for row := range rows {
		vector.I64s()[row] = int64(rows - row)
	}
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	defer table.Release()
	directory := t.TempDir()
	engine := peg.MakeEngine(peg.EngineOptions{Workers: 4, MemoryBudget: 128 << 10, SpillDirectory: directory})
	query := engine.ScanTable(table, []string{"id"}).OrderBy(peg.C("id").Asc())
	p, err := engine.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	seen := int64(0)
	status, err := p.RunContext(context.Background(), func(batch peg.Batch) bool {
		column, err := batch.Column("id")
		if err != nil {
			t.Fatal(err)
		}
		return batch.ForEachActive(func(row int) bool {
			seen++
			if column.Int64s()[row] != seen {
				t.Fatal("incorrect sorted value")
			}
			return true
		})
	})
	if err != nil || status != peg.RunCompleted || seen != rows {
		t.Fatalf("stream: status=%v rows=%d err=%v", status, seen, err)
	}
	result, err := p.Exec()
	var failure *peg.Error
	if result != nil || !errors.As(err, &failure) || failure.Code() != peg.ErrorResourceExhausted {
		t.Fatalf("retained output exceeded budget without error: %v", err)
	}
	limited, err := query.Limit(7, 3).Exec()
	if err != nil || limited.NumRows() != 7 {
		t.Fatalf("small retained result: %v", err)
	}
	limited.Release()
	ctx, cancel := context.WithCancel(context.Background())
	_, err = p.RunContext(ctx, func(peg.Batch) bool { cancel(); return true })
	if !errors.As(err, &failure) || failure.Code() != peg.ErrorCancelled {
		t.Fatalf("cancellation: %v", err)
	}
	files, err := os.ReadDir(directory)
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files remain: %v %v", files, err)
	}
	usage := engine.MemoryUsage()
	if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal("engine storage leak", usage)
	}
}
