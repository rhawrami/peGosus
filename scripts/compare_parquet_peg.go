package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/peg"
	"github.com/rhawrami/peGosus/pkg/store"
)

func collectRows(result *peg.Result) [][]any {
	fields := result.Schema()
	data := make([][]any, 0, result.NumRows())
	for batchIndex := 0; batchIndex < result.NumBatches(); batchIndex++ {
		batch, _ := result.BatchAt(batchIndex)
		columns := make([]peg.Column, len(fields))
		for col, field := range fields {
			var err error
			columns[col], err = batch.Column(field.Name)
			if err != nil {
				panic(err)
			}
		}
		batch.ForEachActive(func(row int) bool {
			values := make([]any, len(columns))
			for col, column := range columns {
				if !column.IsValid(row) {
					continue
				}
				switch column.Kind() {
				case peg.Int32, peg.Date:
					values[col] = column.Int32s()[row]
				case peg.Int64, peg.TimestampTZ:
					values[col] = column.Int64s()[row]
				case peg.Float32:
					values[col] = column.Float32s()[row]
				case peg.Float64:
					values[col] = column.Float64s()[row]
				case peg.String:
					value, _ := column.StringAt(row)
					values[col] = strings.Clone(value)
				case peg.Bool:
					values[col], _ = column.BoolAt(row)
				}
			}
			data = append(data, values)
			return true
		})
	}
	return data
}

func measureProcessMemory(engine *peg.Engine, prepared *peg.Prepared, warmup, runs, retain int) {
	encoder := json.NewEncoder(os.Stdout)
	held := make([]*peg.Result, 0, retain)
	emit := func(stage string, rows [][]any) {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if err := encoder.Encode(struct {
			Stage       string             `json:"stage"`
			Version     string             `json:"version"`
			HeldResults int                `json:"held_results"`
			Allocator   mem.AllocatorUsage `json:"allocator"`
			Heap        heapSnapshot       `json:"heap"`
			Rows        [][]any            `json:"rows,omitempty"`
		}{stage, runtime.Version(), len(held), engine.MemoryUsage(), heapSnapshot{stats.HeapAlloc, stats.HeapSys, stats.HeapObjects}, rows}); err != nil {
			panic(err)
		}
	}
	execute := func() *peg.Result {
		result, err := prepared.Exec()
		if err != nil {
			panic(err)
		}
		return result
	}
	release := func() {
		for i, result := range held {
			result.Release()
			held[i] = nil
		}
		held = held[:0]
	}
	runtime.GC()
	emit("ready", nil)
	commands := bufio.NewScanner(os.Stdin)
	for commands.Scan() {
		stage := commands.Text()
		switch stage {
		case "cold":
			held = append(held, execute())
		case "release", "release_all":
			release()
		case "warmup":
			for range warmup {
				execute().Release()
			}
			runtime.GC()
		case "steady":
			for range runs {
				execute().Release()
			}
		case "retain":
			for range retain {
				held = append(held, execute())
			}
		case "gc":
			runtime.GC()
		case "validate":
			result := execute()
			rows := collectRows(result)
			result.Release()
			emit(stage, rows)
			continue
		case "quit":
			release()
			prepared.Release()
			runtime.KeepAlive(engine)
			return
		default:
			panic("invalid process-memory command")
		}
		emit(stage, nil)
	}
	if err := commands.Err(); err != nil {
		panic(err)
	}
	panic("process-memory controller disconnected")
}

type measurement struct {
	Name      string              `json:"name"`
	PrepareNS int64               `json:"prepare_ns"`
	TimesNS   []int64             `json:"times_ns"`
	ExecNS    []int64             `json:"exec_ns"`
	CollectNS []int64             `json:"collect_ns"`
	ReleaseNS []int64             `json:"release_ns"`
	Rows      [][]any             `json:"rows"`
	Memory    []memoryMeasurement `json:"memory,omitempty"`
}

type report struct {
	Engine     string        `json:"engine"`
	Source     string        `json:"source"`
	LoadNS     int64         `json:"load_ns,omitempty"`
	Version    string        `json:"version"`
	Workers    int           `json:"workers"`
	GOOS       string        `json:"goos"`
	GOARCH     string        `json:"goarch"`
	GOMAXPROCS int           `json:"gomaxprocs"`
	Experiment string        `json:"goexperiment"`
	Queries    []measurement `json:"queries"`
}

type heapSnapshot struct {
	Alloc   uint64 `json:"heap_alloc_bytes"`
	Sys     uint64 `json:"heap_sys_bytes"`
	Objects uint64 `json:"heap_objects"`
}

type memorySample struct {
	AllocatedBytes  uint64             `json:"allocated_bytes"`
	Allocations     uint64             `json:"allocations"`
	GCs             uint32             `json:"gcs"`
	Heap            heapSnapshot       `json:"heap"`
	Allocator       mem.AllocatorUsage `json:"allocator"`
	ResultAllocator mem.AllocatorUsage `json:"result_allocator"`
}

type memoryMeasurement struct {
	Mode                  string             `json:"mode"`
	Before                heapSnapshot       `json:"before"`
	BeforeAllocator       mem.AllocatorUsage `json:"before_allocator"`
	Samples               []memorySample     `json:"samples"`
	HeldResults           int                `json:"held_results"`
	HeldRows              int                `json:"held_rows"`
	AfterReleaseGC        heapSnapshot       `json:"after_release_gc"`
	AfterReleaseAllocator mem.AllocatorUsage `json:"after_release_allocator"`
}

func measureMemory(engine *peg.Engine, prepared *peg.Prepared, runs, retained int, streaming bool) memoryMeasurement {
	entry := memoryMeasurement{Mode: "release"}
	if streaming {
		entry.Mode = "stream"
	} else if retained > 0 {
		entry.Mode = "retain"
	}
	// Memory instrumentation runs separately so ReadMemStats and forced GC do not affect timings.
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	entry.Before = heapSnapshot{before.HeapAlloc, before.HeapSys, before.HeapObjects}
	entry.BeforeAllocator = engine.MemoryUsage()
	held := make([]*peg.Result, 0, retained+1)
	for i := 0; i < runs; i++ {
		runtime.ReadMemStats(&before)
		var resultUsage mem.AllocatorUsage
		if streaming {
			_, err := prepared.RunContext(context.Background(), func(batch peg.Batch) bool { return true })
			if err != nil {
				panic(err)
			}
		} else {
			result, err := prepared.Exec()
			if err != nil {
				panic(err)
			}
			resultUsage = engine.MemoryUsage()
			if retained == 0 {
				result.Release()
			} else {
				held = append(held, result)
				if len(held) > retained {
					held[0].Release()
					copy(held, held[1:])
					held = held[:len(held)-1]
				}
			}
		}
		runtime.ReadMemStats(&after)
		entry.Samples = append(entry.Samples, memorySample{
			AllocatedBytes: after.TotalAlloc - before.TotalAlloc, Allocations: after.Mallocs - before.Mallocs,
			GCs: after.NumGC - before.NumGC, Heap: heapSnapshot{after.HeapAlloc, after.HeapSys, after.HeapObjects},
			Allocator: engine.MemoryUsage(), ResultAllocator: resultUsage,
		})
	}
	entry.HeldResults = len(held)
	for _, result := range held {
		entry.HeldRows += result.NumRows()
		result.Release()
	}
	held = nil
	runtime.GC()
	runtime.ReadMemStats(&after)
	entry.AfterReleaseGC = heapSnapshot{after.HeapAlloc, after.HeapSys, after.HeapObjects}
	entry.AfterReleaseAllocator = engine.MemoryUsage()
	runtime.KeepAlive(prepared)
	return entry
}

func main() {
	path := flag.String("parquet", "", "Parquet file to scan")
	rows := flag.Int("rows", 0, "number of source rows")
	runs := flag.Int("runs", 7, "timed runs per query")
	warmup := flag.Int("warmup", 2, "untimed runs per query")
	workers := flag.Int("workers", 1, "query workers")
	source := flag.String("source", "parquet", "scan source: parquet or retained table (loading excluded from timings)")
	only := flag.String("only", "", "run one named query for profiling")
	profile := flag.String("cpuprofile", "", "write a CPU profile")
	memory := flag.Bool("memory", false, "measure heap allocations in separate release, streaming, and retained-result passes")
	processMemory := flag.Bool("process-memory", false, "run one query with an external process-memory controller")
	retained := flag.Int("retain", 4, "result retention window in the separate memory pass")
	flag.Parse()
	if *path == "" || *rows <= 0 || *runs < 1 || *warmup < 0 || *workers < 1 || *retained < 1 || (*source != "parquet" && *source != "table") {
		panic("invalid benchmark arguments")
	}
	if *processMemory && (*only == "" || *source != "parquet" || *profile != "" || *memory) {
		panic("process-memory requires one Parquet query without profiling")
	}
	engine := peg.MakeEngine(peg.EngineOptions{MemoryBudget: 512 << 20, Workers: *workers})
	scan := engine.ScanParquet(*path)
	var loadNS int64
	if *source == "table" {
		start := time.Now()
		input, err := os.Open(*path)
		if err != nil {
			panic(err)
		}
		info, err := input.Stat()
		if err != nil {
			panic(err)
		}
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		defer func() { runtime.KeepAlive(a) }()
		reader, failure := parquet.MakeParquetReader(input, info.Size(), a, parquet.ParquetOptions{})
		if failure != nil {
			panic(failure)
		}
		names := make([]string, reader.ColumnCount())
		for i := range names {
			names[i], _, _ = reader.Column(i)
		}
		var batches []*store.Batch
		count := 0
		for {
			batch, failure := reader.Next(context.Background())
			if failure != nil {
				panic(failure)
			}
			if batch == nil {
				break
			}
			batches = append(batches, batch)
			count += batch.Len()
		}
		reader.Close()
		input.Close()
		if count != *rows {
			panic(fmt.Errorf("table loaded %d rows, expected %d", count, *rows))
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		defer table.Release()
		scan = engine.ScanTable(table, names)
		loadNS = time.Since(start).Nanoseconds()
		runtime.GC()
	}
	var profileOutput *os.File
	if *profile != "" {
		output, err := os.Create(*profile)
		if err != nil {
			panic(err)
		}
		defer output.Close()
		profileOutput = output
	}
	profileStarted := false
	queries := []struct {
		name  string
		query peg.Query
	}{
		{"filter_count", scan.Filter(peg.C("id").Ge(0), peg.C("active")).Select(peg.C("id"), peg.C("active")).Agg(peg.CountStar().Alias("rows"))},
		{"selective_count", scan.Filter(peg.C("id").Ge(*rows-4000), peg.C("active")).Select(peg.C("id"), peg.C("active")).Agg(peg.CountStar().Alias("rows"))},
		{"group_sum", scan.Filter(peg.C("id").Ge(*rows/4)).Select(peg.C("id"), peg.C("category")).GroupBy(peg.C("category")).Agg(peg.C("id").Sum().Alias("total")).OrderBy(peg.C("category").Asc().NullsLast())},
		{"top50", scan.Filter(peg.C("score").IsNotNull()).Select(peg.C("id"), peg.C("score")).OrderBy(peg.C("score").Desc(), peg.C("id").Asc()).Limit(50)},
		{"string_filter_sum", scan.Filter(peg.C("category").Eq("north"), peg.C("score").Gt(-23.0)).Select(peg.C("id")).Agg(peg.C("id").Sum().Alias("total"))},
		{"square_sum", scan.Filter(peg.C("score").IsNotNull()).Select(peg.C("score")).Agg(peg.C("score").Sq().Sum().Alias("total"))},
	}
	if *only == "group_day_sum" {
		queries = append(queries, struct {
			name  string
			query peg.Query
		}{"group_day_sum", scan.Filter(peg.C("id").Ge(*rows/4)).Select(peg.C("id"), peg.C("day")).GroupBy(peg.C("day")).Agg(peg.C("id").Sum().Alias("total")).OrderBy(peg.C("day").Asc().NullsLast())})
	}
	output := report{Engine: "peGosus", Source: *source, LoadNS: loadNS, Version: runtime.Version(), Workers: *workers,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GOMAXPROCS: runtime.GOMAXPROCS(0), Experiment: os.Getenv("GOEXPERIMENT")}
	for _, item := range queries {
		if *only != "" && item.name != *only {
			continue
		}
		start := time.Now()
		prepared, err := engine.Prepare(item.query)
		if err != nil {
			panic(fmt.Errorf("%s prepare: %w", item.name, err))
		}
		entry := measurement{Name: item.name, PrepareNS: time.Since(start).Nanoseconds()}
		if *processMemory {
			measureProcessMemory(engine, prepared, *warmup, *runs, *retained)
			return
		}
		for i := 0; i < *warmup+*runs; i++ {
			if i == *warmup && profileOutput != nil && !profileStarted {
				if err := pprof.StartCPUProfile(profileOutput); err != nil {
					panic(err)
				}
				defer pprof.StopCPUProfile()
				profileStarted = true
			}
			start = time.Now()
			result, err := prepared.Exec()
			if err != nil {
				panic(fmt.Errorf("%s execute: %w", item.name, err))
			}
			execDone := time.Now()
			data := collectRows(result)
			collectDone := time.Now()
			result.Release()
			releaseDone := time.Now()
			if i >= *warmup {
				entry.TimesNS = append(entry.TimesNS, releaseDone.Sub(start).Nanoseconds())
				entry.ExecNS = append(entry.ExecNS, execDone.Sub(start).Nanoseconds())
				entry.CollectNS = append(entry.CollectNS, collectDone.Sub(execDone).Nanoseconds())
				entry.ReleaseNS = append(entry.ReleaseNS, releaseDone.Sub(collectDone).Nanoseconds())
			}
			entry.Rows = data
		}
		if *memory {
			entry.Memory = []memoryMeasurement{measureMemory(engine, prepared, *runs, 0, false), measureMemory(engine, prepared, *runs, 0, true), measureMemory(engine, prepared, *runs, *retained, false)}
		}
		prepared.Release()
		output.Queries = append(output.Queries, entry)
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		panic(err)
	}
}
