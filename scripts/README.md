# Development on Linux amd64

The engine requires Go 1.27.1, as declared in `go.mod`. Normal builds use
scalar Go and the existing CPU-dispatched assembly. CSV's experimental AVX2
intrinsics are enabled separately with `GOEXPERIMENT=simd`; keep normal builds
working and do not set the experiment globally. AVX2 dispatch detects CPU
support at runtime; leave `GOAMD64` at its portable default unless deliberately
measuring a different target.

## Dependencies

On the current Fedora x64 machine, Go 1.27.1 is installed under
`~/.local/lib/go1.27.1`, with `go` and `gofmt` links in `~/.local/bin`, which is
already on the shell's PATH. The official Linux amd64 archive's SHA-256 was
verified before installation. The Fedora system Go remains available at
`/usr/bin/go`. Git, GCC (needed for the Go race detector), Python 3, and curl
are available. The engine itself uses no cgo.

Optional development tools installed in `~/.local/bin`:

- `gopls` v0.23.0 for Go editor support.
- `benchstat` from `golang.org/x/perf` v0.0.0-20260929162123-406019bb8b68.
- Standalone `pprof` from `github.com/google/pprof`
  v0.0.0-20261002000307-77d3b59017a0. This Go archive lacks `go tool pprof`.

Run from the repository root:

```bash
go version
go mod download
go mod verify
python3 -m venv .venv
.venv/bin/python -m pip install -r scripts/requirements-dev.txt
.venv/bin/python scripts/prepare_io_testdata.py
```

The Python environment pins DuckDB 1.5.6 and Polars 1.34.0 to match the
historical benchmark reports. Python 3.14 works with their Linux wheels.
The fixture preparation script downloads seven checksum-pinned Apache Parquet
files and generates 16,387 paired CSV/Parquet rows. `.venv/`, Python bytecode,
and generated `testdata/` are ignored by Git. No Python environment activation
is required when using the explicit interpreter paths below.

DataFusion is an optional fourth comparator. Install it separately so the
default DuckDB/Polars environment stays independent:

```bash
.venv/bin/python -m pip install -r scripts/requirements-datafusion.txt
```

This pins DataFusion Python 54.0.0, PyArrow 25.0.1, and cloudpickle 3.1.2. It adds
no Go dependencies. Select it with `--engines peGosus,DuckDB,Polars,DataFusion`;
omitting `--engines` preserves the historical three-engine comparison. The
adapter supports Parquet and retained tables. Its CSV mode is explicitly
rejected because the pinned reader did not preserve the fixtures' `\N` null
marker in validation; use `--source parquet` or `--source table` when including
DataFusion. DataFusion collects native Arrow batches inside the timed region;
Arrow-to-Python row conversion occurs only during result validation. Each query
execution includes SQL planning and collection. The runtime uses a 1 GiB fair
spill pool and allows disk spilling; target partitions follow `--threads` but
are not a cap on every runtime thread.

## Correctness checks

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go test -gcflags=all=-d=checkptr=2 -count=1 ./...
GOEXPERIMENT=simd go test -count=1 ./...
GOEXPERIMENT=simd go test -race -count=1 ./...
GOEXPERIMENT=simd go test -gcflags=all=-d=checkptr=2 -count=1 ./...
GODEBUG=cpu.avx2=off go test -count=1 ./pkg/op/...
.venv/bin/python -m unittest discover -s scripts -p '*_test.py' -v
.venv/bin/python -m unittest discover -s scripts -p 'test_*.py' -v
```

Generate external fixtures before testing so external IO tests run instead of
skipping. Historical macOS physical-footprint tests skip on Linux. Linux
memory-counter tests run without optional packages; DataFusion parity tests
skip unless its optional requirements and DuckDB are installed.

## Cross-engine smoke check

The timing harness runs on Linux and validates results against DuckDB and
Polars. This small run exercises all 37 workloads at one and four workers:

```bash
.venv/bin/python scripts/compare_workload_engines.py \
  --scratch /tmp/peg-linux-workloads --cases balanced:512 --threads 1,4 \
  --rounds 1 --runs 1 --warmup 0 --json /tmp/peg-linux-smoke.json
```

Use repeated samples and warmups for actual performance measurements, and run
them separately from correctness suites. Old M4 timings are historical
references; establish new native amd64 baselines before comparing changes.

`compare_parquet_memory.py` and the workload harness's `--memory` mode support
macOS and Linux. macOS retains libproc's physical-footprint measurement and
resettable interval peak. Linux samples approximate `/proc/PID/status` RSS every
5 ms plus stage endpoints, with precise `smaps_rollup` RSS/PSS endpoint snapshots
when accessible. These measurements have different definitions; compare each
engine on the same platform and metric.

Linux reports these fields:

- `resident_size`: current approximate `VmRSS`; anonymous, file, and shared RSS
  components are recorded separately.
- `interval_peak_sampled_rss_bytes`: highest observed RSS in one stage. Samples
  can miss transient allocations; this is not a resettable kernel peak.
- `lifetime_max_resident_size`: lifetime `VmHWM`, kept separate from stage peaks.
- `resident_size_smaps` and `proportional_set_size`: precise endpoint RSS/PSS,
  or `null` when unavailable.
- `peak_memory_bytes` and `peak_growth_bytes`: observed stage peak and growth
  above its baseline, identified by `memory_metric: "sampled_rss"`.
- `rss_samples`, `requested_sample_period_ns`, `largest_sample_gap_ns`, and
  `sampling_errors`: evidence of sampling coverage. Inspect these when judging
  short stages; requested intervals are not guarantees of scheduler precision.

Memory runs use one fresh process per engine/query/worker/round and keep native
results through cold, release, warmup, steady, retention, release-all, and GC
stages. Startup, retained input, runtime caches, and output representation affect
absolute process memory. PSS, sampled RSS, requested allocator bytes, and slab
capacity are distinct metrics. Run memory measurements separately from timing
suites because monitoring adds overhead. Linux counter definitions follow the
[Linux kernel's `/proc` documentation](https://docs.kernel.org/filesystems/proc.html).

See `HANDOFF.md` for current implementation, benchmark methodology, and next
priorities, and `KERNELS_TODO.md` for architecture-specific correctness debt.

## Machine validation: October 2, 2026

Fedora 44, Linux amd64, AMD Ryzen 7 7700X with AVX2:

- Full normal, race, and aggressive checkptr suites passed, both with and
  without `GOEXPERIMENT=simd`.
- Kernel tests passed with AVX2 dispatch disabled.
- All 22 external IO test cases passed; none skipped.
- Four Python tests passed; two macOS memory-counter tests skipped.
- The pure-Go (`CGO_ENABLED=0`) runner completed all 37 Parquet workloads on
  `balanced:512` at one/four workers against DuckDB and Polars: zero query
  failures and zero result mismatches. Report: `/tmp/peg-linux-smoke.json`.

This smoke run verifies the environment and result agreement, rather than
establishing a statistically useful performance baseline.

## Native amd64 timing baseline

Build commands in the historical sections describe their original source
revisions. On current sources, build the round-two runner documented below;
keep earlier frozen binaries untouched.

The October 2, 2026 Ryzen 7 7700X baseline uses the normal Go build with native
AVX2 dispatch, `CGO_ENABLED=0`, and `GOAMD64=v1`. It covers all 37 Parquet queries
on five fixtures (512 rows, 65K rows, one million rows, skewed/null-heavy 65K,
and PLAIN long-string 65K), plus seven focused queries on a retained million-row
table. Each source runs at one/four workers with two rounds, two warmups, and
five timed samples per round. Worker and engine order reverses in the second
round. Correctness suites and benchmark suites run separately.

- [Human-readable summary](benchmark-results/2026-10-02-amd64-baseline-summary.txt)
- [Pooled medians, ranges, per-round medians, and validation](benchmark-results/2026-10-02-amd64-baseline-summary.json)
- [Full Parquet samples and fingerprints](benchmark-results/2026-10-02-amd64-baseline-workloads.json)
- [Retained-table samples and fingerprints](benchmark-results/2026-10-02-amd64-baseline-table.json)
- [Machine settings, source/binary hashes, and reproduction commands](benchmark-results/2026-10-02-amd64-baseline-machine.json)

The fixed runner and checksum-verified fixtures persist under ignored
`testdata/amd64-baseline/`. These are desktop measurements with the existing
`powersave` governor and adaptive CPU boost. Large output adapters differ:
DuckDB includes Python row conversion, Polars shares column buffers, and
peGosus returns owned results. Use this baseline for future changes on the same
machine; historical M4 results are a separate hardware baseline.

## Portable string-prefix comparison

The first efficiency change after setup normalizes compact string-sort prefixes
and limits full comparisons to collisions. It uses ordinary portable Go; the
worker caps also cover upstream scan work and do not imply parallel sorting.
The broader implementation plan is [the research roadmap](../docs/RESEARCH_ROADMAP.md).

- [Paired medians and limitations](benchmark-results/2026-10-02-string-prefix-summary.txt)
- [Ranges, round medians and allocator snapshots](benchmark-results/2026-10-02-string-prefix-summary.json)
- [All paired samples, commands, source/fixture/binary hashes](benchmark-results/2026-10-02-string-prefix-paired.json)
- [Full 37-query cross-engine smoke validation](benchmark-results/2026-10-02-string-prefix-smoke.json)
- [Sort profile before](benchmark-results/2026-10-02-string-prefix-profile-before.txt),
  [after](benchmark-results/2026-10-02-string-prefix-profile-after.txt), and
  [join diagnostic](benchmark-results/2026-10-02-string-prefix-profile-join.txt).

There are 1,440 paired timed samples over five Parquet cases, one retained-table
case, six queries, two worker caps, and two reversed-order rounds. Each paired
record stores its complete command and environment. To rerun a single case:

```bash
CGO_ENABLED=0 GOEXPERIMENT= GOAMD64=v1 go build \
  -o testdata/string-prefix/peg-workloads ./scripts/workload_peg
GOMAXPROCS=1 testdata/amd64-baseline/peg-workloads \
  -manifest testdata/amd64-baseline/fixtures/balanced-1048576/manifest.json \
  -workers 1 -warmup 2 -runs 5 -only full_sort_string,full_sort_numeric,group_sum,top50,wide_materialize,repeated_expression_32
GOMAXPROCS=1 testdata/string-prefix/peg-workloads \
  -manifest testdata/amd64-baseline/fixtures/balanced-1048576/manifest.json \
  -workers 1 -warmup 2 -runs 5 -only full_sort_string,full_sort_numeric,group_sum,top50,wide_materialize,repeated_expression_32
```

Repeat with `-source table` for retained input and with `GOMAXPROCS=4 -workers 4`,
then reverse the variant/worker sequence for round two. The frozen baseline
binary was built before the change at `9ee0f74`; rebuilding it from the current
working tree would invalidate the comparison. Generated fixtures and frozen
binaries persist in ignored `testdata/`. Run timing suites separately from
correctness suites and CPU profiles.

```bash
testdata/string-prefix/peg-workloads \
  -manifest testdata/amd64-baseline/fixtures/balanced-1048576/manifest.json \
  -workers 1 -warmup 3 -runs 10 -only full_sort_string \
  -cpuprofile testdata/string-prefix/sort-after.cpu
pprof -top testdata/string-prefix/peg-workloads testdata/string-prefix/sort-after.cpu
```

Profile sampling excludes setup, warmups and fingerprinting. Use unprofiled
paired medians for speedups. The cache adds up to 16 bytes per active row while
sorting, can be declined under the query budget, and releases before gathering.
These string-prefix reports contain no Linux process-memory measurements.
Current Linux measurement support is described above; native arm64 execution
remains unverified.

## Batched joins, parallel operators, and runtime filtering

The five-step follow-on adds bounded typed join output and batched residuals,
immutable shared join build/private parallel probing, stable parallel compact
sorts, eligible partitioned DISTINCT reductions, and conservative runtime
integer Parquet filters. See [HANDOFF.md](../HANDOFF.md) for exact eligibility.

- [Paired medians and limitations](benchmark-results/2026-10-02-library-efficiency-summary.txt)
- [Ranges, per-round medians, allocator snapshots, and validation metadata](benchmark-results/2026-10-02-library-efficiency-summary.json)
- [Raw samples, commands, fixture/source/binary hashes](benchmark-results/2026-10-02-library-efficiency-paired.json)
- [Sort, DISTINCT, and pruning microbenchmarks](benchmark-results/2026-10-02-library-efficiency-micro.txt)
- [All 37 workloads against DuckDB and Polars](benchmark-results/2026-10-02-library-efficiency-smoke.json)
- [Full native validation and arm64 compilation logs](benchmark-results/2026-10-02-library-efficiency-validation.txt)

The before runner is frozen at `testdata/string-prefix/peg-workloads`, so gains
are incremental to normalized string prefixes. Do not rebuild that runner from
the current sources. The after runner is `testdata/library-efficiency/peg-workloads`.
Both use `CGO_ENABLED=0 GOEXPERIMENT= GOAMD64=v1`; candidate source and binary
hashes are preserved in the report. Five checksummed Parquet cases and one
retained million-row table each run fourteen queries at one/four workers, with
two rounds, two warmups, and five samples per round (3,360 timed samples).
Reverse worker and variant order in round two; execute variants sequentially.

```bash
CGO_ENABLED=0 GOEXPERIMENT= GOAMD64=v1 go build \
  -o testdata/library-efficiency/peg-workloads ./scripts/workload_peg
GOMAXPROCS=4 testdata/library-efficiency/peg-workloads \
  -manifest testdata/amd64-baseline/fixtures/balanced-1048576/manifest.json \
  -source parquet -workers 4 -warmup 2 -runs 5 \
  -only join_int_report,join_string_report,join_left_report,join_semi_count,join_anti_count,grouped_self_join,full_sort_numeric,full_sort_string,distinct_strings,count_distinct,multi_group_report,group_sum,top50,wide_materialize
```

Repeat with the frozen before runner, `GOMAXPROCS=1 -workers 1`, `-source table`,
and the four other fixture manifests. Source loading, preparation, compilation,
and result hashing are excluded from timing; owned results release after each
query. Reports include controls and any slower cases, not only improvements.

Run the full cross-engine smoke matrix separately; its single sample verifies
correctness and is not used to rank timings:

```bash
.venv/bin/python scripts/compare_workload_engines.py \
  --scratch testdata/amd64-baseline/fixtures \
  --cases balanced:512,balanced:65536,balanced:1048576,skewed_nulls:65536,plain_strings:65536 \
  --threads 1,4 --rounds 1 --runs 1 --warmup 1 \
  --binary "$PWD/testdata/library-efficiency/peg-workloads" --skip-build \
  --json /tmp/peg-library-smoke.json
GOMAXPROCS=8 go test ./pkg/plan -run '^$' \
  -bench '^BenchmarkParallelCompactSort/rows=1048576/' -benchtime=3x -count=2
GOMAXPROCS=8 go test ./pkg/plan -run '^$' \
  -bench '^BenchmarkParallelDistinct$' -benchtime=3x -count=2
GOMAXPROCS=8 go test ./pkg/io/parquet -run '^$' \
  -bench '^BenchmarkRuntimePruningParquet$' -benchtime=10x -count=2
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -vet=off \
  -exec=/usr/bin/true ./pkg/plan ./pkg/peg ./pkg/io/parquet
```

The last command verifies compilation only; existing assembly debt requires
vet suppression. It does not establish native arm64 correctness or performance.
Microbenchmark Go heap allocations, live requested allocator bytes, retained
slab capacity, and process memory are distinct metrics. Runtime-pruning counters
report decoded rows and unopened groups rejected. The standard balanced timing
fixture has overlapping group statistics, so its Top50 timing is not a pruning
speedup demonstration; dedicated row-group fixtures prove avoided IO.

## Expanded parallelism, selective materialization, and spilling

The next efficiency round builds on the committed `48c9640` implementation.
Its candidate runner is `testdata/efficiency-round2/peg-workloads`; the frozen
before runner remains `testdata/library-efficiency/peg-workloads`. Rebuilding
that before runner from current sources would invalidate the comparison.

The changes broaden DISTINCT aggregation and parallel pipelines, avoid copying
rejected flat-string scan payloads, skip exact short-string sort collisions,
and add opt-in execution through temporary files. Compact hash controls remain
a private experiment enabled only by tests and benchmarks. Scalar and SIMD hit
cases regressed; SIMD gains on misses did not justify enabling controls by
default.
Library spilling is enabled with `SpillDirectory` in `peg.EngineOptions` or
`plan.ExecutionOptions`; the workload runner's standard timing examples below
measure its default in-memory path. See [HANDOFF.md](../HANDOFF.md) for supported
spill shapes, budget limits, cleanup behavior, and remaining fallbacks.

Round-two reports use `benchmark-results/2026-10-02-efficiency-round2-*` names;
raw JSON records preserve commands, samples, fingerprints, and source/binary
hashes. Build the candidate and run the paired comparison:

```bash
mkdir -p testdata/efficiency-round2
CGO_ENABLED=0 GOEXPERIMENT= GOAMD64=v1 go build \
  -o testdata/efficiency-round2/peg-workloads ./scripts/workload_peg
.venv/bin/python scripts/compare_peg_variants.py \
  --before testdata/library-efficiency/peg-workloads \
  --after testdata/efficiency-round2/peg-workloads \
  --scratch testdata/amd64-baseline/fixtures --table-million \
  --json scripts/benchmark-results/2026-10-02-efficiency-round2-paired.json
```

Reproduce a focused paired timing subset sequentially:

```bash
GOMAXPROCS=4 testdata/library-efficiency/peg-workloads \
  -manifest testdata/amd64-baseline/fixtures/balanced-1048576/manifest.json \
  -source parquet -workers 4 -warmup 2 -runs 5 \
  -only count_distinct,distinct_strings,multi_group_report,join_int_report,join_string_report,full_sort_string,full_sort_numeric,wide_materialize
GOMAXPROCS=4 testdata/efficiency-round2/peg-workloads \
  -manifest testdata/amd64-baseline/fixtures/balanced-1048576/manifest.json \
  -source parquet -workers 4 -warmup 2 -runs 5 \
  -only count_distinct,distinct_strings,multi_group_report,join_int_report,join_string_report,full_sort_string,full_sort_numeric,wide_materialize
```

Repeat at `GOMAXPROCS=1 -workers 1`, with `-source table`, and with the other
checksummed fixture manifests. Reverse variant and worker order for round two.
Use the complete commands recorded in the paired JSON to reproduce its exact
query matrix. Build the candidate before measuring; isolate timings from CPU
profiles, correctness suites, and process-memory sampling.

After installing the optional comparator, validate all 37 workloads across all
four engines. This one-sample smoke command checks result agreement:

```bash
.venv/bin/python scripts/compare_workload_engines.py \
  --scratch testdata/amd64-baseline/fixtures \
  --cases balanced:512,balanced:65536,balanced:1048576,skewed_nulls:65536,plain_strings:65536 \
  --threads 1,4 --rounds 1 --runs 1 --warmup 1 \
  --engines peGosus,DuckDB,Polars,DataFusion \
  --binary "$PWD/testdata/efficiency-round2/peg-workloads" --skip-build \
  --json scripts/benchmark-results/2026-10-02-efficiency-round2-smoke.json
```

Measure Linux process memory separately for bounded joins and wide owned output:

```bash
.venv/bin/python scripts/compare_workload_engines.py \
  --scratch testdata/amd64-baseline/fixtures --cases balanced:65536 \
  --threads 1,4 --rounds 2 --runs 5 --warmup 2 --retain 2 \
  --only wide_materialize,join_int_report --memory \
  --engines peGosus,DuckDB,Polars,DataFusion \
  --binary "$PWD/testdata/efficiency-round2/peg-workloads" --skip-build \
  --json scripts/benchmark-results/2026-10-02-efficiency-round2-memory.json
```

Focused benchmarks isolate copied scan payload bytes and hash metadata probing:

```bash
GOMAXPROCS=1 go test ./pkg/io/parquet -run '^$' \
  -bench '^BenchmarkParquetSelectedMaterialization$' -benchtime=100x -count=2
GOMAXPROCS=1 go test ./pkg/plan -run '^$' \
  -bench '^BenchmarkKeyIndexControls$' -benchtime=100ms -count=3
GOEXPERIMENT=simd GOMAXPROCS=1 go test ./pkg/plan -run '^$' \
  -bench '^BenchmarkKeyIndexControls$' -benchtime=100ms -count=3
```

The scan benchmark reports `copied-string-bytes/op` and `selected-rows/op`.
Selected scans retain the physical row domain and source validity; values outside
the row selection are unspecified. Low-level Parquet readers retain full
materialization by default. The intrinsic benchmark must be compared with the
normal build on the same machine and workload; feature dispatch and scalar
fallback remain required. Small smoke and microbenchmark results do not replace
the paired end-to-end timings.

## DISTINCT/spill memory follow-on

The final follow-on runner is
`testdata/efficiency-round3-final/peg-owned`; its frozen baseline is the
round-two runner `testdata/efficiency-round2/peg-workloads`. The reuse-only
intermediate binary remains `testdata/efficiency-round3/peg-workloads`, with
reports named `2026-10-02-efficiency-round3-reuse-*`. The key-only intermediate
runner remains `testdata/efficiency-round3-final/peg-workloads`, with reports
named `2026-10-02-efficiency-round3-keyonly-*`. Do not rebuild a historical
runner from current sources. The final summary records binary hashes, complete
before/after source hashes, commands, allocator/process memory distinctions and
validation results.

Build the candidate and reproduce the seven-query paired matrix separately from
correctness suites, profiles and process-memory sampling:

```bash
CGO_ENABLED=0 GOEXPERIMENT= GOAMD64=v1 go build \
  -o testdata/efficiency-round3-final/peg-owned ./scripts/workload_peg
.venv/bin/python scripts/compare_peg_variants.py \
  --before testdata/efficiency-round2/peg-workloads \
  --after testdata/efficiency-round3-final/peg-owned \
  --scratch testdata/amd64-baseline/fixtures --table-million \
  --only count_distinct,distinct_strings,multi_group_report,group_sum,full_sort_string,join_int_report,wide_materialize \
  --json /tmp/peg-round3-paired.json
```

The frozen plan test binaries isolate external sort/group/join workloads. Repeat
before/after, then reverse their order, with `GOMAXPROCS=8`:

```bash
CGO_ENABLED=0 GOEXPERIMENT= GOAMD64=v1 go test -c \
  -o testdata/efficiency-round3-final/plan-owned.test ./pkg/plan
GOMAXPROCS=8 testdata/efficiency-round3/plan-before.test -test.run '^$' \
  -test.bench '^Benchmark(SpilledGroup|SpilledSort|SpillJoinHotKey)$' \
  -test.benchmem -test.benchtime 3x -test.count 2
GOMAXPROCS=8 testdata/efficiency-round3-final/plan-owned.test -test.run '^$' \
  -test.bench '^Benchmark(SpilledGroup|SpilledSort|SpillJoinHotKey)$' \
  -test.benchmem -test.benchtime 3x -test.count 2
```

`B/op` and `allocs/op` measure cumulative Go allocation; `scope_peak_B` and
`peak-live-bytes` measure requested live segments. Reusable spill frames can
slightly increase live peaks while sharply reducing cumulative allocation.
Oversized records transfer frame ownership instead of failing the smaller batch
allowance. Key-only COUNT/MIN/MAX lanes retain encoded keys and indexes; any shared SUM/AVG
lane retains representative values. Standalone DISTINCT keeps full rows.

Run fresh-process RSS checks separately, for each binary, then reverse variant
and worker order for round two. Memory-run times are not benchmark speed claims:

```bash
.venv/bin/python scripts/compare_workload_engines.py \
  --scratch testdata/amd64-baseline/fixtures --cases balanced:1048576 \
  --threads 1,4 --rounds 1 --runs 3 --warmup 2 --retain 2 \
  --only count_distinct,full_sort_string --memory --engines peGosus \
  --binary "$PWD/testdata/efficiency-round3-final/peg-owned" --skip-build \
  --json /tmp/peg-round3-memory-after.json
```

Require exact before/after fingerprints when comparing these single-engine memory
reports. Linux samples VmRSS every 5 ms; interval peaks are lower bounds, while
`smaps_rollup` gives exact endpoint RSS/PSS. Neither RSS nor slab capacity is the
query's requested-byte budget.

The final four-engine correctness smoke covers all 37 workloads over small,
million-row and skewed/null Parquet fixtures. These one-sample timings are not
used to rank engines:

```bash
.venv/bin/python scripts/compare_workload_engines.py \
  --scratch testdata/amd64-baseline/fixtures \
  --cases balanced:512,balanced:1048576,skewed_nulls:65536 \
  --threads 1,4 --rounds 1 --runs 1 --warmup 0 \
  --engines peGosus,DuckDB,Polars,DataFusion \
  --binary "$PWD/testdata/efficiency-round3-final/peg-owned" --skip-build \
  --json /tmp/peg-round3-smoke.json
```

## Focused four-engine timing and memory comparison

The [October 2 comparison](benchmark-results/2026-10-02-cross-engine-followon-summary.txt)
uses the final third-pass binary, normal pure-Go amd64 with `GOAMD64=v1`. It has
6,440 timing samples, 112 fresh-process memory records and three separate CPU
profiles. Successful outputs have zero mismatches; 32 peGosus resource errors
reproduce across 16 configurations. Raw samples, per-round medians, release
timings, fixture checksums, exact RSS/PSS endpoints and all invocation arguments
are retained in the matching `summary.json` and `machine.json` reports. The
frozen binary and generated fixtures persist under ignored
`testdata/cross-engine-followon/`; preserve that binary when benchmarking later
sources. It exactly matches `testdata/efficiency-round3-final/peg-owned`.

For a new comparison, build a new runner and run the low-output core matrix:

```bash
CGO_ENABLED=0 GOAMD64=v1 GOEXPERIMENT= go build \
  -o /tmp/peg-cross-next ./scripts/workload_peg
.venv/bin/python scripts/compare_workload_engines.py \
  --scratch testdata/cross-engine-followon/fixtures \
  --binary /tmp/peg-cross-next --skip-build --source parquet \
  --cases balanced:1048576,skewed_nulls:1048576,plain_strings:1048576,balanced:10000000,skewed_nulls:10000000,plain_strings:10000000 \
  --only count_distinct,multi_group_report,group_sum,join_int_report,join_string_report \
  --threads 1,4,8 --rounds 2 --runs 5 --warmup 2 --peg-timeout 120 \
  --engines peGosus,DuckDB,Polars,DataFusion --json /tmp/peg-cross-next-core.json
```

The other timing matrices use the same worker/round/sample settings. Exact
commands are in the machine report:

| Matrix | Cases | Queries/source |
| --- | --- | --- |
| Sort/output and DISTINCT pipeline control | Three profiles at 1,048,576 rows; balanced at 10,000,000 | `full_sort_numeric,full_sort_string,distinct_strings`; Parquet |
| High-cardinality grouping | Same four cases | `high_card_int_group,high_card_string_group`; Parquet |
| Retained table | Balanced at 1,048,576 rows | Same five core queries; `--source table` |

Run memory separately from timing with `--memory --rounds 2 --runs 2 --warmup 1
--retain 1`. The million-row balanced matrix covers COUNT DISTINCT, grouped
report and string sorting at one/four/eight workers. Ten-million-row balanced
memory covers the grouped report at all counts and COUNT DISTINCT at four/eight;
the serial DISTINCT memory configuration is skipped because both timing rounds
fail under the budget. Each engine/query/worker/round starts in a fresh process.
Cold growth is measured above immediate pre-query RSS; ready/startup RSS is a
separate snapshot. Sampled peaks are lower bounds; exact endpoint RSS/PSS,
sample gaps and errors are preserved. Live peGosus payload returns to zero after
release, but unused slab capacity and process RSS can remain high.

This harness leaves peGosus spilling disabled and charges retained result payload
to its 1 GiB query budget. DuckDB and DataFusion may spill, Polars has no equivalent
configured hard cap, and output accounting differs. Failed configurations have no
ranked timing. DuckDB converts full outputs to Python rows inside timing; peGosus,
Polars and DataFusion return different native representations. Keep large-output
adapter results separate from low-output comparisons. Prepared peGosus plans are
excluded from timing; competitor planning/optimization costs differ as described
in the raw limitations. This is a warm-filesystem-cache synthetic suite, not an
official benchmark score or a universal engine ranking.

Diagnostic eight-worker CPU profiles for ten-million-row integer grouping,
COUNT DISTINCT and the grouped report ran only after all comparative timing and
memory stages. Their fingerprints match, and their timings are excluded from
comparison statistics. Text profiles are saved with the report; raw `.cpu` files
persist under ignored `testdata/cross-engine-followon/`. The group-then-reduce
query remains serial because the parallel executor accepts one aggregate boundary;
the profile confirms about one active core. `u64Index.get`, string/encoded-key
lookup, generic grouping and string materialization are measured hot paths.
