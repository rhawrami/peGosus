package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/peg"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestRetainedTableSource(t *testing.T) {
	encoded, err := os.ReadFile("../../pkg/io/parquet/testdata/duckdb-predicates.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	engine := peg.MakeEngine(peg.EngineOptions{Workers: 1})
	query, table, err := makeSource(engine, path, "", "table", []field{{Name: "id"}, {Name: "value"}, {Name: "active"}, {Name: "category"}})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Release()
	prepared, err := engine.Prepare(query.Agg(peg.CountStar().Alias("rows")))
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Release()
	result, err := prepared.Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Release()
	batch, _ := result.BatchAt(0)
	column, _ := batch.Column("rows")
	if column.Int64s()[0] != 4097 {
		t.Fatalf("retained input rows: %d", column.Int64s()[0])
	}
}

func makeSignatureResult(t *testing.T, allocator *mem.Allocator, reverse bool) (*peg.Result, *peg.Prepared, *store.Table) {
	t.Helper()
	ids := store.MakeVector(allocator, 2, dtype.Int32T(), false)
	score := store.MakeVector(allocator, 2, dtype.Float64T(), true)
	values := [][]byte{[]byte("Café 東京 retained category value"), []byte("second long category value")}
	copy(ids.I32s(), []int32{1, 2})
	copy(score.F64s(), []float64{0, 1.25})
	if reverse {
		ids.I32s()[0], ids.I32s()[1] = ids.I32s()[1], ids.I32s()[0]
		score.F64s()[0], score.F64s()[1] = score.F64s()[1], score.F64s()[0]
		values[0], values[1] = values[1], values[0]
	}
	category := store.MakeStringVector(allocator, values, nil)
	batch := store.MakeBatch([]store.Vector{ids, category, score})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	engine := peg.MakeEngine(peg.EngineOptions{Workers: 1})
	prepared, err := engine.Prepare(engine.ScanTable(table, []string{"id", "category", "score"}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepared.Exec()
	if err != nil {
		t.Fatal(err)
	}
	return result, prepared, table
}

func TestSignatureOrderingAndSampleOwnership(t *testing.T) {
	allocator := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	forward, fp, ft := makeSignatureResult(t, allocator, false)
	reverse, rp, rt := makeSignatureResult(t, allocator, true)
	if resultSignature(forward, "ordered").SHA256 == resultSignature(reverse, "ordered").SHA256 {
		t.Fatal("ordered fingerprint ignored row order")
	}
	if resultSignature(forward, "wide_materialize").SHA256 != resultSignature(reverse, "wide_materialize").SHA256 {
		t.Fatal("multiset fingerprint depends on row order")
	}
	preview := resultSignature(forward, "preview_limit")
	forward.Release()
	reverse.Release()
	fp.Release()
	rp.Release()
	ft.Release()
	rt.Release()
	if usage := allocator.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("signature pass retained source payload: %+v", usage)
	}
	for range 8 {
		churn := store.MakeStringVector(allocator, [][]byte{[]byte(strings.Repeat("x", 64))}, nil)
		churn.Release()
	}
	if preview.Sample[0][1] != "Café 東京 retained category value" {
		t.Fatalf("saved sample changed after source release: %+v", preview.Sample)
	}
}
