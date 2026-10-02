package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"os"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

type parquetReadCounter struct {
	io.ReaderAt
	calls, bytes int64
}

func (r *parquetReadCounter) ReadAt(p []byte, offset int64) (int, error) {
	r.calls++
	n, err := r.ReaderAt.ReadAt(p, offset)
	r.bytes += int64(n)
	return n, err
}

func BenchmarkParquetReadPattern(b *testing.B) {
	data, err := base64.StdEncoding.DecodeString(duckDBGroupsFixture)
	if err != nil {
		b.Fatal(err)
	}
	path := os.Getenv("PEG_BENCH_PARQUET")
	if path != "" {
		data, err = os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
	} else {
		f, err := os.CreateTemp(b.TempDir(), "pages-*.parquet")
		if err != nil {
			b.Fatal(err)
		}
		path = f.Name()
		_, err = f.Write(data)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			b.Fatalf("fixture write: %v, %v", err, closeErr)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	for _, mode := range []string{"file", "memory"} {
		for _, projection := range []string{"first", "all"} {
			b.Run(mode+"/"+projection, func(b *testing.B) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				var input io.ReaderAt = f
				if mode == "memory" {
					input = bytes.NewReader(data)
				}
				counter := &parquetReadCounter{ReaderAt: input}
				var selected []int
				if projection == "first" {
					selected = []int{0}
				}
				rows := 0
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					r, failure := MakeParquetReaderProjected(counter, int64(len(data)), a, selected, ParquetOptions{})
					if failure != nil {
						b.Fatal(failure)
					}
					count := 0
					for {
						batch, failure := r.Next(context.Background())
						if failure != nil {
							b.Fatal(failure)
						}
						if batch == nil {
							break
						}
						count += batch.Len()
						batch.Release()
					}
					r.Close()
					if count == 0 || rows != 0 && count != rows {
						b.Fatalf("inconsistent row count: %d, %d", rows, count)
					}
					rows = count
				}
				b.ReportMetric(float64(counter.calls)/float64(b.N), "reads/scan")
				b.ReportMetric(float64(counter.bytes)/float64(b.N), "read-B/scan")
				b.ReportMetric(float64(rows)*float64(b.N)/b.Elapsed().Seconds(), "rows/s")
			})
		}
	}
}
