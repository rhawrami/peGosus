package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/peg"
)

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
	only := flag.String("only", "", "run one named query for profiling")
	profile := flag.String("cpuprofile", "", "write a CPU profile")
	memory := flag.Bool("memory", false, "measure heap allocations in separate release, streaming, and retained-result passes")
	retained := flag.Int("retain", 4, "result retention window in the separate memory pass")
	flag.Parse()
	if *path == "" || *rows <= 0 || *runs < 1 || *warmup < 0 || *workers < 1 || *retained < 1 {
		panic("invalid benchmark arguments")
	}
	engine := peg.MakeEngine(peg.EngineOptions{MemoryBudget: 512 << 20, Workers: *workers})
	if *profile != "" {
		output, err := os.Create(*profile)
		if err != nil {
			panic(err)
		}
		defer output.Close()
		if err := pprof.StartCPUProfile(output); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}
	scan := engine.ScanParquet(*path)
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
	output := report{Engine: "peGosus", Version: runtime.Version(), Workers: *workers,
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
		for i := 0; i < *warmup+*runs; i++ {
			start = time.Now()
			result, err := prepared.Exec()
			if err != nil {
				panic(fmt.Errorf("%s execute: %w", item.name, err))
			}
			execDone := time.Now()
			fields := result.Schema()
			data := make([][]any, 0, result.NumRows())
			for batchIndex := 0; batchIndex < result.NumBatches(); batchIndex++ {
				batch, _ := result.BatchAt(batchIndex)
				columns := make([]peg.Column, len(fields))
				for col, field := range fields {
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
