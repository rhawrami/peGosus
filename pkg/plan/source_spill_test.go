package plan

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParquetSpillInputPredicatesAndEmptyProjection(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(parquetScanFixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	schema := MakeSchemaWithNullability([]string{"id", "txt"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true})
	scan := MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 2})
	for _, tc := range []struct {
		name       string
		query      LogicalPlan
		want       []string
		projection []int
	}{
		{"ordinary", scan.Project(MakeColumn("txt")).OrderBy(MakeOrderKey(MakeColumn("txt"))), []string{"aaaaaaaaaaaaaaa", "ccccccccccccccc", "NULL"}, []int{1}},
		{"exact-removed-column", scan.Filter(MakeColumn("id").Ge(1)).Project(MakeColumn("txt")).OrderBy(MakeOrderKey(MakeColumn("txt"))), []string{"ccccccccccccccc", "NULL"}, []int{1}},
		{"exact-and-residual", scan.Filter(MakeColumn("id").Ge(1).And(MakeColumn("txt").Coalesce("").Contains("c"))).Project(MakeColumn("txt")).OrderBy(MakeOrderKey(MakeColumn("txt"))), []string{"ccccccccccccccc"}, []int{1}},
		{"empty-projection", scan.Project(MakeLiteral(int64(7)).Alias("n")).OrderBy(MakeOrderKey(MakeColumn("n"))), []string{"7", "7", "7"}, []int{}},
		{"empty-projection-after-predicate", scan.Filter(MakeColumn("id").Ge(1)).Project(MakeLiteral(int64(7)).Alias("n")).OrderBy(MakeOrderKey(MakeColumn("n"))), []string{"7", "7"}, []int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
			p, failure := MakePhysicalPlan(tc.query)
			if failure != nil {
				t.Fatal(failure)
			}
			defer p.Release()
			if p.source.projection == nil || !slices.Equal(p.source.projection, tc.projection) {
				t.Fatalf("projection=%v want=%v", p.source.projection, tc.projection)
			}
			for _, spill := range []bool{false, true} {
				scope := mem.MakeAllocationScope(a, 128<<10)
				options := ExecutionOptions{MemoryBudget: scope.Limit(), Workers: 1}
				directory := t.TempDir()
				if spill {
					options.SpillDirectory = directory
				}
				var got []string
				var retained []*store.Batch
				result := p.executeWithScope(context.Background(), a, options, func(batch *store.Batch) bool {
					if batch.NVectors() != 1 {
						t.Fatal("changed output schema")
					}
					v := batch.VectorAt(0)
					for row := range batch.Len() {
						if v.Validity() != nil && !v.Validity().IsSet(row) {
							got = append(got, "NULL")
						} else if v.Type().ID() == dtype.STRT {
							got = append(got, string([]byte(v.StringAt(row).View())))
						} else if v.I64s()[row] == 7 {
							got = append(got, "7")
						} else {
							t.Fatal("changed literal projection")
						}
					}
					retained = append(retained, batch.Retain())
					return true
				}, scope)
				if result.Code() != ExecutionCompleted || !slices.Equal(got, tc.want) {
					t.Fatalf("spill=%t result=%v error=%v got=%v want=%v", spill, result.Code(), result.Err(), got, tc.want)
				}
				files, err := os.ReadDir(directory)
				if err != nil || len(files) != 0 {
					t.Fatalf("temporary file leak files=%v error=%v", files, err)
				}
				for _, batch := range retained {
					batch.Release()
				}
				if scope.Live() != 0 || scope.Peak() > scope.Limit() {
					t.Fatalf("scope live=%d peak=%d limit=%d", scope.Live(), scope.Peak(), scope.Limit())
				}
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("scan leaked %+v", usage)
			}
		})
	}
}
