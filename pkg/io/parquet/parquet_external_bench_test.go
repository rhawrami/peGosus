package parquet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func BenchmarkExternalParquetPhases(b *testing.B) {
	for _, codec := range []string{"scan_snappy.parquet", "scan_plain.parquet"} {
		path := filepath.Join("../../../testdata/paired", codec)
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			b.Skip("prepare ignored data with scripts/prepare_io_testdata.py")
		}
		if err != nil {
			b.Fatal(err)
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			b.Fatal(err)
		}
		b.Run(codec, func(b *testing.B) {
			defer f.Close()
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			for _, mode := range []string{"footer", "id-only", "all-columns"} {
				b.Run(mode, func(b *testing.B) {
					var selected []int
					if mode == "id-only" {
						selected = []int{0}
					}
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						r, failure := MakeParquetReaderProjected(f, info.Size(), a, selected, ParquetOptions{})
						if failure != nil {
							b.Fatal(failure)
						}
						if mode != "footer" {
							count := 0
							for {
								batch, err := r.Next(context.Background())
								if err != nil {
									b.Fatal(err)
								}
								if batch == nil {
									break
								}
								count += batch.Len()
								batch.Release()
							}
							if count != 16387 {
								b.Fatalf("decoded %d rows", count)
							}
						}
						r.Close()
					}
					if mode != "footer" {
						b.ReportMetric(float64(16387)*float64(b.N)/b.Elapsed().Seconds(), "input-rows/s")
					}
				})
			}
		})
	}
}
