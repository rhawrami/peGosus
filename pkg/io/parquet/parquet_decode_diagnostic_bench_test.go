package parquet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParquetColumnDiagnostic(b *testing.B) {
	path := os.Getenv("PEG_BENCH_PARQUET")
	if path == "" {
		b.Skip("set PEG_BENCH_PARQUET to the paired benchmark fixture")
	}
	f, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	r, failure := MakeParquetReader(f, int64(len(data)), a, ParquetOptions{})
	if failure != nil {
		b.Fatal(failure)
	}
	columns := make(map[string]int)
	for i := range r.ColumnCount() {
		name, _, _ := r.Column(i)
		columns[name] = i
	}
	want := 0
	for group := range r.RowGroupCount() {
		want += int(r.RowGroupRows(group))
	}
	r.Close()
	for _, name := range []string{"footer-only", "id", "active", "day", "score", "category", "customer_code", "message", "flag_text", "fiscal_year", "price", "quantity", "wide"} {
		column, exists := columns[name]
		projection := []int{column}
		if name == "wide" {
			projection = nil
			exists = true
			for _, field := range []string{"id", "category", "customer_code", "message", "flag_text", "fiscal_year", "price", "quantity", "score"} {
				index, present := columns[field]
				exists = exists && present
				projection = append(projection, index)
			}
		}
		if !exists && name != "footer-only" {
			continue
		}
		for _, mode := range []string{"file", "memory"} {
			b.Run(name+"/"+mode, func(b *testing.B) {
				counter := &parquetReadCounter{ReaderAt: f}
				if mode == "memory" {
					counter.ReaderAt = bytes.NewReader(data)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					r, failure := MakeParquetReaderProjected(counter, int64(len(data)), a, projection, ParquetOptions{})
					if failure != nil {
						b.Fatal(failure)
					}
					rows := 0
					for name != "footer-only" {
						batch, failure := r.Next(context.Background())
						if failure != nil {
							b.Fatal(failure)
						}
						if batch == nil {
							break
						}
						rows += batch.Len()
						batch.Release()
					}
					r.Close()
					if name != "footer-only" && rows != want {
						b.Fatalf("decoded %d rows, want %d", rows, want)
					}
				}
				b.ReportMetric(float64(counter.calls)/float64(b.N), "reads/scan")
				b.ReportMetric(float64(counter.bytes)/float64(b.N), "read-B/scan")
			})
		}
	}
}

func BenchmarkParquetPackedIndexDiagnostic(b *testing.B) {
	const rows = 4096
	for _, width := range []byte{1, 3, 9, 14, 32} {
		b.Run(fmt.Sprint(width), func(b *testing.B) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
			out := a.AllocSegTemp(rows * 4)
			defer out.Dec()
			payload := make([]byte, rows*int(width)/8)
			mask := uint32(uint64(1)<<width - 1)
			rng := rand.New(rand.NewPCG(47, uint64(width)))
			want := make([]uint32, rows)
			for row := range want {
				want[row] = rng.Uint32() & mask
				for bit := range int(width) {
					at := row*int(width) + bit
					payload[at/8] |= byte(want[row]>>bit&1) << (at & 7)
				}
			}
			encoded := binary.AppendUvarint(nil, (rows/8)<<1|1)
			encoded = append(encoded, payload...)
			for _, mode := range []string{"current", "word-prototype"} {
				b.Run(mode, func(b *testing.B) {
					b.SetBytes(rows * 4)
					b.ReportAllocs()
					for range b.N {
						if mode == "current" {
							if err := decodeParquetIndices(encoded, width, out.AsBytes()); err != nil {
								b.Fatal(err)
							}
						} else {
							// This prototype handles one validated packed run, not the full hybrid format.
							dst := out.AsBytes()
							for row := range rows {
								at := row * int(width)
								src := payload[at/8:]
								var word uint64
								if len(src) >= 8 {
									word = binary.LittleEndian.Uint64(src)
								} else {
									for i, v := range src {
										word |= uint64(v) << (8 * i)
									}
								}
								binary.LittleEndian.PutUint32(dst[4*row:], uint32(word>>(at&7))&mask)
							}
						}
					}
					b.StopTimer()
					for row, expected := range want {
						if got := binary.LittleEndian.Uint32(out.AsBytes()[4*row:]); got != expected {
							b.Fatalf("row %d: %d, want %d", row, got, expected)
						}
					}
				})
			}
		})
	}
}

func BenchmarkParquetFixedMaterializationDiagnostic(b *testing.B) {
	const rows = 4096
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	src := a.AllocSegTemp(rows * 4)
	defer src.Dec()
	for row := range rows {
		binary.LittleEndian.PutUint32(src.AsBytes()[4*row:], uint32(row*17))
	}
	levels := a.AllocSegTemp(rows)
	defer levels.Dec()
	for i := range levels.AsBytes() {
		levels.AsBytes()[i] = 1
	}
	indices := a.AllocSegTemp(rows * 4)
	defer indices.Dec()
	for row := range rows {
		indices.AsI32T()[row] = int32(row % 367)
	}
	for _, shape := range []string{"plain", "nullable-plain", "nullable-dictionary"} {
		for _, mode := range []string{"current", "typed-prototype"} {
			b.Run(shape+"/"+mode, func(b *testing.B) {
				vector := store.MakeVector(a, rows, dtype.Int32T(), shape != "plain")
				defer vector.Release()
				c := parquetCursor{column: parquetColumn{typ: dtype.Int32T(), optional: shape != "plain"}, pageRows: rows, values: src.AsBytes(), levels: levels}
				if shape == "nullable-dictionary" {
					c.encoding, c.dictCount, c.indices, c.dictRaw = 8, 367, indices, src.AsBytes()[:367*4]
				}
				ctx := context.Background()
				b.ReportAllocs()
				b.SetBytes(rows * 4)
				b.ResetTimer()
				for range b.N {
					c.pagePos, c.rows, c.valuePos, c.nonNull = 0, 0, 0, rows
					if mode == "current" {
						for row := range rows {
							v, valid, err := c.next(ctx)
							if err != nil || !valid {
								b.Fatal(err)
							}
							vector.I32s()[row] = int32(binary.LittleEndian.Uint32(v))
						}
					} else {
						// All values are valid; production bulk decoding must also preserve null/page semantics.
						dst := vector.I32s()
						if c.encoding == 0 {
							copy(vector.Data().AsBytes(), src.AsBytes())
						} else {
							ids := indices.AsI32T()
							dict := src.AsI32T()
							for row, id := range ids {
								dst[row] = dict[id]
							}
						}
					}
				}
				b.StopTimer()
				for row, got := range vector.I32s() {
					id := row
					if c.encoding != 0 {
						id %= 367
					}
					if got != int32(id*17) {
						b.Fatalf("row %d: %d", row, got)
					}
				}
			})
		}
	}
}
