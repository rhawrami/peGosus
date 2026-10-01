package peg

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestParquetTopNAllocatorReuse(t *testing.T) {
	fixture, err := os.ReadFile("testdata/duckdb-people.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "reuse.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	engine := MakeEngine(EngineOptions{Workers: 1})
	engine.allocator = mem.MakeAllocatorWithProfiles([]int{256}, []int{2048})
	query := engine.ScanParquet(path, ParquetOptions{BatchSize: 2}).
		Select(C("age").Add(int64(0)).Alias("key"), C("income").Add(int64(0)).Alias("value")).
		OrderBy(C("key").Asc(), C("value").Desc()).Limit(3)
	prepared, err := engine.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Release()
	before := engine.MemoryUsage()
	retained, err := prepared.Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer retained.Release()
	batch, ok := retained.BatchAt(0)
	if !ok || retained.NumRows() != 3 {
		t.Fatal("missing TopN output")
	}
	column, err := batch.Column("key")
	if err != nil {
		t.Fatal(err)
	}
	borrowed := column.Int64s()
	reference := slices.Clone(borrowed)
	var warmed mem.AllocatorUsage
	for iteration := range 24 {
		result, err := prepared.Exec()
		if err != nil {
			t.Fatal(err)
		}
		result.Release()
		_, err = prepared.RunContext(context.Background(), func(current Batch) bool {
			key, err := current.Column("key")
			if err != nil || !slices.Equal(key.Int64s(), reference) {
				t.Errorf("repeated TopN output changed: %v", err)
			}
			return true
		})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(borrowed, reference) {
			t.Fatal("allocation reuse invalidated retained TopN output")
		}
		usage := engine.MemoryUsage()
		if iteration == 3 {
			warmed = usage
		} else if iteration > 3 && (usage.GeneralCapacity != warmed.GeneralCapacity || usage.ScratchCapacity != warmed.ScratchCapacity || usage.GeneralUsed != warmed.GeneralUsed || usage.ScratchUsed != warmed.ScratchUsed) {
			t.Fatalf("warm TopN grew storage or leaked segments: iteration=%d warm=%+v actual=%+v", iteration, warmed, usage)
		}
	}
	retained.Release()
	if usage := engine.MemoryUsage(); usage.GeneralUsed != before.GeneralUsed || usage.ScratchUsed != before.ScratchUsed {
		t.Fatalf("retained TopN release leaked segments: %+v", usage)
	}
}
