package peg_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rhawrami/peGosus/pkg/peg"
)

func BenchmarkParquetPublicAPI(b *testing.B) {
	path := filepath.Join("..", "..", "testdata", "paired", "scan_snappy.parquet")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		b.Skip("prepare ignored data with scripts/prepare_io_testdata.py")
	} else if err != nil {
		b.Fatal(err)
	}
	engine := peg.MakeEngine(peg.EngineOptions{MemoryBudget: 64 << 20})
	b.Run("prepared", func(b *testing.B) {
		query := engine.ScanParquet(path).Filter(peg.C("id").Ge(16000), peg.C("active")).Agg(peg.CountStar().Alias("rows"))
		prepared, err := engine.Prepare(query)
		if err != nil {
			b.Fatal(err)
		}
		defer prepared.Release()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			result, err := prepared.Exec()
			if err != nil {
				b.Fatal(err)
			}
			batch, _ := result.BatchAt(0)
			count, err := batch.Column("rows")
			if err != nil || count.Int64s()[0] != 129 {
				b.Fatalf("incorrect count: %v", err)
			}
			result.Release()
		}
	})
	b.Run("construct-and-execute", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			result, err := engine.ScanParquet(path).Filter(peg.C("id").Ge(16000), peg.C("active")).Agg(peg.CountStar().Alias("rows")).Exec()
			if err != nil {
				b.Fatal(err)
			}
			batch, _ := result.BatchAt(0)
			count, err := batch.Column("rows")
			if err != nil || count.Int64s()[0] != 129 {
				b.Fatalf("incorrect count: %v", err)
			}
			result.Release()
		}
	})
}
