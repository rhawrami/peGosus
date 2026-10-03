package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func BenchmarkParquetSelectedMaterialization(b *testing.B) {
	encoded, err := os.ReadFile("testdata/duckdb-predicates.b64")
	if err != nil {
		b.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		b.Fatal(err)
	}
	for _, selected := range []bool{false, true} {
		name := "dense"
		if selected {
			name = "selected"
		}
		b.Run(name, func(b *testing.B) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			copied, rows := 0, 0
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, []int{3, 1, 0}, ParquetOptions{BatchSize: 317})
				if failure != nil {
					b.Fatal(failure)
				}
				r.SetSelectedMaterialization(selected)
				r.SetScanPredicates([]ScanPredicate{{Column: 1, Op: PruneEqual, Integer: -7}})
				for {
					batch, failure := r.Next(context.Background())
					if failure != nil {
						b.Fatal(failure)
					}
					if batch == nil {
						break
					}
					if backing := batch.VectorAt(0).Backing(); backing != nil {
						copied += backing.Len()
					}
					rows += batch.ActiveLen()
					batch.Release()
				}
				r.Close()
			}
			b.StopTimer()
			if rows != b.N*38 {
				b.Fatalf("selected rows=%d want %d", rows, b.N*38)
			}
			if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
				b.Fatalf("leak %+v", u)
			}
			b.ReportMetric(float64(copied)/float64(b.N), "copied-string-bytes/op")
			b.ReportMetric(float64(rows)/float64(b.N), "selected-rows/op")
		})
	}
}
