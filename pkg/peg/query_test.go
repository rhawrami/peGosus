package peg_test

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/peg"
)

func peopleFile(t *testing.T) string {
	t.Helper()
	fixture, err := os.ReadFile("testdata/duckdb-people.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "people.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParquetFluentGroupAndOwnership(t *testing.T) {
	path := peopleFile(t)
	engine := peg.MakeEngine(peg.EngineOptions{MemoryBudget: 8 << 20, Workers: 1})
	query := engine.ScanParquet(path).
		Filter(peg.C("age").Gt(10)).
		WithCols(
			peg.C("income").Mul(20).Alias("cool"),
			peg.C("sex").Replace("m", "male"),
		).
		GroupBy(peg.C("occupation"), peg.C("year")).
		Agg(peg.C("cool").Sum().Alias("cool_sum"))
	prepared, err := engine.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepared.Exec()
	if err != nil {
		t.Fatal(err)
	}
	if result.NumRows() != 4 || result.NumBatches() != 1 {
		t.Fatalf("materialized %d rows in %d batches", result.NumRows(), result.NumBatches())
	}
	fields := result.Schema()
	if len(fields) != 3 || fields[0].Name != "occupation" || fields[1].Name != "year" || fields[2].Name != "cool_sum" || fields[2].Kind != peg.Int64 {
		t.Fatalf("wrong result schema %v", fields)
	}
	batch, ok := result.BatchAt(0)
	if !ok {
		t.Fatal("missing result batch")
	}
	occupation, err := batch.Column("occupation")
	if err != nil {
		t.Fatal(err)
	}
	year, err := batch.Column("year")
	if err != nil {
		t.Fatal(err)
	}
	total, err := batch.Column("cool_sum")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{
		"engineering occupation long/2020": 6000,
		"teaching occupation long/2021":    3000,
		"engineering occupation long/2021": -200,
		"<null>/2020":                      0,
	}
	seen := map[string]bool{}
	if !batch.ForEachActive(func(row int) bool {
		name := "<null>"
		if value, valid := occupation.StringAt(row); valid {
			name = value
		}
		key := name + "/" + strconv.Itoa(int(year.Int32s()[row]))
		if seen[key] || !total.IsValid(row) || total.Int64s()[row] != want[key] {
			t.Errorf("unexpected grouped result %q = %d", key, total.Int64s()[row])
		}
		seen[key] = true
		return true
	}) || len(seen) != len(want) {
		t.Fatalf("wrong grouped keys: %v", seen)
	}
	independent := batch.Retain()
	result.Release()
	prepared.Release()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	retained, err := independent.Column("occupation")
	if err != nil {
		t.Fatal(err)
	}
	longStrings := 0
	independent.ForEachActive(func(row int) bool {
		if value, valid := retained.StringAt(row); valid {
			if value == "" {
				t.Error("retained string backing lost")
			}
			longStrings++
		}
		return true
	})
	if longStrings != 3 {
		t.Fatalf("retained %d non-null occupations", longStrings)
	}
	independent.Release()
}

func TestParquetFluentWithColsAndFilterSelection(t *testing.T) {
	engine := peg.MakeEngine()
	result, err := engine.ScanParquet(peopleFile(t)).
		Filter(peg.C("age").Gt(10)).
		WithCols(peg.C("income").Mul(20).Alias("cool"), peg.C("sex").Replace("m", "male")).
		Select(peg.C("sex"), peg.C("cool")).Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Release()
	if result.NumRows() != 6 {
		t.Fatalf("selected %d rows", result.NumRows())
	}
	count := 0
	for i := range result.NumBatches() {
		batch, _ := result.BatchAt(i)
		sex, err := batch.Column("sex")
		if err != nil {
			t.Fatal(err)
		}
		cool, err := batch.Column("cool")
		if err != nil {
			t.Fatal(err)
		}
		batch.ForEachActive(func(row int) bool {
			if row == 0 {
				if value, ok := sex.StringAt(row); !ok || value != "male" {
					t.Errorf("sex replacement: %q", value)
				}
			}
			if row == 4 && cool.IsValid(row) {
				t.Error("NULL income became a value")
			}
			count++
			return true
		})
	}
	if count != 6 {
		t.Fatalf("visited %d rows", count)
	}
	visited := 0
	if finished := result.ForEachRow(func(batch peg.Batch, row int) bool {
		visited++
		return visited < 2
	}); finished || visited != 2 {
		t.Fatalf("row iterator finished=%t after %d visits", finished, visited)
	}
}

func TestParquetFluentGlobalAggregate(t *testing.T) {
	engine := peg.MakeEngine()
	result, err := engine.ScanParquet(peopleFile(t)).Filter(peg.C("age").Gt(10)).Agg(
		peg.CountStar().Alias("rows"), peg.C("income").Sum().Alias("income_sum"),
	).Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Release()
	if result.NumRows() != 1 {
		t.Fatalf("global aggregate returned %d rows", result.NumRows())
	}
	batch, _ := result.BatchAt(0)
	rows, err := batch.Column("rows")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := batch.Column("income_sum")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Int64s()[0] != 6 || !sum.IsValid(0) || sum.Int64s()[0] != 440 {
		t.Fatalf("global aggregate rows=%v sum=%v", rows.Int64s(), sum.Int64s())
	}
}

func TestParquetStreamingSinkAndRetainedBatch(t *testing.T) {
	path := peopleFile(t)
	engine := peg.MakeEngine(peg.EngineOptions{MemoryBudget: 4 << 20, Workers: 2})
	prepared, err := engine.Prepare(engine.ScanParquet(path).Select(peg.C("occupation")))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	var retained peg.Batch
	status, err := prepared.RunContext(context.Background(), func(batch peg.Batch) bool {
		calls++
		retained = batch.Retain()
		return false
	})
	prepared.Release()
	if status != peg.RunStopped || err != nil || calls != 1 {
		t.Fatalf("early sink termination %v/%v, calls %d", status, err, calls)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	defer retained.Release()
	column, err := retained.Column("occupation")
	if err != nil {
		t.Fatal(err)
	}
	if value, valid := column.StringAt(0); !valid || value != "engineering occupation long" {
		t.Fatalf("retained string after close: %q valid %v", value, valid)
	}
}

func TestParquetFluentStructuredErrorsAndReuse(t *testing.T) {
	path := peopleFile(t)
	engine := peg.MakeEngine()
	for _, test := range []struct {
		query peg.Query
		code  peg.ErrorCode
	}{
		{engine.ScanParquet(path).Filter(peg.C("unknown").Gt(0)), peg.ErrorMissingColumn},
		{engine.ScanParquet(path).WithCols(peg.C("sex").Upper(), peg.C("sex").Lower()), peg.ErrorInvalidExpression},
		{engine.ScanParquet(path + "-missing"), peg.ErrorSource},
	} {
		result, err := test.query.Exec()
		if result != nil {
			result.Release()
			t.Fatal("invalid query produced a result")
		}
		var failure *peg.Error
		if !errors.As(err, &failure) || failure.Code() != test.code {
			t.Fatalf("failure %v, want code %v", err, test.code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := engine.ScanParquet(path).ExecContext(ctx)
	var cancelled *peg.Error
	if !errors.As(err, &cancelled) || cancelled.Code() != peg.ErrorCancelled || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled query: %v", err)
	}
	tiny := peg.MakeEngine(peg.EngineOptions{MemoryBudget: 32})
	_, err = tiny.ScanParquet(path).Exec()
	var resource *peg.Error
	if !errors.As(err, &resource) || resource.Code() != peg.ErrorResourceExhausted {
		t.Fatalf("budget error: %v", err)
	}
	if peg.MakeEngine(peg.EngineOptions{Workers: -1}) != nil {
		t.Fatal("invalid engine accepted")
	}
	prepared, err := engine.Prepare(engine.ScanParquet(path).Filter(peg.C("age").Gt(10)).Select(peg.C("income")))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := prepared.ExecContext(context.Background())
			if err != nil {
				failures <- err
				return
			}
			if res.NumRows() != 6 {
				failures <- errors.New("wrong rows")
			}
			res.Release()
		}()
	}
	wg.Wait()
	prepared.Release()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}
