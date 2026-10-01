package parquet

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParquetStringMaterialization(b *testing.B) {
	const n = 4096
	for _, shape := range []string{"inline", "mixed", "long", "null"} {
		values, valid := make([][]byte, n), make([]bool, n)
		var inputBytes int64
		nonNull := 0
		for i := range values {
			length := 12
			if shape == "long" {
				length = 128
			}
			if shape == "mixed" && i%2 == 0 {
				length = 48
			}
			values[i] = bytes.Repeat([]byte("a"), length)
			valid[i] = shape != "null" && i%7 != 0
			if valid[i] {
				inputBytes += int64(length)
				nonNull++
			}
		}
		for _, dictionary := range []bool{false, true} {
			mode := "plain"
			if dictionary {
				mode = "dictionary"
			}
			b.Run(shape+"/"+mode, func(b *testing.B) {
				for _, materializer := range []struct {
					name string
					run  func(context.Context, *parquetCursor, int) (store.Vector, *ParquetError)
				}{{"baseline", makeParquetStringVectorBaseline}, {"direct-descriptors", makeParquetStringVector}} {
					b.Run(materializer.name, func(b *testing.B) {
						a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
						scope := mem.MakeAllocationScope(a, 32<<20)
						c := makeParquetStringTestCursor(mem.MakeAllocatorWithScope(a, scope), values, valid, dictionary)
						sourceBytes := scope.Live()
						var outputBytes int64
						ctx := context.Background()
						b.ReportAllocs()
						b.SetBytes(inputBytes)
						b.ResetTimer()
						for range b.N {
							c.pagePos, c.rows, c.valuePos, c.nonNull = 0, 0, 0, nonNull
							v, failure := materializer.run(ctx, &c, n)
							if failure != nil {
								b.Fatal(failure)
							}
							outputBytes = scope.Live() - sourceBytes
							v.Release()
						}
						b.StopTimer()
						b.ReportMetric(float64(scope.Peak()-sourceBytes), "peak-output-B")
						b.ReportMetric(float64(outputBytes), "retained-output-B")
						b.ReportMetric(float64(n)*float64(b.N)/b.Elapsed().Seconds(), "rows/s")
						c.close()
						if scope.Live() != 0 {
							b.Fatalf("leaked %d bytes", scope.Live())
						}
					})
				}
			})
		}
	}
}

func makeParquetStringVectorBaseline(ctx context.Context, c *parquetCursor, n int) (store.Vector, *ParquetError) {
	values := make([][]byte, n)
	lengths := make([]int, n)
	var scratch *mem.Segment
	var copied []byte
	var valid []bool
	if c.column.optional {
		valid = make([]bool, n)
	}
	defer func() {
		if scratch != nil {
			scratch.Dec()
		}
	}()
	for j := range values {
		v, yes, err := c.next(ctx)
		if err != nil {
			return store.Vector{}, err
		}
		if !yes {
			continue
		}
		lengths[j] = len(v)
		if len(v) > cap(copied)-len(copied) {
			if len(v) > math.MaxInt-len(copied) {
				return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("string scratch overflow"))
			}
			size := max(4096, cap(copied)*2, len(copied)+len(v))
			if size < len(copied) || size < len(v) {
				return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("string scratch overflow"))
			}
			next := c.a.AllocSegTemp(size)
			if next == nil {
				return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("string scratch allocation failed"))
			}
			grown := next.AsBytes()[:len(copied)]
			copy(grown, copied)
			if scratch != nil {
				scratch.Dec()
			}
			scratch, copied = next, grown
		}
		copied = append(copied, v...)
		if valid != nil {
			valid[j] = true
		}
	}
	on := 0
	for j := range values {
		if valid != nil && !valid[j] {
			continue
		}
		values[j] = copied[on : on+lengths[j]]
		on += lengths[j]
	}
	v := store.MakeStringVector(c.a, values, valid)
	if v.Kind() == store.VectorInvalid {
		return v, c.failure(ParquetResourceExhausted, errors.New("string allocation failed"))
	}
	return v, nil
}
