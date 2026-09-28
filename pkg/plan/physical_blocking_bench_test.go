package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeBlockingBenchmarkTable(a *mem.Allocator, rows, cardinality, selectionFraction int, strings bool) (*store.Table, dtype.Type) {
	const batchSize = 1024
	rng := rand.New(rand.NewPCG(1307, 11))
	typeValue := dtype.Int32T()
	if strings {
		typeValue = dtype.StringT()
	}
	batches := make([]*store.Batch, 0, (rows+batchSize-1)/batchSize)
	for start := 0; start < rows; start += batchSize {
		length := min(batchSize, rows-start)
		var vector store.Vector
		var texts [][]byte
		var valid []bool
		if strings {
			texts = make([][]byte, length)
			valid = make([]bool, length)
		} else {
			vector = store.MakeVector(a, length, typeValue, true)
		}
		mask := store.MakeBitMap(a, length)
		for row := range length {
			value := rng.IntN(cardinality)
			isValid := rng.IntN(32) != 0
			if strings {
				texts[row] = strconv.AppendInt([]byte("benchmark-key-"), int64(value), 10)
				valid[row] = isValid
			} else {
				vector.I32s()[row] = int32(value)
				if !isValid {
					vector.Validity().Clear(row)
				}
			}
			if (start+row)%selectionFraction == 0 {
				mask.Set(row)
			}
		}
		if strings {
			vector = store.MakeStringVector(a, texts, valid)
		}
		batch := store.MakeBatch([]store.Vector{vector})
		batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
		batches = append(batches, batch)
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	return table, typeValue
}

func BenchmarkBlockingOperators(b *testing.B) {
	for _, operation := range []string{"group-string", "distinct", "sort", "join"} {
		for _, fraction := range []int{1, 8} {
			b.Run(fmt.Sprintf("%s/selected=1in%d", operation, fraction), func(b *testing.B) {
				b.StopTimer()
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				table, keyType := makeBlockingBenchmarkTable(a, 32768, 4096, fraction, operation == "group-string")
				scan := MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{keyType}))
				var query LogicalPlan
				var other *store.Table
				switch operation {
				case "group-string":
					query = scan.GroupBy([]Expr{MakeColumn("key")}, MakeCountStar())
				case "distinct":
					query = scan.Distinct()
				case "sort":
					query = scan.OrderBy(MakeOrderKey(MakeColumn("key")))
				case "join":
					other, _ = makeBlockingBenchmarkTable(a, 4096, 4096, fraction, false)
					query = scan.As("left").Join(
						MakeScan(other, MakeSchema([]string{"key"}, []dtype.Type{keyType})).As("right"), JoinInner,
						[]Expr{MakeColumn("left.key")}, []Expr{MakeColumn("right.key")},
					)
				}
				plan, err := MakePhysicalPlan(query)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				var outputRows int
				b.StartTimer()
				for range b.N {
					result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: math.MaxInt64}, func(output *store.Batch) bool {
						outputRows += output.ActiveLen()
						return true
					})
					if result.Code() != ExecutionCompleted {
						b.Fatalf("%s failed: %v", operation, result.Code())
					}
				}
				b.StopTimer()
				b.ReportMetric(float64(outputRows)/float64(b.N), "rows/op")
				plan.Release()
				table.Release()
				if other != nil {
					other.Release()
				}
			})
		}
	}
}
