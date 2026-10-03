# Handoff: peGosus query engine

## Current state: October 2, 2026

The completed portability and first library-efficiency round is being committed
from base `9ee0f74`: string-prefix sorting, batched/parallel joins, parallel full
sorting, expanded DISTINCT execution, runtime Parquet filters, Linux setup,
research notes, and native amd64 reports. Existing local contract documents
remain outside this commit; `AGENTS.md` is ignored. The next efficiency round
starts after this checkpoint is pushed. Commit and push only on explicit request.

peGosus is a pure-Go vectorized query engine with allocator-backed vectors,
validity, row selections, operator state, and retained results. `pkg/plan`
binds immutable expressions to exact types and stable field IDs, then executes
push-based batches with query budgets and cooperative cancellation. `pkg/peg`
provides lazy queries over Parquet, explicitly typed CSV, and retained tables,
including grouping, distinct, sorting/TopN, equality joins, prepared execution,
and streaming sinks. CSV schema inference and spilling are not implemented.
Parquet's supported subset is defined in [PARQUET_SUPPORT.md](PARQUET_SUPPORT.md).

Recent work covers slab coalescing before growth, projected Parquet decode,
compact numeric/string grouping and dictionary propagation, buffer reuse,
parallel eligible numeric TopN, compact multi-key full sorting, CASE admission,
and identical global aggregate sharing. Eligible equality joins now gather
typed columns in bounded batches and probe immutable build state in parallel.
Full compact sorting uses stable local runs and parallel merges; standalone
DISTINCT and eligible DISTINCT reductions reuse partitioned grouping. Integer
TopN and inner/semi joins publish conservative runtime Parquet bounds. Parallel
eligibility still depends on the pipeline shape. Retained
wrappers share payload, validity, selections, and string backing; borrowed
result views expire on release unless independently retained or copied.

Read [AGENTS.md](AGENTS.md), [SEMANTICS.md](SEMANTICS.md), [OPS.md](OPS.md),
[KERNEL_ABI.md](KERNEL_ABI.md), and the Parquet contract before changing execution.
Architecture-specific `simd/archsimd` kernels are allowed behind experiment and
architecture constraints, with scalar fallbacks and supported-CPU dispatch.
New computation-kernel assembly requires an explicit request. Outstanding
kernel correctness debt is in [KERNELS_TODO.md](KERNELS_TODO.md).

## Next priorities

1. **Broader parallel pipelines:** support multiple joins and more blocking-stage
   combinations, then multiple/grouped DISTINCT aggregate expressions. Preserve
   local/merge/final ownership and stable ordered output; join-to-TopN currently
   falls back to serial execution.
2. **Sort merge costs:** retain normalized representations through merging and
   measure random/high-cardinality strings, long common prefixes, ordered input,
   and worker scaling. Collection and owned output gathering remain serial.
3. **Memory reuse and spilling:** investigate reusable slab-capacity policy,
   then spillable sort runs and partitioned grouping/join state. Query budgets
   constrain requested live bytes, not process footprint or idle slab capacity.
4. **Late payload materialization and Parquet strings:** extend runtime filtering
   beyond direct signed integer keys, retain encoded dictionaries where useful,
   and isolate decoding, copying, and allocator/OS-page costs before SIMD work.
5. **SIMD hash metadata and expression sharing:** measure compact control-byte
   probing before adding AVX2/NEON intrinsics; broaden exact subexpression reuse
   only with evidence and unchanged IEEE, null, and field-ID semantics.

Use the expanded 37-query harness to validate both affected workloads and
controls. These synthetic comparisons are not official ClickBench/TPC results;
DuckDB Python-row conversion and Polars column sharing affect large outputs.
Research references are in [PARQUET_SUPPORTING_LINKS.md](PARQUET_SUPPORTING_LINKS.md).
The current cross-architecture efficiency plan and primary research sources are
in [docs/RESEARCH_ROADMAP.md](docs/RESEARCH_ROADMAP.md), including dynamic scan
filters, SIMD hash metadata, and spillable state.
Older measurements and superseded diagnoses remain in the historical sections
below and their referenced reports.

## Library efficiency: batched joins, parallel operators, runtime filtering

The five requested implementation steps are complete within these eligibility
boundaries:

1. Equality joins retain immutable build batches and allocator-backed row
   references instead of repacking right-side payloads. Probing emits at most
   1,024 candidate pairs per batch, gathers typed output columns directly, and
   evaluates residual predicates over candidate batches. Inner, left, full,
   semi, and anti joins preserve duplicates, null/NaN matching rules, selections,
   owned strings, early sinks, and cooperative cancellation. Nested build
   execution shares the parent allocation scope, including retained computed
   vectors; fanout output stays bounded by the same query budget.
2. Eligible single-join pipelines build once, then share immutable columns/index
   across probe workers with private lookup scratch and full-join match bitmaps.
   Local match maps merge before unmatched build rows are emitted. Downstream
   ordinary global/grouped aggregation is supported. Multiple joins and
   join-to-TopN pipelines use the serial path; sources need multiple tasks for
   parallel probing. Sinks serialize worker output without promising unordered
   query input order.
3. Full compact sorting creates disjoint stable local radix/comparison runs and
   uses merge-path partitions ordered by key and original compact row ordinal.
   Already ordered adjacent runs copy directly. Primary string merges compare
   optional normalized prefix words before exact collision handling; this cache
   allocates after local caches are released. Parallel work begins at 32K rows, with at least 16K rows per worker; explicit worker caps respect
   GOMAXPROCS and automatic selection caps at eight. Collection and final owned
   gathering remain serial; the optional packed-sort path is unchanged.
4. Standalone compound DISTINCT reuses zero-accumulator grouping and existing
   partitioned merges. A single global DISTINCT count/sum/avg/min/max, including
   string extrema, reduces globally unique output. DISTINCT followed immediately
   by terminal ordinary global aggregates also reduces partition output directly,
   including multiple/reused aggregates and computed expressions. Multiple
   DISTINCT aggregates, grouped DISTINCT aggregates, and unsupported tails use
   serial execution.
5. Parquet readers recheck shared atomic signed-integer bounds before opening
   unread row-group column chunks. A full eligible numeric TopN heap publishes
   an inclusive primary-key threshold; direct integer inner/semi join keys
   publish build-side min/max. Filters handle projections and exact INT32-to-
   INT64 widening, preserve primary ties and nullable ordering, and require an
   active proof before pruning all-null groups. Outer/anti joins and unsupported
   computed/float keys do not publish filters. Already opened groups finish
   normally; static predicates and execution filtering still apply.

All changes are portable Go, add no dependencies or assembly, and preserve the
public query API. Reports use the prefix
`scripts/benchmark-results/2026-10-02-library-efficiency-`; the comparator is the
frozen string-prefix runner, so reported gains are incremental to that work.
See the summary, raw paired samples, microbenchmarks, and full cross-engine
smoke reports for measured effects and limitations.

The final paired matrix measures fourteen queries over five Parquet fixtures
and one retained million-row table, with one/four workers, two reversed-order
rounds, two warmups, and five samples per round: 672 observations / 3,360 timed
samples. All fingerprints match the prior DuckDB/Polars-validated references;
live allocator bytes return to zero. Million-row four-worker pooled medians:

| Source/query | Before, ms | After, ms | Speedup |
|---|---:|---:|---:|
| Parquet integer join | 173.136 | 27.438 | 6.31× |
| Parquet string join | 224.763 | 35.538 | 6.32× |
| Parquet grouped self join | 242.958 | 55.869 | 4.35× |
| Parquet numeric sort | 97.437 | 82.978 | 1.17× |
| Parquet string sort | 267.002 | 190.048 | 1.40× |
| Parquet string DISTINCT | 184.480 | 101.296 | 1.82× |
| Retained-table integer join | 155.753 | 21.183 | 7.35× |
| Retained-table string sort | 228.242 | 129.243 | 1.77× |
| Retained-table string DISTINCT | 138.931 | 87.684 | 1.58× |

One-worker joins improve about 1.7–2.4× on these fixtures. Ordinary grouping and
Top50 controls stay close; the two-DISTINCT-aggregate workload stays on its
serial fallback. One-worker Parquet string sorting is about 8.5% slower in this
mixed sequence. Some tiny controls are 10–25% slower, with differences below
0.03 ms. Raw ranges, per-round medians and all slower cases remain reported;
these desktop measurements are not a universal scaling claim.

Dedicated pruning microbenchmarks decode 2,048 instead of 4,097 rows and reject
two unopened groups, roughly halving measured reader time. Guarded-reader tests
prove no column-chunk IO occurs for those rejected groups. The standard balanced
fixture has overlapping statistics, so its Top50 timings do not demonstrate
pruning gains. Random-string direct sort microbenchmarks scale from about
151 ms at one worker to 118 ms at four and 112 ms at eight; these are stage
comparisons, separate from the frozen-runner query timings above.

Normal, race, aggressive checkptr, and SIMD versions of all three suites pass.
The full 37-query comparison across five fixtures, one/four workers, DuckDB,
Polars and peGosus passes 1,110 observations with zero failures or mismatches.
Tests include scalar-reference join output, multi-batch duplicate explosions,
residuals, retained ownership, immutable shared build payload, full-match merging,
sink termination, cancellation, tiny budgets, stable mixed-key sort ties,
DISTINCT null/NaN/signed-zero semantics, and zero live allocator bytes. Guarded
ReaderAt tests prove that rejected row groups cause no column-chunk IO. Native
arm64 execution remains untested; plan/API/Parquet cross-compilation passes with
vet disabled for existing assembly debt. Linux process memory is not measured.

## Portable string-prefix sorting: October 2, 2026

String full-sort keys now optionally cache a null class and 15 raw string bytes
in two uint64 words. Stable radix passes skip invariant bytes; full-string stable
merges resolve equal-prefix runs, including embedded NULs and long suffixes.
Small inputs and queries that cannot budget the cache use comparison sorting.
The cache costs 16 bytes per active row and is released before gathering; it
does not change query budget semantics. Source backing stays retained and output
remains owned. This is portable Go with no new dependencies, SIMD, or assembly.

The paired comparison uses frozen before/after pure-Go runners, five checksummed
Parquet fixtures and one retained million-row table, one/four workers, two
rounds, two warmups, and five samples per round. Worker/variant order reverses
in round two. All 288 query observations / 1,440 timed samples succeed and match
the prior DuckDB/Polars-validated fingerprints. Live allocator bytes return to
zero after every process. Pooled median milliseconds at 1,048,576 rows:

| Source | Worker cap | Before | After | Speedup |
|---|---:|---:|---:|---:|
| Parquet | 1 | 621.492 | 251.625 | 2.47× |
| Parquet | 4 | 642.612 | 239.820 | 2.68× |
| Retained table | 1 | 559.075 | 240.692 | 2.32× |
| Retained table | 4 | 605.026 | 207.431 | 2.92× |

Balanced and skewed/null-heavy 65K string sorts improve roughly 2×; PLAIN long
strings sharing the prefix change only about 2%. Numeric sort, group SUM, Top50,
wide materialization and repeated aggregates are controls. Four-worker Parquet
controls vary by up to roughly 6% in pooled medians, with overlapping ranges;
these are desktop measurements, not proof of universal regression-free behavior.
Retained-table one-worker string samples vary between rounds, so use the saved
ranges and per-round medians. Sort itself remains serial; worker caps affect
eligible upstream pipelines. No native arm64 timing claim or Linux process
footprint claim is made.

Normal, race, aggressive checkptr, and experimental SIMD full suites pass.
arm64 plan/API compilation passes with assembly vet disabled for existing kernel
debt; arm64 execution has not been tested on this PC. New differential tests
exercise radix thresholds, flat/dictionary strings, computed keys, binary/UTF-8
strings, nulls, selections, multi-batch stable ties, retained ownership, optional
cache denial, cancellation during radix work, and cleanup. Existing multi-key,
numeric edge-case, early-sink, merge-cancellation and budget tests also pass.
An additional full 37-query smoke comparison against DuckDB and Polars covers
tiny, balanced 65K, and long-string 65K fixtures at one/four workers: 666 query
observations, zero failures or mismatches.

Reports are prefixed `scripts/benchmark-results/2026-10-02-string-prefix-`:
`summary.txt`, `summary.json`, `paired.json`, `smoke.json`, `smoke.txt`, and
`profile-{before,after,join}.txt`. Paired JSON preserves exact commands, source,
fixture and binary hashes, samples and allocator snapshots. Frozen runners are
`testdata/amd64-baseline/peg-workloads` and `testdata/string-prefix/peg-workloads`.
Fixture and binary directories are ignored. See [scripts/README.md](scripts/README.md)
for machine setup and reproduction.

## Compact execution state and admission fixes: October 1–2, 2026

Committed and pushed to `origin/main`: `1151af7` (`perf: compact sort state and share repeated aggregates`) and `9ee0f74` (`add: expand cross-engine workload benchmarks`). The frozen pre-change package baseline is `be740cf`.

### Changes

- Admission now measures CASE nesting within each physical expression program, rather than counting CASE width across unrelated aggregates. Global aggregate programs contribute their maximum sequential requirement; grouped aggregate programs still contribute the sum of their simultaneously live requirements. Gathered CASE input/row indices and overflow-safe bounds remain included. Fallible query-scope allocation still enforces actual requested-byte limits. The 128-expression query now completes under the original 1 GiB budget on all five benchmark fixtures; tiny-budget rejection remains tested.
- Full compact sorting accepts multiple numeric/date/time/string keys and variable-width output columns. Allocator-backed row references and normalized numeric key columns replace per-row Scalar/key/text slices. Stable radix passes handle numeric keys; stable merge passes handle string keys, working from the final key to the first. Null placement, descending order, NaNs, signed-zero ties, selections, and stable input order are preserved. Retained input batches and computed string-key vectors keep backing alive; output strings are copied once into owned contiguous backing. Generic TopN behavior and optional packed-key sorting remain available. Full sorting still executes serially; SIMD and parallel run/merge kernels are future work.
- Identical global aggregates share one accumulated result, including identical DISTINCT aggregates on the serial path. Equality includes aggregate kind, DISTINCT, exact physical nodes/types/literal bits, roots, materialization, and specialization flags. Output aliases/schema field IDs remain intact. Duplicate slots stay empty during local accumulation and worker merging; finalization reads the canonical slot without duplicating text ownership. Grouped aggregate sharing and common subexpressions across different aggregate kinds are not implemented.
- Compact sort checks cancellation during radix/merge work; serial and aggregate-tail execution return cancellation when interrupted finalization yields no batch. Failure cleanup releases partially allocated state.
- The benchmark runner supports `-cpuprofile` for one query, excluding setup, warmups, and result hashing. The Parquet column diagnostic now includes every wide-projection column and the combined projection.

### Benchmarks

The same immutable/checksummed fixtures, canonical fingerprints, budgets, native output adapters, warm-cache policy, and one/four-worker caps from the expanded suite are used. Main affected-query matrix: seven queries × five fixtures × two worker counts × two rounds × three engines, 60 worker records / 420 observations, with two warmups and five timed samples per round. Retained-table comparison: seven queries at one million rows, 12 records / 84 observations. Fresh-process memory: four queries at one million rows / four workers / two rounds / three engines, 24 records. Additional full 37-query smoke checks cover tiny, balanced 65K, and PLAIN/long-string 65K inputs at one/four workers, 18 records / 666 observations; these one-sample smoke timings are not used for performance claims.

**Every new suite has zero query failures and zero result mismatches.** The affected matrix includes the formerly rejected 128-expression shape and formerly cancelled full-string sorts. Initial tiny-case matrix samples overlapped a short normal-test run; the larger timings, retained-table, paired, memory, and smoke runs were separated from correctness suites. Results remain synthetic; DuckDB Python-row conversion and Polars column sharing still prevent treating large-output adapter timings as pure engine rankings.

One million rows, four workers, cross-engine pooled median execution milliseconds:

| Query/source | peGosus | DuckDB | Polars |
|---|---:|---:|---:|
| Numeric sort, Parquet | 80.613 | 106.017 | 47.770 |
| String sort, Parquet | 569.650 | 186.483 | 80.498 |
| 32 repeated aggregates, Parquet | 9.861 | 4.045 | 4.442 |
| Numeric sort, retained table | 69.805 | 102.757 | 46.256 |
| 32 repeated aggregates, retained table | 1.667 | 2.183 | 2.841 |

A separate frozen-runner comparison alternates variant and worker order over two rounds, with one warmup and three samples per query/round. Queries run in the same explicit order: numeric sort, string sort, repeated aggregates, wide projection, group SUM, Top50. Eight old string sorts and two old one-worker Parquet repeated-aggregate runs cancel at the guard deadline; these failures remain recorded and have no ranked timing. Successful outputs match the new cross-engine references. Four-worker pooled medians:

| Query/source | Before → after, ms |
|---|---:|
| Numeric sort, Parquet | 1704.942 → 83.755 |
| Numeric sort, retained table | 1668.390 → 69.432 |
| Repeated aggregates, retained table | 38.027 → 1.714 |
| Wide projection after sorts, Parquet | 1756.951 → 60.054 |
| Wide projection after sorts, retained table | 1.348 → 1.352 |

Numeric sorting improves about 20× on Parquet and 24× on loaded input in this paired sequence; repeated global aggregates improve about 22× on loaded input. Group SUM and Top50 controls stay close to the baseline. Native full-string sorting remains faster than peGosus, so representation removal is a foundation rather than the end of sorting work.

Fresh-process warm immediate-release physical-footprint interval peaks, MiB, one million rows / four workers:

| Query | peGosus | DuckDB | Polars |
|---|---:|---:|---:|
| Numeric sort | 140.42 | 161.88 | 165.69 |
| String sort | 154.06 | 204.95 | 202.16 |
| Repeated aggregates | 15.47 | 30.47 | 152.52 |
| Wide projection | 210.38 | 520.96 | 193.31 |

The freshly rerun peGosus numeric-sort baseline measures **872.10 → 140.42 MiB**, about an 84% peak reduction. Its post-GC reusable allocator capacity grows **41.90 → 169.06 MiB** because bulk sort state moved into reusable slabs. These are distinct metrics: lower execution peak does not imply lower idle retained capacity. Every new peGosus release/steady/final-GC snapshot returns live general/scratch payload to zero. Memory budgets remain requested-byte limits, not process-footprint caps.

### Corrected wide-scan diagnosis

The earlier ~966 ms wide-materialization matrix result was strongly affected by preceding generic sort allocation/reuse. In isolated fresh query processes, the frozen baseline and new binary measure roughly **50.1 and 44.1 ms** respectively at four workers. The paired mixed sequence above shows a much larger difference after old string-sort cancellation. No production Parquet decoder code changed in this pass; do not describe that mixed-sequence improvement as a faster decoder or assign the old full gap to SIMD.

One-worker direct-reader diagnostics on the same million-row fixture, releasing each batch immediately, measure the wide projection around 110–112 ms from a cached file and 109–110 ms from an in-memory ReaderAt, with 419 reads / 17,143,759 requested bytes per scan. Individual string columns take roughly 12–25 ms; numeric score is ~7.6 ms and non-null id ~0.7 ms. This points toward bulk string materialization/dictionary propagation as a useful scan investigation. These diagnostics include footer parsing and have different retention from native full-result materialization; their early samples overlapped a separate one-worker profile, so treat fine timing differences cautiously.

A separate, isolated public-query CPU profile uses five warmups followed by twenty profiled executions at one worker, excluding setup/warmup/hashing. It records substantial `runtime.madvise` and page-read syscall samples, plus string materialization, level/index decoding, and allocator work. The direct-reader file/memory contrast and the profile measure different ownership patterns; the syscall samples are not evidence that disk bandwidth alone limits the query. Further allocator/OS-page and encoded-string isolation should precede a specific parser-kernel speedup claim.

### Validation

Full normal, race, and aggressive checkptr suites pass. amd64 cross-compilation and arm64 SIMD-enabled parser/plan/API checks pass. Six Python query/fixture/fingerprint/process-protocol tests pass. Final changed-package normal/race/checkptr checks cover the strengthened all-tie sort test and profiling/diagnostic changes. Tests cover random multi-batch sorts, bitmap/selection-vector masks, dictionary and flat strings, computed keys, long/binary/UTF-8 strings, nulls/all-null/empty inputs, NaN/infinities/signed zero, owned output after source release, partial allocation failure, cancellation inside merging, early sink termination, exact aggregate identity, duplicate string/DISTINCT ownership, and local-state merging.

### Artifacts and reproduction

All reports are under `scripts/benchmark-results/`, prefixed `2026-10-02-compact-execution-`: `summary.json`, `workloads.json`, `table.json`, `memory.json`, `smoke.json`, `paired.json`, `column-scans.txt`, `wide-profile.json`, and `wide-profile.txt`. Summary includes medians, baseline failures, binary/artifact/source hashes, process peaks above startup, held-result peaks, reusable capacities, and limitations. Frozen runners are `/tmp/peg-execution-before` and `/tmp/peg-execution-after`; the final profiling runner is `/tmp/peg-execution-profile`. Later runner-only profiling/argument validation edits preserve ordinary query/timing behavior; raw reports retain historical helper source hashes. Raw CPU profile: `/tmp/peg-wide-steady.cpu`.

```bash
PYTHONDONTWRITEBYTECODE=1 /tmp/peg-perf-env/bin/python scripts/compare_workload_engines.py --scratch /tmp/peg-expanded-workloads-v2 --cases balanced:512,balanced:65536,balanced:1048576,skewed_nulls:65536,plain_strings:65536 --only full_sort_numeric,full_sort_string,repeated_expression_32,expression_width_128,wide_materialize,group_sum,top50 --json /tmp/compact-workloads.json
PYTHONDONTWRITEBYTECODE=1 /tmp/peg-perf-env/bin/python scripts/compare_workload_engines.py --scratch /tmp/peg-expanded-workloads-v2 --cases balanced:1048576 --source table --only full_sort_numeric,full_sort_string,repeated_expression_32,expression_width_128,wide_materialize,group_sum,top50 --json /tmp/compact-table.json
PYTHONDONTWRITEBYTECODE=1 /tmp/peg-perf-env/bin/python scripts/compare_workload_engines.py --scratch /tmp/peg-expanded-workloads-v2 --cases balanced:1048576 --threads 4 --runs 3 --warmup 1 --memory --only full_sort_numeric,full_sort_string,repeated_expression_32,wide_materialize --json /tmp/compact-memory.json
PEG_BENCH_PARQUET=/tmp/peg-expanded-workloads-v2/balanced-1048576/facts.parquet GOMAXPROCS=1 go test ./pkg/io/parquet -run '^$' -bench '^BenchmarkParquetColumnDiagnostic$' -benchtime=200ms -count=2
```
