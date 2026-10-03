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
```

Generate external fixtures before testing so external IO tests run instead of
skipping. The Python memory-counter tests are macOS-specific and skip on Linux.

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

`compare_parquet_memory.py` and the workload harness's `--memory` mode require
macOS libproc physical-footprint counters. Installing packages cannot supply
those counters on Linux; these modes need an explicit Linux implementation
with clearly identified metrics before they can be used here.

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
Linux process-memory measurement and native arm64 execution remain separate work.

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
