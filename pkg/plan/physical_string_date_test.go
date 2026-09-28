package plan

import (
	"errors"
	"math"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalStringAndDateExpressions(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	var batches []*store.Batch
	texts := []string{"AbC", "a%b", "a_b", "a\x00B", "long ASCIi and \xff", "", "élong string content", "zebra", "A-b"}
	patterns := []string{"A%", "a\\%b", "a\\_b", "a_", "%\x00_", "", "%content", "%", "_-%"}
	days := []int32{0, -1, 11016, 18719, 20630, 365, -365, 730, 20000}
	for _, length := range []int{0, len(texts)} {
		stringsIn := make([][]byte, length)
		patternsIn := make([][]byte, length)
		valid := make([]bool, length)
		for i := range length {
			stringsIn[i] = []byte(texts[i])
			patternsIn[i] = []byte(patterns[i])
			valid[i] = i != 6
		}
		text := store.MakeStringVector(a, stringsIn, valid)
		pattern := store.MakeStringVector(a, patternsIn, nil)
		date := store.MakeVector(a, length, dtype.DateT(), true)
		copy(date.I32s(), days)
		if length != 0 {
			date.Validity().Clear(7)
		}
		batch := store.MakeBatch([]store.Vector{text, pattern, date})
		if length != 0 {
			batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, length, []uint32{0, 2, 4, 8})))
		}
		batches = append(batches, batch)
	}
	table := store.MakeTable(batches)
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"text", "pattern", "date"}, []dtype.Type{dtype.StringT(), dtype.StringT(), dtype.DateT()})).Project(
		MakeColumn("text").Upper(), MakeColumn("text").Lower(), MakeColumn("text").Concat("!"),
		MakeColumn("text").Replace("a", "z"), MakeColumn("text").Replace("", "-"),
		MakeColumn("text").Slice(-2, 4), MakeColumn("text").Like(MakeColumn("pattern")),
		MakeColumn("text").Contains("b"), MakeColumn("date").ExtractYear(), MakeColumn("date").ExtractMonth(),
		MakeColumn("date").ExtractDay(), MakeColumn("date").TruncateYear(), MakeColumn("date").TruncateMonth(),
	))
	if err != nil {
		t.Fatal(err)
	}
	var outputs []*store.Batch
	if !plan.Execute(a, func(batch *store.Batch) bool { outputs = append(outputs, batch.Retain()); return true }) {
		t.Fatal("execute string/date expressions")
	}
	plan.Release()
	table.Release()
	for _, batch := range batches {
		batch.Release()
	}
	if len(outputs) != 2 || outputs[0].Len() != 0 || outputs[1].Len() != len(texts) || outputs[1].ActiveLen() != 4 {
		t.Fatal("expression changed row domain or selection")
	}
	for _, batch := range outputs {
		defer batch.Release()
	}
	output := outputs[1]
	likeWant := []bool{true, true, true, false, false, true, false, true, true}
	for row, text := range texts {
		if row != 6 {
			upper, lower := []byte(text), []byte(text)
			for i := range upper {
				if upper[i] >= 'a' && upper[i] <= 'z' {
					upper[i] -= 'a' - 'A'
				}
				if lower[i] >= 'A' && lower[i] <= 'Z' {
					lower[i] += 'a' - 'A'
				}
			}
			want := []string{string(upper), string(lower), text + "!", strings.ReplaceAll(text, "a", "z")}
			var interleaved []byte
			interleaved = append(interleaved, '-')
			for i := range len(text) {
				interleaved = append(interleaved, text[i], '-')
			}
			want = append(want, string(interleaved))
			stop := min(4, len(text))
			want = append(want, text[:stop])
			for i, expected := range want {
				if got := output.VectorAt(i).Strings()[row].View(); got != expected {
					t.Fatalf("row %d expression %d: got %q, want %q", row, i, got, expected)
				}
			}
			assertPhysicalBoolAt(t, output.VectorAt(6), row, likeWant[row], true)
			assertPhysicalBoolAt(t, output.VectorAt(7), row, strings.Contains(text, "b"), true)
		} else {
			for i := range 8 {
				if output.VectorAt(i).Validity() == nil || output.VectorAt(i).Validity().IsSet(row) {
					t.Fatalf("row %d expression %d lost NULL", row, i)
				}
			}
		}
		if row == 7 {
			for i := 8; i < 13; i++ {
				if output.VectorAt(i).Validity() == nil || output.VectorAt(i).Validity().IsSet(row) {
					t.Fatalf("row %d date expression %d lost NULL", row, i)
				}
			}
			continue
		}
		date := time.Unix(int64(days[row])*86400, 0).UTC()
		want := []int32{int32(date.Year()), int32(date.Month()), int32(date.Day()), int32(time.Date(date.Year(), 1, 1, 0, 0, 0, 0, time.UTC).Unix() / 86400), int32(time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC).Unix() / 86400)}
		for i, expected := range want {
			if got := output.VectorAt(i + 8).I32s()[row]; got != expected {
				t.Fatalf("row %d date expression %d: got %d, want %d", row, i, got, expected)
			}
		}
	}
}

func TestLikeRandomDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(181, 11))
	alphabet := []byte{'a', 'b', 0, '%', '_', '\\', 0xff}
	for range 2000 {
		text := make([]byte, rng.IntN(9))
		pattern := make([]byte, rng.IntN(9))
		for i := range text {
			text[i] = alphabet[rng.IntN(len(alphabet))]
		}
		for i := range pattern {
			pattern[i] = alphabet[rng.IntN(len(alphabet))]
		}
		memo := make(map[[2]int]bool)
		seen := make(map[[2]int]bool)
		var reference func(int, int) bool
		reference = func(i, j int) bool {
			key := [2]int{i, j}
			if seen[key] {
				return memo[key]
			}
			seen[key] = true
			if j == len(pattern) {
				memo[key] = i == len(text)
				return memo[key]
			}
			p := pattern[j]
			if p == '%' {
				memo[key] = reference(i, j+1) || i < len(text) && reference(i+1, j)
				return memo[key]
			}
			if p == '_' {
				memo[key] = i < len(text) && reference(i+1, j+1)
				return memo[key]
			}
			if p == '\\' && j+1 < len(pattern) && (pattern[j+1] == '%' || pattern[j+1] == '_' || pattern[j+1] == '\\') {
				j++
				p = pattern[j]
			}
			memo[key] = i < len(text) && text[i] == p && reference(i+1, j+1)
			return memo[key]
		}
		if got, want := likeMatches(string(text), string(pattern)), reference(0, 0); got != want {
			t.Fatalf("text=%q pattern=%q: got %t, want %t", text, pattern, got, want)
		}
	}
}

func TestBindStringAndDateFunctionTypes(t *testing.T) {
	table := store.MakeEmptyTable([]dtype.Type{dtype.Int32T(), dtype.StringT(), dtype.DateT()})
	scan := MakeScan(table, MakeSchema([]string{"integer", "text", "date"}, []dtype.Type{dtype.Int32T(), dtype.StringT(), dtype.DateT()}))
	for _, expr := range []Expr{
		MakeColumn("integer").Upper(), MakeColumn("text").ExtractYear(),
		MakeColumn("text").Slice(1.5, 2), MakeColumn("text").Replace(1, "x"),
		MakeColumn("date").Like("x"),
	} {
		if plan, err := MakePhysicalPlan(scan.Project(expr)); err == nil || plan != nil {
			t.Fatalf("accepted invalid expression: %+v", expr)
		} else {
			var e *PlanError
			if !errors.As(err, &e) || e.Code() != ErrorTypeMismatch {
				t.Fatalf("got %v, expected ErrorTypeMismatch", err)
			}
		}
	}
	plan, err := MakePhysicalPlan(scan.Project(MakeLiteral(nil).Upper(), MakeLiteral(nil).ExtractYear()))
	if err != nil {
		t.Fatalf("typed NULL inference failed: %v", err)
	}
	if !plan.Schema().FieldAt(0).Type().Equal(dtype.StringT()) || !plan.Schema().FieldAt(1).Type().Equal(dtype.Int32T()) {
		t.Fatal("typed NULL results have incorrect types")
	}
	plan.Release()
	table.Release()
}

func TestDateTruncationOutsidePhysicalRangeIsNull(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	v := store.MakeVector(a, 2, dtype.DateT(), false)
	copy(v.I32s(), []int32{math.MinInt32, 0})
	batch := store.MakeBatch([]store.Vector{v})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"d"}, []dtype.Type{dtype.DateT()}, []bool{false})).Project(MakeColumn("d").TruncateYear(), MakeColumn("d").TruncateMonth()))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	if !plan.Schema().FieldAt(0).Nullable() || !plan.Schema().FieldAt(1).Nullable() {
		t.Fatal("date truncation did not conservatively mark result nullable")
	}
	if !plan.Execute(a, func(output *store.Batch) bool {
		for i := range 2 {
			if output.VectorAt(i).Validity().IsSet(0) || !output.VectorAt(i).Validity().IsSet(1) || output.VectorAt(i).I32s()[1] != 0 {
				t.Fatalf("date truncation %d failed at int32 boundary", i)
			}
		}
		return true
	}) {
		t.Fatal("could not execute date truncation")
	}
	plan.Release()
}
