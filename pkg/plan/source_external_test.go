package plan

import (
	"context"
	"crypto/sha256"
	stdcsv "encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const externalIOData = "../../testdata"

type externalScanManifest struct {
	Rows   int `json:"rows"`
	Filter struct {
		Rows           int   `json:"rows"`
		IDSum          int64 `json:"id_sum"`
		NonNullMessage int   `json:"non_null_message"`
	} `json:"filter_id_ge_1000_and_active"`
	Selective struct {
		Rows           int   `json:"rows"`
		IDSum          int64 `json:"id_sum"`
		NonNullMessage int   `json:"non_null_message"`
	} `json:"filter_id_ge_16000_and_active"`
	Files map[string]string `json:"files_sha256"`
}

func pairedScanData(t testing.TB) externalScanManifest {
	t.Helper()
	manifest, err := os.ReadFile(filepath.Join(externalIOData, "manifest.json"))
	if os.IsNotExist(err) {
		t.Skip("prepare the ignored data with scripts/prepare_io_testdata.py")
	}
	if err != nil {
		t.Fatal(err)
	}
	var expected externalScanManifest
	if err := json.Unmarshal(manifest, &expected); err != nil {
		t.Fatal(err)
	}
	if expected.Rows != 16387 || expected.Filter.Rows != 5129 || expected.Filter.IDSum != 44591526 || expected.Filter.NonNullMessage != 4827 || expected.Selective.Rows != 129 || expected.Selective.IDSum != 2089026 || expected.Selective.NonNullMessage != 121 {
		t.Fatalf("unexpected independent writer results: %+v", expected.Filter)
	}
	for _, name := range []string{"paired/scan.csv", "paired/scan_snappy.parquet", "paired/scan_plain.parquet"} {
		f, err := os.Open(filepath.Join(externalIOData, name))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.New()
		_, err = io.Copy(hash, f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(hash.Sum(nil)) != expected.Files[name] {
			t.Fatalf("fixture checksum changed: %s", name)
		}
	}
	return expected
}

func pairedScanTypes() []dtype.Type {
	return []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.BoolT(), dtype.DateT(), dtype.TimestampTZT(), dtype.StringT(), dtype.StringT()}
}

func TestExternalPairedCSVParquet(t *testing.T) {
	expected := pairedScanData(t)
	for _, name := range []string{"scan_snappy.parquet", "scan_plain.parquet"} {
		t.Run(name, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			pqFile, err := os.Open(filepath.Join(externalIOData, "paired", name))
			if err != nil {
				t.Fatal(err)
			}
			defer pqFile.Close()
			info, err := pqFile.Stat()
			if err != nil {
				t.Fatal(err)
			}
			pq, failure := parquet.MakeParquetReader(pqFile, info.Size(), a, parquet.ParquetOptions{BatchSize: 307})
			if failure != nil {
				t.Fatal(failure)
			}
			defer pq.Close()
			csvFile, err := os.Open(filepath.Join(externalIOData, "paired", "scan.csv"))
			if err != nil {
				t.Fatal(err)
			}
			defer csvFile.Close()
			nullable := make([]bool, 9)
			for i := range nullable {
				nullable[i] = true
			}
			reader, csvErr := csv.MakeCSVReader(csvFile, a, pairedScanTypes(), nullable, csv.CSVOptions{BatchSize: 257, HasHeader: true})
			if csvErr != nil {
				t.Fatal(csvErr)
			}
			var pqBatch, csvBatch, retained *store.Batch
			defer func() {
				if pqBatch != nil {
					pqBatch.Release()
				}
				if csvBatch != nil {
					csvBatch.Release()
				}
				if retained != nil {
					retained.Release()
				}
			}()
			pqAt, csvAt := 0, 0
			for row := range expected.Rows {
				if pqBatch == nil || pqAt == pqBatch.Len() {
					if pqBatch != nil {
						pqBatch.Release()
					}
					pqBatch, failure = pq.Next(context.Background())
					if failure != nil || pqBatch == nil {
						t.Fatalf("Parquet row %d: %v", row, failure)
					}
					pqAt = 0
					if retained == nil {
						retained = pqBatch.Retain()
					}
				}
				if csvBatch == nil || csvAt == csvBatch.Len() {
					if csvBatch != nil {
						csvBatch.Release()
					}
					csvBatch, csvErr = reader.Next(context.Background())
					if csvErr != nil || csvBatch == nil {
						t.Fatalf("CSV row %d: %v", row, csvErr)
					}
					csvAt = 0
				}
				for col, kind := range pairedScanTypes() {
					p, c := pqBatch.VectorAt(col), csvBatch.VectorAt(col)
					pv := p.Validity() == nil || p.Validity().IsSet(pqAt)
					cv := c.Validity() == nil || c.Validity().IsSet(csvAt)
					if pv != cv {
						t.Fatalf("row %d col %d validity differs", row, col)
					}
					if !pv {
						continue
					}
					same := false
					switch kind.ID() {
					case dtype.INT32T, dtype.DATET:
						same = p.I32s()[pqAt] == c.I32s()[csvAt]
					case dtype.INT64T, dtype.TIMESTAMPTZT:
						same = p.I64s()[pqAt] == c.I64s()[csvAt]
					case dtype.FLOAT32T:
						same = math.Float32bits(p.F32s()[pqAt]) == math.Float32bits(c.F32s()[csvAt])
					case dtype.FLOAT64T:
						same = math.Float64bits(p.F64s()[pqAt]) == math.Float64bits(c.F64s()[csvAt])
					case dtype.BOOLT:
						same = (p.Bools()[pqAt>>3]>>(pqAt&7))&1 == (c.Bools()[csvAt>>3]>>(csvAt&7))&1
					case dtype.STRT:
						same = p.Strings()[pqAt].View() == c.Strings()[csvAt].View()
					}
					if !same {
						t.Fatalf("row %d col %d differs between Parquet and CSV", row, col)
					}
				}
				pqAt++
				csvAt++
			}
			if pqBatch != nil {
				pqBatch.Release()
				pqBatch = nil
			}
			if csvBatch != nil {
				csvBatch.Release()
				csvBatch = nil
			}
			if batch, failure := pq.Next(context.Background()); batch != nil || failure != nil {
				t.Fatalf("Parquet end: %v/%v", batch, failure)
			}
			if batch, failure := reader.Next(context.Background()); batch != nil || failure != nil {
				t.Fatalf("CSV end: %v/%v", batch, failure)
			}
			pq.Close()
			pqFile.Close()
			if retained.VectorAt(8).Strings()[1].View() != "payload-1;payload-1;payload-1;payload-1;" {
				t.Fatal("retained string backing was lost")
			}
		})
	}
}

func TestExternalScanFilterProjectReference(t *testing.T) {
	expected := pairedScanData(t)
	types := pairedScanTypes()
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(types))
	for i := range nullable {
		nullable[i] = true
	}
	schema := MakeSchemaWithNullability(names, types, nullable)
	for _, name := range []string{"scan_snappy.parquet", "scan_plain.parquet", "scan.csv"} {
		t.Run(name, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			path := filepath.Join(externalIOData, "paired", name)
			var scan LogicalPlan
			if filepath.Ext(path) == ".csv" {
				scan = MakeCSVScan(path, schema, csv.CSVOptions{HasHeader: true, BatchSize: 257})
			} else {
				scan = MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 307})
			}
			p, err := MakePhysicalPlan(scan.Filter(MakeColumn("id").Ge(1000)).Filter(MakeColumn("active")).Project(MakeColumn("id"), MakeColumn("message")))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Release()
			count, messages := 0, 0
			var sum int64
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20}, func(batch *store.Batch) bool {
				visit := func(row int) {
					count++
					sum += int64(batch.VectorAt(0).I32s()[row])
					if batch.VectorAt(1).Validity() == nil || batch.VectorAt(1).Validity().IsSet(row) {
						messages++
					}
				}
				if selection := batch.Selection(); selection == nil {
					for row := range batch.Len() {
						visit(row)
					}
				} else if bitmap, ok := selection.AsBitMap(); ok {
					for row := range batch.Len() {
						if bitmap.IsSet(row) {
							visit(row)
						}
					}
				} else if offsets, ok := selection.AsSelVec(); ok {
					for _, row := range offsets.Offsets() {
						visit(int(row))
					}
				} else {
					t.Fatal("invalid row selection")
				}
				return true
			})
			if result.Code() != ExecutionCompleted || count != expected.Filter.Rows || sum != expected.Filter.IDSum || messages != expected.Filter.NonNullMessage {
				t.Fatalf("query %v/%v: rows %d sum %d non-null message %d", result.Code(), result.Err(), count, sum, messages)
			}
		})
	}
}

func TestExternalParquetPruningSkipsCorruptExcludedGroup(t *testing.T) {
	pairedScanData(t)
	input, err := os.ReadFile(filepath.Join(externalIOData, "paired", "scan_snappy.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	input[4] = 0xff
	path := filepath.Join(t.TempDir(), "corrupt-first-group.parquet")
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatal(err)
	}
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(names))
	for i := range nullable {
		nullable[i] = true
	}
	schema := MakeSchemaWithNullability(names, pairedScanTypes(), nullable)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	unpruned, err := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{}).Project(MakeColumn("id")))
	if err != nil {
		t.Fatal(err)
	}
	result := unpruned.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 1}, func(*store.Batch) bool { t.Fatal("corrupt page reached sink"); return true })
	unpruned.Release()
	if result.Code() != ExecutionSourceFailure {
		t.Fatalf("unpruned corrupt group: %v/%v", result.Code(), result.Err())
	}
	pruned, err := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{}).Filter(MakeColumn("id").Ge(2048)).Project(MakeColumn("id")))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	result = pruned.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20}, func(batch *store.Batch) bool {
		count += batch.ActiveLen()
		return true
	})
	if result.Code() != ExecutionCompleted || count != 16387-2048 {
		t.Fatalf("pruned result: %v/%v, %d rows", result.Code(), result.Err(), count)
	}
	count = 0
	parallelResult, parallel := pruned.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(batch *store.Batch) bool {
		count += batch.ActiveLen()
		return true
	})
	if !parallel || parallelResult.Code() != ExecutionCompleted || count != 16387-2048 {
		t.Fatalf("parallel pruning %t %v/%v, %d rows", parallel, parallelResult.Code(), parallelResult.Err(), count)
	}
	count = 0
	autoResult, automatic := pruned.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20}, func(batch *store.Batch) bool {
		count += batch.ActiveLen()
		return true
	})
	if !automatic || autoResult.Code() != ExecutionCompleted || count != 16387-2048 {
		t.Fatalf("auto Parquet workers %t %v/%v, %d rows", automatic, autoResult.Code(), autoResult.Err(), count)
	}
	stopCalls := 0
	stopped := pruned.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(*store.Batch) bool { stopCalls++; return false })
	if stopped.Code() != ExecutionStopped || stopCalls != 1 {
		t.Fatalf("parallel Parquet sink stop %v/%v, %d calls", stopped.Code(), stopped.Err(), stopCalls)
	}
	pruned.Release()
}

func TestExternalSelectiveScanReference(t *testing.T) {
	expected := pairedScanData(t)
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(names))
	for i := range nullable {
		nullable[i] = true
	}
	schema := MakeSchemaWithNullability(names, pairedScanTypes(), nullable)
	for _, name := range []string{"scan_snappy.parquet", "scan_plain.parquet", "scan.csv"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(externalIOData, "paired", name)
			var scan LogicalPlan
			if filepath.Ext(name) == ".csv" {
				scan = MakeCSVScan(path, schema, csv.CSVOptions{HasHeader: true})
			} else {
				scan = MakeParquetScan(path, schema, parquet.ParquetOptions{})
			}
			p, err := MakePhysicalPlan(scan.Filter(MakeColumn("id").Ge(16000)).Filter(MakeColumn("active")).Aggregate(MakeCountStar(), MakeSum(MakeColumn("id")), MakeCount(MakeColumn("message"))))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Release()
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			calls := 0
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20}, func(batch *store.Batch) bool {
				calls++
				if batch.Len() != 1 || batch.VectorAt(0).I64s()[0] != int64(expected.Selective.Rows) || batch.VectorAt(1).I64s()[0] != expected.Selective.IDSum || batch.VectorAt(2).I64s()[0] != int64(expected.Selective.NonNullMessage) {
					t.Fatalf("wrong selective aggregate from %s", name)
				}
				return true
			})
			if result.Code() != ExecutionCompleted || calls != 1 {
				t.Fatalf("selective result %v/%v, calls %d", result.Code(), result.Err(), calls)
			}
			if filepath.Ext(name) == ".parquet" {
				calls = 0
				parallelResult, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(batch *store.Batch) bool {
					calls++
					if batch.VectorAt(0).I64s()[0] != int64(expected.Selective.Rows) || batch.VectorAt(1).I64s()[0] != expected.Selective.IDSum || batch.VectorAt(2).I64s()[0] != int64(expected.Selective.NonNullMessage) {
						t.Error("parallel aggregation differs from DuckDB")
					}
					return true
				})
				if !parallel || parallelResult.Code() != ExecutionCompleted || calls != 1 {
					t.Fatalf("parallel Parquet aggregate %t %v/%v, %d calls", parallel, parallelResult.Code(), parallelResult.Err(), calls)
				}
			}
		})
	}
}

func TestExternalParallelParquetGroupingAgainstCSV(t *testing.T) {
	pairedScanData(t)
	file, err := os.Open(filepath.Join(externalIOData, "paired", "scan.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reference := stdcsv.NewReader(file)
	if _, err := reference.Read(); err != nil {
		t.Fatal(err)
	}
	type stats struct{ count, sum, nonNull int64 }
	want := make(map[string]stats)
	for {
		record, err := reference.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		key := record[7]
		if key == `\N` {
			key = "<null>"
		}
		id, err := strconv.ParseInt(record[0], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		current := want[key]
		current.count++
		current.sum += id
		if record[8] != `\N` {
			current.nonNull++
		}
		want[key] = current
	}
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(names))
	for i := range nullable {
		nullable[i] = true
	}
	scan := MakeParquetScan(filepath.Join(externalIOData, "paired", "scan_snappy.parquet"), MakeSchemaWithNullability(names, pairedScanTypes(), nullable), parquet.ParquetOptions{})
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	p, err := MakePhysicalPlan(scan.GroupBy([]Expr{MakeColumn("category")}, MakeCountStar(), MakeSum(MakeColumn("id")), MakeCount(MakeColumn("message"))))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	for _, workers := range []int{1, 4} {
		seen := make(map[string]bool)
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: workers}, func(batch *store.Batch) bool {
			for row := range batch.Len() {
				key := "<null>"
				if v := batch.VectorAt(0); v.Validity() == nil || v.Validity().IsSet(row) {
					key = v.Strings()[row].View()
				}
				stats, exists := want[key]
				if !exists || seen[key] || batch.VectorAt(1).I64s()[row] != stats.count || batch.VectorAt(2).I64s()[row] != stats.sum || batch.VectorAt(3).I64s()[row] != stats.nonNull {
					t.Errorf("workers=%d wrong Parquet group %q", workers, key)
				}
				seen[key] = true
			}
			return true
		})
		if result.Code() != ExecutionCompleted || len(seen) != len(want) {
			t.Fatalf("workers=%d grouped scan %v/%v groups %d/%d", workers, result.Code(), result.Err(), len(seen), len(want))
		}
	}
}
