package parquet

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

const externalParquetCorpus = "../../../testdata/parquet-testing"

func TestExternalParquetCorpus(t *testing.T) {
	if _, err := os.Stat(externalParquetCorpus); errors.Is(err, os.ErrNotExist) {
		t.Skip("prepare the ignored corpus with scripts/prepare_io_testdata.py")
	} else if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	open := func(t *testing.T, name string) *ParquetReader {
		t.Helper()
		f, err := os.Open(filepath.Join(externalParquetCorpus, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		info, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		r, failure := MakeParquetReader(f, info.Size(), a, ParquetOptions{BatchSize: 257})
		if failure != nil {
			t.Fatal(failure)
		}
		t.Cleanup(r.Close)
		return r
	}
	t.Run("snappy-crc-two-pages", func(t *testing.T) {
		r := open(t, "datapage_v1-snappy-compressed-checksum.parquet")
		var rows int
		var sumA, sumB int64
		for {
			batch, failure := r.Next(context.Background())
			if failure != nil {
				t.Fatal(failure)
			}
			if batch == nil {
				break
			}
			for i := range batch.Len() {
				rows++
				sumA += int64(batch.VectorAt(0).I32s()[i])
				sumB += int64(batch.VectorAt(1).I32s()[i])
			}
			batch.Release()
		}
		if rows != 5120 || sumA != 43118090240 || sumB != 129016125440 {
			t.Fatalf("decoded rows %d, sums %d/%d", rows, sumA, sumB)
		}
	})
	t.Run("v2-all-null-empty-snappy-section", func(t *testing.T) {
		r := open(t, "datapage_v2_empty_datapage.snappy.parquet")
		batch, failure := r.Next(context.Background())
		if failure != nil {
			t.Fatal(failure)
		}
		if batch == nil {
			t.Fatal("missing all-null row")
		}
		if batch.Len() != 1 || batch.VectorAt(0).NiN() != 1 {
			t.Fatal("wrong validity for all-null row")
		}
		batch.Release()
	})
	t.Run("corrupt-crc", func(t *testing.T) {
		r := open(t, "datapage_v1-corrupt-checksum.parquet")
		batch, failure := r.Next(context.Background())
		if batch != nil {
			batch.Release()
			t.Fatal("accepted corrupt page")
		}
		if failure == nil || failure.Code() != ParquetInvalid {
			t.Fatalf("CRC failure was not classified as invalid: %v", failure)
		}
	})
	for _, name := range []string{
		"byte_array_decimal.parquet",
		"data_index_bloom_encoding_stats.parquet",
		"datapage_v2.snappy.parquet",
		"rle-dict-snappy-checksum.parquet",
	} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(filepath.Join(externalParquetCorpus, name))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			info, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			r, failure := MakeParquetReader(f, info.Size(), a, ParquetOptions{})
			if failure == nil {
				defer r.Close()
				batch, readErr := r.Next(context.Background())
				if batch != nil {
					batch.Release()
					t.Fatal("accepted unsupported file")
				}
				failure = readErr
			}
			if failure == nil || failure.Code() != ParquetUnsupported {
				t.Fatalf("unsupported feature was not classified: %v", failure)
			}
		})
	}
}
