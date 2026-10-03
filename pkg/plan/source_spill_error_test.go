package plan

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestSpillScanStructuredErrors(t *testing.T) {
	directory := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	csvInvalid := write("invalid.csv", []byte("not-an-integer\n"))
	csvLarge := write("large.csv", []byte(strings.Repeat("x", 512<<10)+"\n"))
	parquetData, err := base64.StdEncoding.DecodeString(parquetScanFixture)
	if err != nil {
		t.Fatal(err)
	}
	parquetPath := write("input.parquet", parquetData)
	badParquet := write("bad.parquet", []byte("PAR1bad"))
	csvSchema := MakeSchema([]string{"n"}, []dtype.Type{dtype.Int32T()})
	parquetSchema := MakeSchemaWithNullability([]string{"id", "txt"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true})
	for _, tc := range []struct {
		name            string
		scan            LogicalPlan
		column          string
		expected        ExecutionCode
		csvCode         csv.CSVErrorCode
		parquetCode     parquet.ParquetErrorCode
		missing, cancel bool
	}{
		{"csv-resource", MakeCSVScan(csvLarge, MakeSchema([]string{"n"}, []dtype.Type{dtype.StringT()}), csv.CSVOptions{BatchSize: 1}), "n", ExecutionResourceExhausted, csv.CSVResourceExhausted, 0, false, false},
		{"csv-invalid", MakeCSVScan(csvInvalid, csvSchema, csv.CSVOptions{BatchSize: 1}), "n", ExecutionSourceFailure, csv.CSVInvalidValue, 0, false, false},
		{"csv-missing", MakeCSVScan(filepath.Join(directory, "missing.csv"), csvSchema, csv.CSVOptions{}), "n", ExecutionSourceFailure, 0, 0, true, false},
		{"parquet-resource", MakeParquetScan(parquetPath, parquetSchema, parquet.ParquetOptions{MaxFooterBytes: 1}), "id", ExecutionResourceExhausted, 0, parquet.ParquetResourceExhausted, false, false},
		{"parquet-invalid", MakeParquetScan(badParquet, parquetSchema, parquet.ParquetOptions{}), "id", ExecutionSourceFailure, 0, parquet.ParquetInvalid, false, false},
		{"parquet-missing", MakeParquetScan(filepath.Join(directory, "missing.parquet"), parquetSchema, parquet.ParquetOptions{}), "id", ExecutionSourceFailure, 0, 0, true, false},
		{"cancelled", MakeCSVScan(csvInvalid, csvSchema, csv.CSVOptions{}), "n", ExecutionCancelled, 0, 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			p, failure := MakePhysicalPlan(tc.scan.OrderBy(MakeOrderKey(MakeColumn(tc.column))))
			if failure != nil {
				t.Fatal(failure)
			}
			for _, spill := range []bool{false, true} {
				ctx, cancel := context.WithCancel(context.Background())
				if tc.cancel {
					cancel()
				}
				scope := mem.MakeAllocationScope(a, 128<<10)
				temporary := t.TempDir()
				options := ExecutionOptions{MemoryBudget: scope.Limit(), Workers: 1}
				if spill {
					options.SpillDirectory = temporary
				}
				result := p.executeWithScope(ctx, a, options, func(*store.Batch) bool { t.Error("failed source reached sink"); return true }, scope)
				cancel()
				if result.Code() != tc.expected {
					t.Fatalf("spill=%t code=%v want=%v cause=%v", spill, result.Code(), tc.expected, result.Err())
				}
				if tc.csvCode != 0 {
					var source *csv.CSVError
					if !errors.As(result.Err(), &source) || source.Code() != tc.csvCode {
						t.Fatalf("spill=%t CSV cause=%v", spill, result.Err())
					}
				}
				if tc.parquetCode != 0 {
					var source *parquet.ParquetError
					if !errors.As(result.Err(), &source) || source.Code() != tc.parquetCode {
						t.Fatalf("spill=%t Parquet cause=%v", spill, result.Err())
					}
				}
				if tc.missing && !errors.Is(result.Err(), os.ErrNotExist) {
					t.Fatalf("spill=%t missing cause=%v", spill, result.Err())
				}
				if tc.cancel && !errors.Is(result.Err(), context.Canceled) {
					t.Fatalf("spill=%t cancel cause=%v", spill, result.Err())
				}
				if files, err := os.ReadDir(temporary); err != nil || len(files) != 0 {
					t.Fatalf("spill=%t files=%v cause=%v", spill, files, err)
				}
				if scope.Live() != 0 {
					t.Fatalf("spill=%t live=%d", spill, scope.Live())
				}
			}
			p.Release()
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("leaked %+v", usage)
			}
		})
	}
}

func TestSpillPrefixAllocationStructuredResource(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	text := store.MakeStringVector(a, [][]byte{[]byte("x")}, nil)
	batch := store.MakeBatch([]store.Vector{text})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	query := MakeScan(table, MakeSchema([]string{"s"}, []dtype.Type{dtype.StringT()})).Project(MakeColumn("s").Concat(MakeLiteral(strings.Repeat("x", 256<<10))).Alias("grown")).OrderBy(MakeOrderKey(MakeColumn("grown")))
	p, err := MakePhysicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	for _, spill := range []bool{false, true} {
		directory := t.TempDir()
		scope := mem.MakeAllocationScope(a, 128<<10)
		options := ExecutionOptions{MemoryBudget: scope.Limit(), Workers: 1}
		if spill {
			options.SpillDirectory = directory
		}
		result := p.executeWithScope(context.Background(), a, options, func(*store.Batch) bool { t.Error("over-budget prefix reached sink"); return true }, scope)
		if result.Code() != ExecutionResourceExhausted {
			t.Fatalf("spill=%t code=%v cause=%v exhausted=%t", spill, result.Code(), result.Err(), scope.Exhausted())
		}
		if scope.Live() != 0 {
			t.Fatalf("spill=%t live=%d", spill, scope.Live())
		}
		if files, err := os.ReadDir(directory); err != nil || len(files) != 0 {
			t.Fatalf("spill=%t files=%v cause=%v", spill, files, err)
		}
	}
	p.Release()
	table.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}
