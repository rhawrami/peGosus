package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"math"
	"math/bits"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/peg"
	"github.com/rhawrami/peGosus/pkg/store"
)

type field struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Nullable bool   `json:"nullable"`
}

type manifest struct {
	Rows            int64   `json:"rows"`
	Parquet         string  `json:"parquet"`
	Dimension       string  `json:"dimension"`
	CSV             string  `json:"csv"`
	DimensionCSV    string  `json:"dimension_csv"`
	Fields          []field `json:"fields"`
	DimensionFields []field `json:"dimension_fields"`
}

type signature struct {
	Rows   int     `json:"rows"`
	SHA256 string  `json:"sha256"`
	Sample [][]any `json:"sample,omitempty"`
}

type measurement struct {
	Name      string        `json:"name"`
	BuildNS   int64         `json:"build_ns"`
	PrepareNS int64         `json:"prepare_ns"`
	TimesNS   []int64       `json:"times_ns,omitempty"`
	ReleaseNS []int64       `json:"release_ns,omitempty"`
	Signature signature     `json:"signature"`
	Error     string        `json:"error,omitempty"`
	ErrorCode peg.ErrorCode `json:"error_code,omitempty"`
}

func makeSource(engine *peg.Engine, path, csvPath, mode string, fields []field) (peg.Query, *store.Table, error) {
	if mode == "csv" {
		kinds := map[string]peg.Kind{"int32": peg.Int32, "int64": peg.Int64, "float64": peg.Float64, "string": peg.String, "bool": peg.Bool, "date": peg.Date, "timestamp": peg.TimestampTZ}
		schema := make([]peg.Field, len(fields))
		for i, f := range fields {
			schema[i] = peg.Field{Name: f.Name, Kind: kinds[f.Kind], Nullable: f.Nullable}
		}
		return engine.ScanCSV(csvPath, schema, peg.CSVOptions{HasHeader: true}), nil, nil
	}
	if mode != "table" {
		return engine.ScanParquet(path), nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return peg.Query{}, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return peg.Query{}, nil, err
	}
	r, readerFailure := parquet.MakeParquetReader(f, info.Size(), mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20}), parquet.ParquetOptions{})
	if readerFailure != nil {
		return peg.Query{}, nil, readerFailure
	}
	defer r.Close()
	var batches []*store.Batch
	for {
		batch, failure := r.Next(context.Background())
		if failure != nil {
			for _, b := range batches {
				b.Release()
			}
			return peg.Query{}, nil, failure
		}
		if batch == nil {
			break
		}
		batches = append(batches, batch)
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	return engine.ScanTable(table, names), table, nil
}

func makeQuery(name string, scan, dimension peg.Query, rows int64) peg.Query {
	revenue := peg.C("price").Mul(peg.C("quantity").Cast(peg.Float64)).Mul(peg.Lit(1.0).Sub(peg.C("discount")))
	switch name {
	case "filter_count":
		return scan.Filter(peg.C("id").Ge(0), peg.C("active")).Select(peg.C("id"), peg.C("active")).Agg(peg.CountStar().Alias("rows"))
	case "selective_count":
		return scan.Filter(peg.C("id").Ge(rows-4000), peg.C("active")).Select(peg.C("id"), peg.C("active")).Agg(peg.CountStar().Alias("rows"))
	case "group_sum":
		return scan.Filter(peg.C("id").Ge(rows/4)).Select(peg.C("id"), peg.C("category")).GroupBy(peg.C("category")).Agg(peg.C("id").Sum().Alias("total")).OrderBy(peg.C("category").Asc().NullsLast())
	case "string_filter_sum":
		return scan.Filter(peg.C("category").Eq("north"), peg.C("score").Gt(-23.0)).Select(peg.C("id")).Agg(peg.C("id").Sum().Alias("total"))
	case "square_sum":
		return scan.Filter(peg.C("score").IsNotNull()).Select(peg.C("score")).Agg(peg.C("score").Sq().Sum().Alias("total"))
	case "group_day_sum":
		return scan.Filter(peg.C("id").Ge(rows/4)).Select(peg.C("id"), peg.C("day")).GroupBy(peg.C("day")).Agg(peg.C("id").Sum().Alias("total")).OrderBy(peg.C("day").Asc().NullsLast())
	case "top10", "top50", "top1000":
		k, _ := strconv.ParseInt(name[3:], 10, 64)
		return scan.Filter(peg.C("score").IsNotNull()).Select(peg.C("id"), peg.C("score")).OrderBy(peg.C("score").Desc(), peg.C("id").Asc()).Limit(k)
	case "full_sort_numeric":
		return scan.Filter(peg.C("score").IsNotNull()).Select(peg.C("id"), peg.C("score")).OrderBy(peg.C("score").Desc(), peg.C("id").Asc())
	case "full_sort_string":
		return scan.Select(peg.C("id"), peg.C("customer_code")).OrderBy(peg.C("customer_code").Asc().NullsLast(), peg.C("id").Asc())
	case "wide_materialize":
		return scan.Select(peg.C("id"), peg.C("category"), peg.C("customer_code"), peg.C("message"), peg.C("flag_text"), peg.C("fiscal_year"), peg.C("price"), peg.C("quantity"), peg.C("score"))
	case "preview_limit":
		return scan.Select(peg.C("id"), peg.C("category"), peg.C("score")).Limit(100)
	case "range_revenue":
		return scan.Filter(peg.C("day").Between(peg.DateDays(19000), peg.DateDays(19060))).Agg(revenue.Sum().Alias("revenue"), peg.C("quantity").Count().Alias("quantity_count"), peg.C("price").Min().Alias("min_price"), peg.C("price").Max().Alias("max_price"))
	case "multi_group_report":
		return scan.GroupBy(peg.C("category"), peg.C("flag_text")).Agg(peg.CountStar().Alias("rows"), revenue.Sum().Alias("revenue"), peg.C("price").Avg().Alias("avg_price"), peg.C("quantity").Count().Alias("quantity_count")).OrderBy(peg.C("category").Asc().NullsLast(), peg.C("flag_text").Asc().NullsLast())
	case "high_card_int_group", "high_card_string_group":
		key := "customer_key"
		if name == "high_card_string_group" {
			key = "customer_code"
		}
		return scan.GroupBy(peg.C(key)).Agg(peg.C("quantity").Sum().Alias("total")).Agg(peg.CountStar().Alias("groups"), peg.C("total").Sum().Alias("total"))
	case "distinct_strings":
		return scan.Select(peg.C("customer_code")).Distinct().Agg(peg.CountStar().Alias("groups"))
	case "count_distinct":
		return scan.Agg(peg.C("customer_code").Count().Distinct().Alias("customers"), peg.C("category").Count().Distinct().Alias("categories"))
	case "string_clean_group":
		return scan.Select(peg.C("message").Coalesce("missing").Lower().Replace("event", "visit").Slice(0, 12).Alias("clean"), peg.C("quantity")).GroupBy(peg.C("clean")).Agg(peg.C("quantity").Sum().Alias("total"), peg.CountStar().Alias("rows")).OrderBy(peg.C("clean").Asc())
	case "string_key_report":
		return scan.Filter(peg.C("flag_text").Eq("Y").Or(peg.C("category").Contains("th"))).GroupBy(peg.C("fiscal_year"), peg.C("category")).Agg(revenue.Sum().Alias("revenue"), peg.CountStar().Alias("rows")).OrderBy(peg.C("fiscal_year").Asc(), peg.C("category").Asc().NullsLast())
	case "case_coalesce_sum":
		x := peg.Case(peg.C("quantity").Eq(0).Or(peg.C("quantity").IsNull()), peg.Null(peg.Float64), peg.C("price").Div(peg.C("quantity").Cast(peg.Float64))).Coalesce(0.0)
		return scan.Agg(x.Sum().Alias("total"))
	case "date_expression_group":
		return scan.GroupBy(peg.C("day").ExtractYear().Cast(peg.Int64).Alias("year"), peg.C("day").ExtractMonth().Alias("month")).Agg(revenue.Sum().Alias("revenue"), peg.CountStar().Alias("rows")).OrderBy(peg.C("year").Asc(), peg.C("month").Asc())
	case "empty_filter_sum":
		return scan.Filter(peg.C("id").Lt(0)).Agg(peg.C("quantity").Sum().Alias("total"), peg.CountStar().Alias("rows"))
	case "selectivity_1", "selectivity_50", "selectivity_99":
		percentage, _ := strconv.ParseInt(name[len("selectivity_"):], 10, 64)
		return scan.Filter(peg.C("selectivity").Lt(percentage)).Agg(revenue.Sum().Alias("total"), peg.CountStar().Alias("rows"))
	case "join_int_report", "join_string_report", "join_left_report", "join_semi_count", "join_anti_count":
		key, rightKey := "segment_key", "segment_key"
		kind := peg.JoinInner
		if name == "join_string_report" {
			key, rightKey = "segment_code", "segment_code"
		}
		if name == "join_left_report" {
			kind = peg.JoinLeft
		}
		if name == "join_semi_count" {
			kind = peg.JoinSemi
		}
		if name == "join_anti_count" {
			kind = peg.JoinAnti
		}
		left := scan.Select(peg.C(key), peg.C("quantity")).As("f")
		right := dimension.Filter(peg.C("enabled")).Select(peg.C(rightKey), peg.C("region")).As("d")
		joined := left.Join(right, kind, []peg.Expr{peg.C("f." + key)}, []peg.Expr{peg.C("d." + rightKey)})
		if kind == peg.JoinSemi || kind == peg.JoinAnti {
			return joined.Agg(peg.CountStar().Alias("rows"))
		}
		return joined.GroupBy(peg.C("d.region").Alias("region")).Agg(peg.C("f.quantity").Sum().Alias("total"), peg.CountStar().Alias("rows")).OrderBy(peg.C("region").Asc().NullsLast())
	case "grouped_self_join":
		left := scan.Select(peg.C("category"), peg.C("quantity")).As("f")
		right := scan.GroupBy(peg.C("category")).Agg(peg.C("quantity").Sum().Alias("group_total")).As("g")
		return left.Join(right, peg.JoinInner, []peg.Expr{peg.C("f.category")}, []peg.Expr{peg.C("g.category")}).Agg(peg.CountStar().Alias("rows"), peg.C("f.quantity").Sum().Alias("quantity"), peg.C("g.group_total").Sum().Alias("joined_total"))
	}
	for _, prefix := range []string{"expression_width_", "repeated_expression_"} {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			width, _ := strconv.Atoi(name[len(prefix):])
			if width < 1 || width > 512 {
				return peg.Query{}
			}
			expressions := make([]peg.Expr, width)
			for i := range expressions {
				x := revenue
				if prefix == "expression_width_" {
					x = peg.Case(peg.C("quantity").Ge((i%8)+1), revenue.Mul(float64(i%5+1)), revenue.Coalesce(0.0).Add(float64(i)/8))
				}
				expressions[i] = x.Sum().Alias(fmt.Sprintf("m%d", i))
			}
			return scan.Agg(expressions...)
		}
	}
	return peg.Query{}
}

func writeValue(h hash.Hash, kind peg.Kind, column peg.Column, row int, buffer []byte) {
	if !column.IsValid(row) {
		h.Write([]byte{0})
		return
	}
	switch kind {
	case peg.String:
		s, _ := column.StringAt(row)
		h.Write([]byte{'s'})
		binary.LittleEndian.PutUint64(buffer, uint64(len(s)))
		h.Write(buffer[:8])
		h.Write([]byte(s))
	case peg.Bool:
		b, _ := column.BoolAt(row)
		buffer[0] = 'b'
		buffer[1] = 0
		if b {
			buffer[1] = 1
		}
		h.Write(buffer[:2])
	case peg.Float32, peg.Float64:
		value := float64(0)
		if kind == peg.Float32 {
			value = float64(column.Float32s()[row])
		} else {
			value = column.Float64s()[row]
		}
		if value == 0 {
			value = 0
		}
		buffer[0] = 'f'
		binary.LittleEndian.PutUint64(buffer[1:], math.Float64bits(value))
		h.Write(buffer[:9])
	default:
		var value int64
		if kind == peg.Int32 || kind == peg.Date {
			value = int64(column.Int32s()[row])
		} else {
			value = column.Int64s()[row]
		}
		buffer[0] = 'i'
		token := strconv.AppendInt(buffer[:1], value, 10)
		token = append(token, '\n')
		h.Write(token)
	}
}

func resultSignature(result *peg.Result, name string) signature {
	h := sha256.New()
	sample := name == "preview_limit"
	unordered := name == "wide_materialize"
	var sum, xor [4]uint64
	fields := result.Schema()
	var buffer [32]byte
	out := signature{Rows: result.NumRows()}
	for i := range result.NumBatches() {
		batch, _ := result.BatchAt(i)
		columns := make([]peg.Column, len(fields))
		for col, f := range fields {
			columns[col], _ = batch.Column(f.Name)
		}
		batch.ForEachActive(func(row int) bool {
			if unordered {
				h.Reset()
			}
			h.Write([]byte{'['})
			var values []any
			if sample {
				values = make([]any, len(fields))
			}
			for col, f := range fields {
				writeValue(h, f.Kind, columns[col], row, buffer[:])
				if sample && columns[col].IsValid(row) {
					switch f.Kind {
					case peg.String:
						s, _ := columns[col].StringAt(row)
						values[col] = strings.Clone(s)
					case peg.Float64:
						values[col] = columns[col].Float64s()[row]
					default:
						values[col] = columns[col].Int32s()[row]
					}
				}
			}
			h.Write([]byte{']'})
			if unordered {
				digest := h.Sum(nil)
				var carry uint64
				for word := range sum {
					value := binary.LittleEndian.Uint64(digest[word*8:])
					sum[word], carry = bits.Add64(sum[word], value, carry)
					xor[word] ^= value
				}
			}
			if sample {
				out.Sample = append(out.Sample, values)
			}
			return true
		})
	}
	if unordered {
		h.Reset()
		for _, words := range [][4]uint64{sum, xor} {
			for _, value := range words {
				binary.LittleEndian.PutUint64(buffer[:], value)
				h.Write(buffer[:8])
			}
		}
		binary.LittleEndian.PutUint64(buffer[:], uint64(out.Rows))
		h.Write(buffer[:8])
	}
	out.SHA256 = hex.EncodeToString(h.Sum(nil))
	return out
}

func executePrepared(prepared *peg.Prepared, timeout time.Duration) (*peg.Result, error) {
	if timeout <= 0 {
		return prepared.Exec()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return prepared.ExecContext(ctx)
}

func memoryProtocol(engine *peg.Engine, prepared *peg.Prepared, name string, warmup, runs, retain int, timeout time.Duration) {
	encoder := json.NewEncoder(os.Stdout)
	var held []*peg.Result
	execute := func() *peg.Result {
		r, e := executePrepared(prepared, timeout)
		if e != nil {
			panic(e)
		}
		return r
	}
	emit := func(stage string, value *signature) {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		encoder.Encode(map[string]any{"stage": stage, "version": runtime.Version(), "held_results": len(held), "allocator": engine.MemoryUsage(), "heap_alloc": stats.HeapAlloc, "heap_sys": stats.HeapSys, "signature": value})
	}
	runtime.GC()
	emit("ready", nil)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		stage := scanner.Text()
		switch stage {
		case "cold":
			held = append(held, execute())
		case "release", "release_all":
			for _, r := range held {
				r.Release()
			}
			held = nil
		case "warmup", "steady":
			count := runs
			if stage == "warmup" {
				count = warmup
			}
			for range count {
				execute().Release()
			}
			if stage == "warmup" {
				runtime.GC()
			}
		case "retain":
			for range retain {
				held = append(held, execute())
			}
		case "gc":
			runtime.GC()
		case "validate":
			r := execute()
			s := resultSignature(r, name)
			r.Release()
			emit(stage, &s)
			continue
		case "quit":
			for _, r := range held {
				r.Release()
			}
			return
		default:
			panic("invalid memory protocol command")
		}
		emit(stage, nil)
	}
	panic("memory controller disconnected")
}

func main() {
	path := flag.String("manifest", "", "fixture manifest")
	mode := flag.String("source", "parquet", "parquet, table, or csv")
	workers := flag.Int("workers", 1, "query workers")
	warmup := flag.Int("warmup", 3, "warmup executions")
	runs := flag.Int("runs", 7, "timed executions")
	only := flag.String("only", "", "comma-separated query names")
	memory := flag.Bool("process-memory", false, "external footprint controller")
	retain := flag.Int("retain", 3, "held-result window")
	budget := flag.Int64("budget", 1<<30, "query budget in bytes")
	timeout := flag.Duration("timeout", 10*time.Second, "per-execution deadline; zero disables")
	cpuProfile := flag.String("cpuprofile", "", "CPU profile path; warmup, setup, and validation are excluded")
	flag.Parse()
	if (*cpuProfile != "" && (*memory || len(splitNames(*only)) != 1)) || *path == "" || *only == "" || *workers < 1 || *runs < 1 || *warmup < 0 || *retain < 1 || *budget <= 0 || (*mode != "parquet" && *mode != "table" && *mode != "csv") {
		panic("invalid workload benchmark arguments")
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		panic(err)
	}
	var fixture manifest
	if err = json.Unmarshal(data, &fixture); err != nil {
		panic(err)
	}
	engine := peg.MakeEngine(peg.EngineOptions{Workers: *workers, MemoryBudget: *budget})
	start := time.Now()
	scan, table, err := makeSource(engine, fixture.Parquet, fixture.CSV, *mode, fixture.Fields)
	if err != nil {
		panic(err)
	}
	if table != nil {
		defer table.Release()
	}
	dimension, dimensionTable, err := makeSource(engine, fixture.Dimension, fixture.DimensionCSV, *mode, fixture.DimensionFields)
	if err != nil {
		panic(err)
	}
	if dimensionTable != nil {
		defer dimensionTable.Release()
	}
	loadNS := time.Since(start).Nanoseconds()
	runtime.GC()
	var measurements []measurement
	for _, name := range splitNames(*only) {
		entry := measurement{Name: name}
		start = time.Now()
		q := makeQuery(name, scan, dimension, fixture.Rows)
		entry.BuildNS = time.Since(start).Nanoseconds()
		start = time.Now()
		prepared, e := engine.Prepare(q)
		entry.PrepareNS = time.Since(start).Nanoseconds()
		if e != nil {
			entry.Error = e.Error()
			if public, ok := e.(*peg.Error); ok {
				entry.ErrorCode = public.Code()
			}
			measurements = append(measurements, entry)
			continue
		}
		if *memory {
			memoryProtocol(engine, prepared, name, *warmup, *runs, *retain, *timeout)
			prepared.Release()
			return
		}
		var profile *os.File
		for i := range *warmup + *runs {
			if *cpuProfile != "" && i == *warmup {
				profile, e = os.Create(*cpuProfile)
				if e != nil {
					panic(e)
				}
				if e = pprof.StartCPUProfile(profile); e != nil {
					panic(e)
				}
			}
			start = time.Now()
			result, e := executePrepared(prepared, *timeout)
			finished := time.Now()
			if e != nil {
				entry.Error = e.Error()
				if public, ok := e.(*peg.Error); ok {
					entry.ErrorCode = public.Code()
				}
				break
			}
			result.Release()
			released := time.Now()
			if i >= *warmup {
				entry.TimesNS = append(entry.TimesNS, finished.Sub(start).Nanoseconds())
				entry.ReleaseNS = append(entry.ReleaseNS, released.Sub(finished).Nanoseconds())
			}
		}
		if profile != nil {
			pprof.StopCPUProfile()
			profile.Close()
		}
		if entry.Error == "" {
			result, e := executePrepared(prepared, *timeout)
			if e != nil {
				entry.Error = e.Error()
			} else {
				entry.Signature = resultSignature(result, name)
				result.Release()
			}
		}
		prepared.Release()
		measurements = append(measurements, entry)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"engine": "peGosus", "version": runtime.Version(), "source": *mode, "workers": *workers, "load_ns": loadNS, "budget": *budget, "timeout_ns": int64(*timeout), "allocator": engine.MemoryUsage(), "queries": measurements})
}

func splitNames(names string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(names); i++ {
		if i == len(names) || names[i] == ',' {
			if i > start {
				result = append(result, names[start:i])
			}
			start = i + 1
		}
	}
	return result
}
