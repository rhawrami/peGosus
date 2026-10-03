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

func BenchmarkRuntimePruningParquet(b *testing.B) {
	encoded, err := os.ReadFile("testdata/duckdb-predicates.b64")
	if err != nil {
		b.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		b.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		b.Run(name, func(b *testing.B) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			cache := MakeParquetMetadataCache()
			input := bytes.NewReader(data)
			var rows, pruned int64
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				reader, failure := MakeParquetReaderProjectedCached(input, int64(len(data)), a, nil, ParquetOptions{BatchSize: 317}, cache)
				if failure != nil {
					b.Fatal(failure)
				}
				filter := MakeRuntimePruningFilter(0, false)
				if enabled && !reader.SetRuntimePruningFilter(filter) {
					b.Fatal("runtime filter installation")
				}
				for {
					batch, failure := reader.Next(context.Background())
					if failure != nil {
						b.Fatal(failure)
					}
					if batch == nil {
						break
					}
					rows += int64(batch.Len())
					batch.Release()
					filter.UpdateUpperBound(12)
				}
				pruned += filter.PrunedRowGroups()
				reader.Close()
			}
			b.StopTimer()
			b.ReportMetric(float64(rows)/float64(b.N), "decoded-rows/op")
			b.ReportMetric(float64(pruned)/float64(b.N), "pruned-groups/op")
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				b.Fatal("reader leaked payload")
			}
		})
	}
}
