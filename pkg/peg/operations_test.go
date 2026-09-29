package peg_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/peg"
	"github.com/rhawrami/peGosus/pkg/store"
)

func csvPeople(t *testing.T) (string, []peg.Field) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "people.csv")
	if err := os.WriteFile(path, []byte("id,value,day,text\n1,10,1970-01-01,abc\n2,\\N,1970-02-01,def\n3,30,1970-03-02,abc\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path, []peg.Field{
		{Name: "id", Kind: peg.Int32}, {Name: "value", Kind: peg.Int32, Nullable: true},
		{Name: "day", Kind: peg.Date}, {Name: "text", Kind: peg.String},
	}
}

func TestPublicExpressionsAndCSVScan(t *testing.T) {
	path, schema := csvPeople(t)
	engine := peg.MakeEngine()
	query := engine.ScanCSV(path, schema, peg.CSVOptions{HasHeader: true}).Select(
		peg.C("value").Coalesce(0).Clip(5, 20).Alias("clipped"),
		peg.C("value").Between(10, 20).Alias("between"),
		peg.C("value").NotBetween(10, 20).Alias("outside"),
		peg.C("value").Abs().Neg().Sqrt().Alias("root"),
		peg.C("value").Add(1).Sq().Alias("squared"),
		peg.C("value").Cast(peg.Float64).Alias("converted"),
		peg.C("text").Slice(0, 2).Alias("prefix"),
		peg.C("day").ExtractYear().Alias("year"),
		peg.C("day").ExtractMonth().Alias("month"),
		peg.C("day").ExtractDay().Alias("date"),
		peg.C("day").TruncateYear().Alias("year_start"),
		peg.C("day").TruncateMonth().Alias("month_start"),
		peg.C("day").Sub(peg.DateDays(0)).Alias("days"),
		peg.C("day").Eq(peg.DateDays(31)).Alias("february"),
		peg.Case(peg.C("value").IsNull(), peg.Null(peg.Int32), peg.C("value")).Alias("conditional"),
		peg.Lit(int32(7)).Alias("constant"),
		peg.TimestampMicros(12).Alias("timestamp"),
	)
	result, err := query.Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Release()
	if result.NumRows() != 3 {
		t.Fatalf("got %d rows", result.NumRows())
	}
	batch, _ := result.BatchAt(0)
	column := func(name string) peg.Column {
		t.Helper()
		c, err := batch.Column(name)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := column("clipped"); c.Int32s()[0] != 10 || c.Int32s()[1] != 5 || c.Int32s()[2] != 20 {
		t.Fatalf("clip: %v", c.Int32s())
	}
	between := column("between")
	if first, ok := between.BoolAt(0); !ok || !first {
		t.Fatal("inclusive between failed")
	}
	if _, ok := between.BoolAt(1); ok {
		t.Fatal("between must be NULL")
	}
	if outside, ok := column("outside").BoolAt(2); !ok || !outside {
		t.Fatal("not-between failed")
	}
	if c := column("converted"); c.Float64s()[0] != 10 || c.IsValid(1) {
		t.Fatalf("cast: %v", c.Float64s())
	}
	if c := column("squared"); c.Int32s()[0] != 121 || c.IsValid(1) || c.Int32s()[2] != 961 {
		t.Fatalf("square: %v", c.Int32s())
	}
	if value, ok := column("prefix").StringAt(1); !ok || value != "de" {
		t.Fatalf("slice: %q", value)
	}
	if column("year").Int32s()[0] != 1970 || column("month").Int32s()[1] != 2 || column("date").Int32s()[2] != 2 || column("year_start").Int32s()[2] != 0 || column("month_start").Int32s()[2] != 59 || column("days").Int32s()[2] != 60 {
		t.Fatal("date expression failed")
	}
	if yes, ok := column("february").BoolAt(1); !ok || !yes {
		t.Fatal("date literal comparison failed")
	}
	if c := column("conditional"); c.IsValid(1) || c.Int32s()[2] != 30 {
		t.Fatal("lazy CASE failed")
	}
	if column("constant").Int32s()[2] != 7 || column("timestamp").Int64s()[0] != 12 {
		t.Fatal("literal projection failed")
	}

	counted, err := engine.ScanCSV(path, schema, peg.CSVOptions{HasHeader: true}).Agg(
		peg.CountStar().Alias("rows"), peg.C("text").Count().Distinct().Alias("unique"),
	).Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer counted.Release()
	output, _ := counted.BatchAt(0)
	rows, _ := output.Column("rows")
	unique, _ := output.Column("unique")
	if rows.Int64s()[0] != 3 || unique.Int64s()[0] != 2 {
		t.Fatalf("counts: %v/%v", rows.Int64s(), unique.Int64s())
	}
}

func TestPublicOrderDistinctJoinAndTable(t *testing.T) {
	path, schema := csvPeople(t)
	engine := peg.MakeEngine()
	scan := engine.ScanCSV(path, schema, peg.CSVOptions{HasHeader: true})
	result, err := scan.Select(peg.C("text")).Distinct().OrderBy(peg.C("text").Desc().NullsFirst()).Limit(1).Exec()
	if err != nil {
		t.Fatal(err)
	}
	batch, _ := result.BatchAt(0)
	text, _ := batch.Column("text")
	if result.NumRows() != 1 {
		t.Fatal("ordered distinct limit did not emit one row")
	}
	if value, ok := text.StringAt(0); !ok || value != "def" {
		t.Fatalf("ordered distinct: %q", value)
	}
	result.Release()

	joined := scan.As("l").Join(scan.As("r"), peg.JoinLeft,
		[]peg.Expr{peg.C("l.id")}, []peg.Expr{peg.C("r.id")}, peg.C("r.id").Ge(2))
	result, err = joined.OrderBy(peg.C("l.id").Asc()).Exec()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Release()
	if result.NumRows() != 3 {
		t.Fatalf("left join: %d rows", result.NumRows())
	}
	batch, _ = result.BatchAt(0)
	left, err := batch.Column("l.id")
	if err != nil {
		t.Fatal(err)
	}
	right, err := batch.Column("r.id")
	if err != nil {
		t.Fatal(err)
	}
	if left.Int32s()[0] != 1 || right.IsValid(0) || right.Int32s()[1] != 2 || right.Int32s()[2] != 3 {
		t.Fatal("qualified columns or left join residual failed")
	}
	if _, err := batch.Column("id"); err == nil {
		t.Fatal("unqualified duplicate result column must be ambiguous")
	}

	allocation := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	v := store.MakeVector(allocation, 2, dtype.Int32T(), false)
	copy(v.I32s(), []int32{7, 8})
	sourceBatch := store.MakeBatch([]store.Vector{v})
	table := store.MakeTable([]*store.Batch{sourceBatch})
	sourceBatch.Release()
	prepared, err := engine.Prepare(engine.ScanTable(table, []string{"x"}).Filter(peg.C("x").Gt(7)))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	fromTable, err := prepared.Exec()
	prepared.Release()
	if err != nil {
		t.Fatal(err)
	}
	if fromTable.NumRows() != 1 {
		t.Fatalf("table scan: %d rows", fromTable.NumRows())
	}
	fromTable.Release()

	_, err = joined.Filter(peg.C("id").Gt(0)).Exec()
	var failure *peg.Error
	if !errors.As(err, &failure) || failure.Code() != peg.ErrorAmbiguousColumn {
		t.Fatalf("ambiguous join field: %v", err)
	}
}
