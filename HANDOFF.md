# Handoff: peGosus query engine

## Current state: October 2, 2026

Checkpoint `48c9640` was committed and pushed to `origin/main` after the user's
explicit request. It contains the portability setup and first library-efficiency
round from base `9ee0f74`, including their native amd64 reports. The second
five-step efficiency round and a follow-on DISTINCT/spill memory pass are
implemented locally and remain uncommitted.
Existing local contract documents remain outside the checkpoint; `AGENTS.md`
is ignored. Commit and push only on explicit request.

peGosus is a pure-Go vectorized query engine with allocator-backed vectors,
validity, row selections, operator state, and retained results. `pkg/plan`
binds immutable expressions to exact types and stable field IDs, then executes
push-based batches with query budgets and cooperative cancellation. `pkg/peg`
provides lazy queries over Parquet, explicitly typed CSV, and retained tables,
including grouping, distinct, sorting/TopN, equality joins, prepared execution,
and streaming sinks. CSV schema inference remains unimplemented. Eligible
blocking operators now support opt-in temporary-file execution through
`ExecutionOptions.SpillDirectory`; the public `peg` execution options forward it.
Parquet's supported subset is defined in [PARQUET_SUPPORT.md](PARQUET_SUPPORT.md).

Recent work covers compact execution state, exact aggregate sharing, bounded
columnar join output, stable parallel sorting, and integer runtime Parquet
filters. The second round expands multiple/grouped DISTINCT reductions and
parallel join pipelines, reduces selected Parquet payload copying, adds an
experimental fingerprint-probing implementation, and implements bounded external
sorting, aggregation/DISTINCT, and eligible single joins. Default hash probing
stays on the existing path pending workload evidence; SIMD availability does not
imply every operator uses SIMD. A third pass reuses DISTINCT merge sets, removes
representative payload from eligible COUNT/MIN/MAX DISTINCT lanes, and reuses
borrowed spill records while scans return bounded owned batches. Exact
eligibility is recorded below.

Retained wrappers share payload, validity, selections, and string backing;
borrowed result views expire on release unless independently retained or copied.
Query scopes bound requested live segment bytes, excluding borrowed sources,
Go metadata, and unused slab capacity. They do not bound RSS. Spilling does not
make an individual row, decoded page, or fully retained `peg.Result` fit a smaller
budget; use a streaming sink when the complete retained result exceeds it.

Read [AGENTS.md](AGENTS.md), [SEMANTICS.md](SEMANTICS.md), [OPS.md](OPS.md),
[KERNEL_ABI.md](KERNEL_ABI.md), and the Parquet contract before changing execution.
Architecture-specific `simd/archsimd` kernels are allowed behind experiment and
architecture constraints, with scalar fallbacks and supported-CPU dispatch.
New computation-kernel assembly requires an explicit request. Outstanding
kernel correctness debt is in [KERNELS_TODO.md](KERNELS_TODO.md).

## Next priorities

1. **Parallelize multiple aggregate boundaries.** Grouped aggregation followed
   by a global reduction currently falls back to serial execution. The new
   high-cardinality grouping comparisons barely scale from one to eight workers.
   Preserve exact types, nulls, selections, ownership, cancellation and existing
   floating reduction semantics while separating local, merge and final stages.
2. **Improve hash lookup, DISTINCT merging and string state.** Integer grouping
   profiles spend 65% flat CPU in `u64Index.get`; DISTINCT spends 48% in string
   and encoded-key lookup. Measure dense integer grouping and alternative index
   layouts, reduce string-group state and merge work, and account for idle
   allocator capacity. Fingerprint controls remain experimental until measured
   across hit/miss mix, key width, load factor and end-to-end workloads.
3. **Reduce grouping interpretation and scan work.** Generic group accumulation,
   row-key encoding and selected string materialization remain substantial CPU
   costs. Measure typed multi-key grouping, dictionary preservation and exact
   expression sharing. Broaden runtime filters only with exact null/type semantics.
4. **Improve join-to-aggregate and blocking composition.** Investigate factorized
   execution with explicit residual/outer/floating eligibility. Measure spill
   blocks, IO, merge fan-in, wide rows and skew; extend spilled joins to sort/limit,
   multiple joins and additional blocking combinations. Required rows/pages and
   retained results must still fit their budgets; measure streaming sinks separately.
5. **Broaden coverage beyond synthetic amd64 results.** Include real datasets,
   explicit spill comparisons and budget sweeps, long-prefix/stable-tie sorting,
   ordered inputs and allocator reuse. Native arm64 runtime/timing remains necessary;
   cross-compilation does not establish architecture performance.

Use the expanded 37-query harness to validate both affected workloads and
controls. These synthetic comparisons are not official ClickBench/TPC results;
DuckDB Python-row conversion and Polars column sharing affect large outputs.
Research references are in [PARQUET_SUPPORTING_LINKS.md](PARQUET_SUPPORTING_LINKS.md).
The current cross-architecture efficiency plan and primary research sources are
in [docs/RESEARCH_ROADMAP.md](docs/RESEARCH_ROADMAP.md), including dynamic scan
filters, SIMD hash metadata, and spillable state.
Older measurements and superseded diagnoses remain in the historical sections
below and their referenced reports.

## Focused four-engine comparison

The October 2 comparison follows the DISTINCT/spill pass without changing library
code. The normal pure-Go `GOAMD64=v1` binary exactly matches the tested final
candidate. Full source/binary hashes, fixture checksums, reproduction commands,
raw samples, round medians and memory endpoints are in
[the comparison report](scripts/benchmark-results/2026-10-02-cross-engine-followon-summary.json)
and its [readable summary](scripts/benchmark-results/2026-10-02-cross-engine-followon-summary.txt).
Competitors are DuckDB 1.5.6, Polars 1.34.0 and DataFusion 54.0.0 / PyArrow 25.0.1.

Five low-output queries cover balanced, skewed/null-heavy and PLAIN long-string
Parquet inputs at 1,048,576 and 10,000,000 rows, one/four/eight workers, two
reversed-order rounds, two warmups and five timed samples per round. Separate
matrices measure full numeric/string sorting and DISTINCT-then-count, retained
million-row tables, and high-cardinality integer/string grouping. There are
360 timing records, 6,440 retained successful timing samples and 112 separate
fresh-process memory records. All 1,400 successful observations match; 32 peGosus
resource errors reproduce across 16 configurations in both rounds. No competitor
query fails. These are synthetic desktop results, not universal engine rankings.

Balanced ten-million-row Parquet medians at eight workers, milliseconds:

| Query | peGosus | DuckDB | Polars | DataFusion |
| --- | ---: | ---: | ---: | ---: |
| Multiple COUNT DISTINCT | 1,315.376 | 117.956 | 682.329 | 635.309 |
| Grouped report | 226.739 | 43.140 | 190.169 | 37.891 |
| Group SUM | 24.109 | 7.657 | 56.847 | 13.840 |
| Integer join report | 143.127 | 17.253 | 118.352 | 20.761 |
| String join report | 189.061 | 47.587 | 167.246 | 26.806 |
| High-cardinality integer grouping | 1,142.235 | 111.436 | 406.671 | 167.998 |

Ordinary aggregation and joins improve 6.63–7.16x within peGosus from one to eight
workers, while multiple COUNT DISTINCT improves only 1.09x from four to eight.
High-cardinality integer grouping is 1,122.073 ms with one worker and 1,142.235 ms
with eight: `execution_parallel.go` rejects a second aggregate boundary, so this
group-then-reduce shape remains serial. Three separate five-run CPU profiles
confirm about one active core for that query and 65.25% flat CPU in `u64Index.get`.
COUNT DISTINCT has 47.58% flat CPU in string/encoded-key lookup and 29.39%
cumulative CPU in `mergeDistinctSet`; those costs overlap. The grouped report
uses about 7.83 cores, with 61.86% cumulative CPU in `groupState.add` and 21.06%
in selected string materialization. Profile timings are excluded from the matrix;
their fingerprints also match. Text profiles persist with the report; raw profiles
are under ignored `testdata/cross-engine-followon/`.

Serial balanced ten-million-row COUNT DISTINCT, long-string COUNT DISTINCT at
all worker counts, high-cardinality ten-million-row string grouping, and the
ten-million-row full-sort/explicit-DISTINCT controls exceed peGosus's 1 GiB budget.
Spilling is disabled in this harness. DuckDB and DataFusion may spill; Polars has
no equivalent configured cap, and output accounting differs. Failed configurations
have no ranked timing. DuckDB's full-sort timings include Python row conversion,
while the other adapters retain their native output, so they measure different
materialization costs. At one million balanced rows/eight workers, peGosus numeric
and string sorts are 79.622/148.527 ms versus Polars 35.542/51.154 and DataFusion
31.534/67.514; long-string sorting grows to 315.510 ms in peGosus versus
75.064/88.568 ms in Polars/DataFusion.

Memory varies by query. At ten million balanced rows/eight workers, COUNT DISTINCT
cold sampled RSS is 2,486.6 MiB for peGosus, 611.2 for DuckDB, 1,070.5 for Polars
and 1,461.3 for DataFusion. The grouped report uses 38.2/85.8/1,696.2/248.3 MiB,
respectively. peGosus live general/scratch payload returns to zero after release
in every memory record; idle allocator capacity and process RSS remain separate.
Samples are lower bounds, with a maximum observed gap of 7.548 ms and no sampling
errors. Exact endpoint RSS/PSS and stage baselines are retained. Cold refers to a
fresh query process on a warm filesystem cache. No code changes, commits or pushes
were made for this comparison.

## DISTINCT and spill memory follow-on

Global DISTINCT merging transfers a worker seed instead of allocating another
complete set, detaches expression lanes before concurrent merging, releases
consumed local indexes/payload and drops final deduplication state before emitting
aggregate scalars. Floating SUM/AVG lanes keep the first populated seed and the
existing worker merge order; COUNT/integer SUM lanes may use the largest seed.
Grouped overlapping rows release consumed state immediately.

A shared DISTINCT expression lane used only by COUNT/MIN/MAX retains encoded
keys, references and its hash index without representative packed rows. MIN/MAX
still owns and merges its separate exact aggregate values, including signed-zero
preferences. Any shared SUM/AVG expression keeps representative payload; group
keys and standalone DISTINCT retain full rows. Full/key-only mixed internal
merges preserve the requested representation or reject unavailable representatives.

Spill streams have a reusable allocator-backed frame and column offsets. Borrowed
record values expire on advance or release. Sort heads and aggregate lane joins
consume these records directly; spill scans copy them into bounded owned batches
before advancing. Deduplication retains only the group key and unique value, and
reuses their packed storage. Retained scan output stays valid across frame reuse,
file boundaries, cursor close and input release. Existing owned `readRow` remains
available for consumers that retain an individual record. An oversized first
record can transfer its validated frame into an owned row rather than exceeding
the smaller batching allowance; frame bytes stay charged until result release.

Budget failures from spill-backed scans report `ExecutionResourceExhausted` and
plain cancellation reports `ExecutionCancelled`. Empty global reductions,
zero-column row domains, malformed/truncated frames, geometric buffer growth and
large budgets have regression coverage. Buffer caches remain charged until
release; lower cumulative allocation does not imply lower live segment peaks.

The follow-on measurements and validation record are in
[`2026-10-02-efficiency-round3-summary.json`](scripts/benchmark-results/2026-10-02-efficiency-round3-summary.json)
and its [readable summary](scripts/benchmark-results/2026-10-02-efficiency-round3-summary.txt).
The frozen baseline is the completed second-round runner; the final candidate is
`testdata/efficiency-round3-final/peg-owned`. Both remain normal pure-Go amd64
builds with `GOAMD64=v1`; these changes add no dependency or ISA requirement.

| Workload | Before | After | Result |
| --- | ---: | ---: | --- |
| Million-row Parquet multiple COUNT DISTINCT, four workers | 176.304 ms | 87.191 ms | 2.02x faster |
| Million-row table multiple COUNT DISTINCT, four workers | 122.515 ms | 73.416 ms | 1.67x faster |
| Parquet COUNT DISTINCT cold sampled RSS growth, four workers | 321.8 MiB | 197.6 MiB | 38.6% lower |
| Spill grouped DISTINCT, 192 KiB budget | 556.977 ms | 263.420 ms | 2.11x faster |
| Spill grouped DISTINCT cumulative Go allocation | 396.6 MB/op | 30.2 MB/op | 92.4% lower |
| Spill sort, 16,384 rows, 128 KiB budget | 68.092 ms | 35.025 ms | 1.94x faster |

Serial million-row COUNT DISTINCT improves 1.12x on Parquet and 1.22x on the
retained table. External global DISTINCT improves 2.04x with 92.1% less cumulative
Go allocation. Spill sort's Go allocation falls 96.3% at 128 KiB and 97.8% at
1 MiB. Reusable buffers slightly raise live segment peaks: grouped spill 34,046
to 36,032 bytes, global spill 31,688 to 32,192, and sort 15,396 to 15,616 at the
128 KiB budget. The memory report distinguishes scope, slab capacity and RSS;
Linux interval peaks are sampled lower bounds with a maximum observed gap of
5.548 ms and zero sampling errors.

The paired matrix has 336 observations and 1,680 samples across five Parquet
fixtures and a million-row table, one/four workers and two reversed-order rounds.
One tiny wide-output control rises 0.121 to 0.147 ms in that matrix; its isolated
three-round, 120-sample recheck measures 0.133 to 0.136 ms (+2.5%). Both reports
are retained. Four-engine smoke checks add 888 observations and fresh-process
memory comparisons add 16, all with zero failures or fingerprint mismatches.
Normal/SIMD full suites, race/checkptr, final affected follow-ups, ARM64 compile
checks and AVX2-off tests pass. Python's 15 tests pass with two macOS-only skips.
Native ARM64 runtime/timing remains unverified. Intermediate reuse-only/key-only
binaries and reports are preserved separately; no commits or pushes were made.

## Second efficiency round: implementation and eligibility

The five follow-on steps are implemented; the benchmark and complete validation
record is in
[`2026-10-02-efficiency-round2-summary.json`](scripts/benchmark-results/2026-10-02-efficiency-round2-summary.json).
The frozen comparison starts at checkpoint `48c9640`; earlier tables below
describe earlier changes and are not measurements of this round.

1. **Multiple and grouped DISTINCT aggregates.** Eligible multiple global
   aggregates with DISTINCT use one constant-key grouped pipeline, including
   mixed ordinary reductions. Identical bound DISTINCT expressions share one
   deduplication set across aggregate kinds; aliases and public field IDs stay
   intact. Local sets merge by globally deduplicating values before COUNT/SUM/AVG
   reductions. Independent global expression sets merge concurrently within the
   worker cap. Grouped DISTINCT participates in ordinary grouped merge/shard
   execution; new shard keys transfer owned group state instead of rebuilding
   all sets. MIN/MAX preserves its signed-zero preference even when DISTINCT
   coalesces the zero keys. Terminal global reductions and supported grouped
   sort/limit tails are eligible; unsupported pipeline shapes retain fallback.
2. **Chained parallel joins and join-to-TopN.** Multiple eligible equality joins
   publish immutable build state, private worker probe scratch, and per-full-join
   match maps. Unmatched build output from an earlier join passes through later
   joins before their final unmatched output. Conservative disjoint ordinal
   ranges cover duplicate expansion and these tails, so TopN ties stay stable;
   range overflow retains serial fallback. Multiple source tasks and the normal
   worker/budget admission rules still apply. Build remains serial, and input
   collection/final gathering for full sorting remain serial.
3. **Selected Parquet materialization and exact short strings.** Engine scans
   opt into copying only active flat-string payloads after pushed predicates.
   Batches retain their physical row domain, source validity and separate row
   selection; inactive flat-string values are unspecified in this mode. The
   default low-level reader still materializes all values, and eligible encoded
   dictionaries remain retained. Fully rejected batches advance and validate
   encoded input without constructing output vectors. String-sort equality can
   use a complete cached prefix plus equal lengths at most 15 bytes; embedded
   NULs, unequal lengths and longer collisions retain exact comparisons.
4. **Experimental fingerprint probing.** Allocator-backed control bytes screen
   16-slot groups before full hashes and exact key comparisons. Scalar masks are
   always available; experiment-gated amd64 AVX2 dispatch checks CPU support,
   and arm64 uses NEON intrinsics. Scalar/intrinsic differential tests cover
   collisions, wraparound and control masks. Default cache activation is disabled:
   measured hit-heavy costs make a universal replacement unjustified. The private
   experimental path and benchmarks remain available; no default SIMD/hash
   speedup is claimed. There is no new computation-kernel assembly or core Go dependency.
5. **Opt-in bounded external execution.** A nonempty `SpillDirectory` enables
   eligible sort/TopN, grouped/global aggregation, standalone DISTINCT, and
   eligible single equality joins. Sort runs preserve stable ordinals and merge
   with bounded fan-in. Aggregation streams sorted group lanes; identical DISTINCT
   expressions share a lane. DISTINCT values retain their first source ordinal
   and are sorted back into that order before SUM/AVG, preserving serial
   floating-point evaluation order. Pairwise lane-file merging bounds open
   readers/state even for a hot group/global DISTINCT. Single joins partition
   serialized rows, build bounded chunks and track matches in temporary storage;
   filter/project streaming tails or terminal ordinary global/grouped aggregate
   tails are supported. Join sort/limit tails, multiple joins and unsupported
   build pipelines retain the normal execution fallback. Temporary files contain
   serialized values, not process pointers, and cleanup runs on completion,
   cancellation, sink stop, allocation failure and IO failure.

Spilling is opt-in and currently selects external execution for eligible shapes;
it is not an adaptive in-memory-to-disk transition. Its memory contract applies
to requested live allocator bytes. Retained sink outputs remain charged until
released; decoded pages and individual records still need to fit. IO failures
return execution errors rather than silently weakening the budget.

Focused normal, race and aggressive checkptr suites cover shared DISTINCT state,
null/NaN/signed-zero behavior, high-cardinality shard transfers, exact floating
DISTINCT order, retained output, early sinks, cancellation and exhausted budgets.
External aggregation tests complete with a 96 KiB budget where the in-memory
fixture exhausts it, including a sorted offset/limit tail. Full normal, race and
aggressive checkptr suites pass with and without the SIMD experiment. ARM64
normal/SIMD compilation passes with vet disabled for existing assembly debt;
AVX2-disabled fallback tests pass. Native arm64 runtime and timing remain
unverified. Python's 15 tests pass with two macOS-only skips, including 222
DataFusion profile/source/query comparisons.

The paired normal-build comparison covers 14 queries, five Parquet fixtures and
one million-row retained table, one/four workers, reversed variant/worker order,
two warmups and five samples in each of two rounds: 48 records, 672 observations,
3,360 samples, zero failures or mismatches, and unchanged source hashes. At one
million Parquet rows, four-worker multiple COUNT DISTINCT improves from
222.360 to 150.152 ms (1.48x); serial improves from 222.508 to 197.629 ms.
String sorting improves from 284.102 to 271.832 ms serially and 180.907 to
174.030 ms with four workers. The only paired median regression above 10% is
tiny four-worker group SUM, 0.037 to 0.046 ms; all regressions remain reported.

The new chained-join/TopN microbenchmark expands 131,072 input rows to 524,288
join candidates, returning 50 rows after offset 17. Six executions per shape
average about 56.93 ms serially and 17.69 ms with four workers (3.22x), with
1.17/1.67 MB peak requested query bytes. Selected Parquet materialization copies
560 rather than 62,664 string payload bytes for the same 38 active rows and takes
about 208 rather than 246 microseconds. Experimental SIMD controls improve
lookup misses about 20–26%, but hits regress 20–35%; scalar controls regress too.
The private cache stays disabled in production.

Spill sort streams 16,384 rows under a 128 KiB budget with a 15,396-byte scoped
peak; a 1 MiB budget reduces its time from about 71.77 to 20.12 ms. External
grouped/shared DISTINCT streams 2,048 groups at a 34,046-byte scoped peak under
192 KiB, but takes about 586 ms and allocates about 397 MB of cumulative Go
metadata per execution. These are bounded-working-state functionality results,
not competitive spill-throughput claims; reusable decoding/output metadata is
an immediate optimization target. Cumulative Go allocations, scoped live bytes,
RSS and idle slab capacity are different quantities.

Four-engine smoke validation covers all 37 queries across five Parquet fixtures
and a million-row retained table: 1,776 engine/query observations, zero failures
or mismatches. A further 32 fresh-process Linux memory records validate four
affected queries at one million rows/four workers over two rounds. DataFusion
is pinned at 54.0.0 with Arrow 25.0.1; its CSV null-marker path is excluded.
Linux peaks sample approximate VmRSS every 5 ms, report precise RSS/PSS endpoints
and separate lifetime VmHWM, and are lower bounds. Actual maximum sample gap was
7.81 ms with no sampling errors. Native output adapters differ, especially
DuckDB's timed Python-row conversion, so these are not pure engine rankings.
Four additional checkpoint memory records match candidate fingerprints. String
sort cold sampled query RSS growth stays approximately 129 MiB; multiple
DISTINCT increases from 148.0 to 311.8 MiB (2.11x). Partial local sets coexist
with global merge state, so the 1.48x timing improvement has a substantial memory
tradeoff. Release/transfer of consumed sets and representation reuse are next.

## First efficiency round: historical checkpoint results

This section records the implementation and measurements committed in `48c9640`.
Its multiple/grouped DISTINCT, chained-join, join-to-TopN and spilling limitations
are superseded by the second-round eligibility above.

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

## Portable string-prefix sorting: historical October 2, 2026

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

## Compact execution state and admission fixes: historical October 1–2, 2026

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
