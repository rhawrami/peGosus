package plan

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestDictionaryStringsAcrossExpressionsAndBlockingOperators(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	schema := MakeSchemaWithNullability([]string{"text", "condition"}, []dtype.Type{dtype.StringT(), dtype.BoolT()}, []bool{true, true})
	queries := []func(LogicalPlan) LogicalPlan{
		func(scan LogicalPlan) LogicalPlan {
			return scan.Filter(MakeColumn("text").Contains("o").Or(MakeColumn("text").IsNull())).Project(
				MakeCase(MakeColumn("condition"), MakeColumn("text"), "alternate long string backing").Alias("chosen"),
				MakeColumn("text").Coalesce("missing long string backing").Alias("filled"),
				MakeColumn("text").Eq("short").Alias("equal"),
			).Distinct().OrderBy(MakeOrderKey(MakeColumn("chosen")), MakeOrderKey(MakeColumn("filled")))
		},
		func(scan LogicalPlan) LogicalPlan {
			return scan.OrderBy(MakeOrderKey(MakeColumn("text")).Desc().NullsFirst()).Limit(17, 3)
		},
		func(scan LogicalPlan) LogicalPlan {
			return scan.GroupBy([]Expr{MakeColumn("text").Alias("key")}, MakeCountDistinct(MakeColumn("condition")).Alias("count")).OrderBy(MakeOrderKey(MakeColumn("key")))
		},
	}
	for queryIndex, query := range queries {
		var results [2][][]string
		for variant := range 2 {
			var batches []*store.Batch
			for part := range 2 {
				texts := [][]byte{[]byte(""), []byte("short"), []byte("long dictionary string payload"), []byte("ø\x00"), []byte("short")}
				if part == 1 {
					texts[0], texts[2] = texts[2], texts[0]
				}
				indices := a.AllocSeg(65 * 4)
				validity := store.MakeBitMap(a, 65)
				condition := store.MakeVector(a, 65, dtype.BoolT(), true)
				flat, valid := make([][]byte, 65), make([]bool, 65)
				var offsets []uint32
				for row := range 65 {
					index := (row + part) % len(texts)
					indices.AsU32T()[row] = uint32(index)
					flat[row], valid[row] = texts[index], row%11 != 0
					if valid[row] {
						validity.Set(row)
					}
					if row%3 == 0 {
						condition.Bools()[row>>3] |= 1 << (row & 7)
					}
					if row%13 == 0 {
						condition.Validity().Clear(row)
					}
					if row%7 != 0 {
						offsets = append(offsets, uint32(row))
					}
				}
				text := store.MakeDictionaryStringVectorFromOwnedSegments(store.MakeStringVector(a, texts, nil), 65, indices, validity)
				if variant == 0 {
					text.Release()
					text = store.MakeStringVector(a, flat, valid)
				}
				batch := store.MakeBatch([]store.Vector{text, condition})
				batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 65, offsets)))
				batches = append(batches, batch)
			}
			table := store.MakeTable(batches)
			for _, batch := range batches {
				batch.Release()
			}
			p, err := MakePhysicalPlan(query(MakeScan(table, schema)))
			if err != nil {
				t.Fatal(err)
			}
			table.Release()
			var outputs []*store.Batch
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{Workers: 4, MemoryBudget: 8 << 20}, func(batch *store.Batch) bool {
				outputs = append(outputs, batch.Retain())
				return true
			})
			p.Release()
			if result.Code() != ExecutionCompleted {
				t.Fatalf("query %d: %v/%v", queryIndex, result.Code(), result.Err())
			}
			for _, batch := range outputs {
				selection := batch.Selection().RetainBitMap(a)
				for row := range batch.Len() {
					if selection != nil && !selection.IsSet(row) {
						continue
					}
					values := make([]string, batch.NVectors())
					for col := range values {
						value := scalarAt(batch.VectorAt(col), row)
						if value.IsNull() {
							values[col] = "null"
						} else if value.Type().ID() == dtype.STRT {
							values[col] = fmt.Sprintf("string:%q", value.stringValue())
						} else {
							values[col] = fmt.Sprintf("%d:%d", value.Type().ID(), value.bits)
						}
					}
					results[variant] = append(results[variant], values)
				}
				selection.Release()
				batch.Release()
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("query %d variant %d leaked %+v", queryIndex, variant, usage)
			}
		}
		if !reflect.DeepEqual(results[0], results[1]) {
			t.Fatalf("query %d: flat %v, dictionary %v", queryIndex, results[0], results[1])
		}
	}
}
