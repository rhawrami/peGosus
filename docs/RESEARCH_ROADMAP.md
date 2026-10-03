# Library efficiency research and implementation priorities

Research checked October 2, 2026. The goal is an efficient, embeddable, pure-Go
analytical engine across supported architectures. The amd64 baseline is one
measurement source; representation, scheduling, ownership, and avoiding work
matter across machines. The string-prefix foundation, first five execution/scan
steps, second five-step efficiency round, and DISTINCT/spill memory follow-on are
implemented. Their concrete
eligibility boundaries and the remaining work are identified below.

## Current implementation: the second efficiency round

Checkpoint `48c9640` contains the first round and has been pushed. The second
round remains uncommitted; these are implementation facts, not speedup claims.
The timing/validation record is in
[the round-two report](../scripts/benchmark-results/2026-10-02-efficiency-round2-summary.json).
Historical reports later in this document measure earlier changes. Paired
four-worker million-row Parquet multiple DISTINCT counts improve 1.48x; the
new chained-join/TopN shape scales 3.22x in its focused microbenchmark. Selected
string copies drop from 62,664 to 560 bytes for 38 active rows. Controls remain
disabled after measured hit regressions. External grouping's high cumulative
metadata allocation rate is a remaining efficiency problem, despite bounded
requested live payload storage. See the report for controls, regressions,
scope/process-memory distinctions and synthetic/adaptor limitations.

- Multiple global and grouped DISTINCT aggregates now use local/merge/final
  execution. Identical bound DISTINCT expressions share deduplication sets
  across COUNT/SUM/AVG and other eligible kinds. Global sets merge independently
  within worker limits; disjoint shard keys transfer owned state. Nulls, canonical
  NaNs, signed-zero extrema and field IDs have differential tests.
- Eligible parallel pipelines support chained equality joins and join-to-TopN,
  including full-join unmatched tails. Conservative ordinals cover source tasks,
  duplicate fanout and later tails; overflow or unsupported shapes fall back.
  Build work and full-sort collection/final gathering remain serial.
- Engine Parquet scans selectively copy active flat-string payloads after pushed
  predicates while retaining physical domains, validity and row selections.
  Encoded dictionaries remain available; the default reader still preserves every
  physical value. Short equal-prefix string-sort groups have an exact equality
  proof from complete cached bytes and equal lengths, with binary/long-collision
  tests rather than assumptions about text encoding.
- Scalar control-byte fingerprint probing and experiment-gated AVX2/NEON masks
  are implemented and differentially tested. Default cache activation is disabled
  because the measured hit-heavy costs do not support a universal replacement.
  Experimental benchmarks remain useful for miss-heavy and cache-resident cases;
  SIMD availability alone is not a performance claim.
- `ExecutionOptions.SpillDirectory` opts eligible sort/TopN, aggregation,
  standalone DISTINCT and single joins into temporary-file execution. Stable
  bounded sort merges underpin streaming group/DISTINCT lanes. DISTINCT lanes
  restore first-occurrence order before floating reductions; pairwise file merges
  bound their final state. Eligible spilled single joins accept filter/project
  streaming tails or terminal ordinary aggregate tails. Join sort/limit tails,
  multiple joins and unsupported build pipelines retain the normal fallback.

External execution is explicitly selected, not an adaptive spill transition.
Requested live segment bytes remain budgeted, including retained outputs. An
individual row, decoded page or fully retained result must still fit; streaming
sinks avoid retaining the whole result. IO failures and cancellation release
query files and state. Linux process metrics and DataFusion comparisons should
be separately named, versioned and validated, rather than conflated with the
allocator's byte budget.

## DISTINCT and spill memory follow-on

The third pass transfers local DISTINCT merge sets, frees consumed state, and
uses encoded keys without representative row payload for shared COUNT/MIN/MAX
lanes. Shared SUM/AVG lanes retain original values, and floating SUM/AVG keeps
its existing merge order. This improves the representation without changing
logical equality, extrema, aliases or stable field IDs.

Reusable allocator-backed spill frames and offsets remove per-record owned batch
construction from sort/lane merging. Scans materialize bounded owned batches;
borrowed string values never escape frame reuse. Deduplication retains only the
columns needed for equality and reuses their packed buffers. Retained output,
empty global reductions, cancellation, resource errors and malformed frames are
covered by normal/race/checkptr tests. These changes require no ISA feature,
assembly or new dependency.

[The follow-on report](../scripts/benchmark-results/2026-10-02-efficiency-round3-summary.json)
records a 2.02x four-worker million-row Parquet COUNT DISTINCT improvement and
38.6% lower cold sampled RSS growth. Grouped spill is 2.11x faster with 92.4% less
cumulative Go allocation, while reusable buffers slightly raise live segment
peaks. Five Parquet fixtures plus a retained table, four-engine correctness,
resource/ownership tests and a tiny-control recheck bound these claims. These are
synthetic amd64 measurements; source/binary provenance, controls, regressions and
memory-method limitations remain in the reports.

## Next experiments

The [focused four-engine comparison](../scripts/benchmark-results/2026-10-02-cross-engine-followon-summary.json)
adds 6,440 timing samples at one/four/eight workers on million/ten-million-row
balanced, skewed/null and long-string inputs, 112 fresh-process memory records
and three isolated CPU profiles. Successful outputs have zero mismatches; 32
peGosus budget failures reproduce across 16 configurations. Spilling is disabled
for peGosus in this matrix, while competitor budget/output accounting differs.

1. Parallelize grouped aggregation followed by a global reduction. The current
   second-aggregate rejection makes high-cardinality group-then-count/SUM shapes
   serial: the ten-million-row integer query is 1.12 s at one worker and 1.14 s
   at eight. Its profile uses about one core and spends 65.25% flat CPU in
   `u64Index.get`. Measure specialized/dense integer grouping alongside boundary
   support, with allocator budgets and exact local/merge/final semantics.
2. Improve string hash/index/merge state and retained capacity. Ten-million-row
   COUNT DISTINCT only improves 1.09x from four to eight workers, and uses about
   2.4 GiB cold sampled RSS at eight despite a 1 GiB requested-byte budget. Its
   profile spends 47.58% flat CPU in lookup and 29.39% cumulative CPU in DISTINCT
   merging; those values overlap. String grouping and long-string DISTINCT fail
   at that budget. Keep alternative index activation conditional on measured
   hit/miss mix, load factor, key widths and complete queries.
3. Reduce grouping/key interpretation and string materialization, then measure
   factorized join-to-aggregate execution. The grouped report scales 7.16x within
   peGosus but remains about 6x slower than DataFusion at ten million rows/eight
   workers. Generic group accumulation is 61.86% cumulative profile CPU and
   selected string materialization is 21.06%. Dictionary preservation, typed
   composite grouping and exact expression sharing are measurable candidates.

Block-oriented spill encoding, merge fan-in, wide/skewed spill workloads, additional
blocking composition and explicit spill/streaming comparisons remain important.
Extend join sort/limit and multiple-join support only with exact residual, outer
and stable-order semantics. Preserve floating reduction order rather than applying
algebraic reassociation. Broader runtime filters, real datasets and native arm64
timings remain open; the new evidence is synthetic amd64 execution, not proof of
cross-platform superiority.

The research sections below explain the architectural choices. Earlier open
items are marked as historical where this round has since implemented them.

## First implementation: normalized string prefixes

DuckDB's [2025 sort redesign](https://duckdb.org/2025/09/24/sorting-again)
uses normalized keys and statically sized integer comparisons, thread-local
sorted runs, and parallel merging. Its
[earlier design](https://duckdb.org/2021/08/27/external-sorting) explains string
prefix comparisons with exact collision handling. The applicable principle is
to remove repeated type/validity/string interpretation from sorting.

peGosus already normalizes numeric keys. The new
[`physical_sort_string.go`](../pkg/plan/physical_sort_string.go) caches a null
class plus the first 15 string bytes in two big-endian uint64 words. Stable
radix passes skip invariant bytes; only equal-prefix runs need full-string
stable merge sorting. Zero padding does not uniquely encode strings: embedded
NUL bytes and long suffixes always receive exact comparisons. Stable passes
preserve earlier ordering from less significant sort keys.

The optional cache uses allocator storage, costs 16 bytes per selected row,
and is released before output gathering. Query-scope allocation may decline
without exhausting the query, selecting the previous comparison algorithm.
Inputs below 256 active rows also use comparison sorting. Null placement and
descending order are independent. The implementation retains source backing,
produces owned output, and checks cancellation during encoding, radix work,
and collision merging. It adds no assembly, architecture requirement, or
dependency. The initial comparison measured serial sorting; the follow-on
implementation adds stable parallel runs and merges.

The baseline CPU profile puts roughly 93% of samples under compact sort
finalization, including 26% flat in string comparison and 12% cumulative in
validity lookup. This motivates the representation change; profile percentages
are diagnostic, not benchmark speedups. Paired results and controls are in
[the string-prefix report](../scripts/benchmark-results/2026-10-02-string-prefix-summary.txt).
The [new profile](../scripts/benchmark-results/2026-10-02-string-prefix-profile-after.txt)
has about 5% flat samples in string comparison; radix work and gathering now
account for more of the remaining time. Profiles use ten timed executions and
exclude loading, warmups and result hashing.

## First follow-on: historical join/output and parallel blocking work

The implementation limits in this section describe the first checkpoint. The
current second-round capabilities above supersede its DISTINCT and pipeline
fallbacks.

1. **Batch join output.** The previous integer-join profile attributed about
   84% of samples to probing, including packed-row reads, writes, and output
   conversion. The implementation now uses allocator-backed match references,
   direct typed column gathers, and batched residual evaluation. Tests cover
   duplicate explosions, compound keys, selections, nullable/binary strings,
   all five join kinds, owned output, cancellation, and small budgets.
2. **Separate join build from probe state.** Eligible single-join pipelines
   publish immutable retained build columns and index state, with private probe
   scratch and full-join match maps. Local match maps merge before unmatched
   build output. Build remains serial, and multi-join/join-to-TopN parallel
   pipelines retain explicit fallback. Allocator thread safety does not permit
   shared payload writes; retained payload still requires owner discipline.
3. **Parallel stable sort runs and merges.** Full compact sorting now uses
   disjoint local runs and ordinal-aware merge-path partitions. Primary string
   merges compare optional cached normalized words, falling back to exact
   comparisons on collisions. Already ordered runs copy directly. Local cache
   release precedes merge-cache allocation; input collection and final owned
   gathering remain serial. Random string microbenchmarks exposed the original
   merge cost and motivated preserving normalization into this phase.
4. **Broaden partitioned aggregation/distinct.** Standalone compound DISTINCT
   now reuses existing local grouping and sharded merging with zero accumulators.
   A single global DISTINCT aggregate, or DISTINCT followed by terminal ordinary
   global aggregates, reduces globally unique output directly. Canonical NaN,
   signed-zero and null semantics stay intact. Multiple/grouped DISTINCT
   aggregates and unsupported post-DISTINCT tails remain future work. Low and
   high cardinality microbenchmarks expose partitioning and state duplication
   costs rather than assuming uniform scaling.

[Morsel-driven parallelism](https://db.in.tum.de/~leis/papers/morsels.pdf)
(Leis et al., SIGMOD 2014) supplies the foundational pipeline/task model:
fine-grained work distribution with local state and locality. It is still
relevant, but Go's scheduler and this allocator require independent measurement.
[DuckDB's external aggregation](https://duckdb.org/2024/03/29/external-aggregation)
connects local hash states, partitioned merging, and bounded spillable storage.
These are execution architecture directions, not a request for a goroutine
per batch or a mutex around every hash lookup.

## Reduce work before accelerating loops

[DataFusion's dynamic filters](https://datafusion.apache.org/blog/2025/09/10/dynamic-filters/)
feed TopK thresholds and join information into scans to avoid decoding and
materializing rows. Its
[55.0.0 release](https://datafusion.apache.org/blog/output/2026/08/25/datafusion-55.0.0/)
adds runtime re-evaluation of remaining Parquet row groups, compound sort
support, broader column-wise group storage, and dictionary-preserving string
functions. These are useful directions for our static scan predicates and
existing dictionary propagation.

peGosus now publishes inclusive signed integer thresholds from full eligible
TopN heaps and min/max ranges from inner/semi build keys. Readers recheck unread
row groups before column-chunk IO; null placement, unfilled heaps, exact widening
and compound primary ties have explicit tests. Guarded ReaderAt fixtures prove
avoided IO, and microbenchmarks report decoded rows and pruned groups. General
computed/float keys and outer/anti joins remain excluded. Late payload
materialization and richer filters should be measured against this foundation.

Typed expression evaluation and exact common-subexpression sharing are another
portable opportunity. Reuse only identical bound expressions with matching
type, nullability, literal bits, and semantics. Measure the 128-expression
workload and realistic shared arithmetic/string projections, accounting for
materialization memory. Preserve field IDs; do not reassociate floating-point
arithmetic or introduce approximate reciprocals.

## SIMD and adaptive hash representations

[Analyzing Vectorized Hash Tables Across CPU Architectures](https://lawben.com/publication/vectorized_hash_tables/vectorized_hash_tables_vldb23.pdf)
(Böther et al., VLDB 2023) compares vectorized linear probing, fingerprinting,
and bucket designs on multiple architectures. It supports testing compact
metadata/fingerprint probing, not assuming one SIMD layout wins everywhere.

The second round implements allocator-backed control bytes and scalar/AVX2/NEON
masks while retaining the default probe path. Compare future concrete key/payload
layouts and cache activation against the specialized hash paths. Sweep load factor,
cardinality, duplicates, key width, cache residency, and batch selection density.
The implementations use Go 1.27.1 `simd/archsimd` behind `GOEXPERIMENT=simd`,
with separate amd64 AVX2 and arm64 NEON files. AVX2 dispatch checks CPU support;
normal and unsupported builds retain scalar masks. Random differential tests
check identical results. Keep IEEE behavior and the kernel ABI intact.
CSV byte classification and Parquet decoding are separate measurable SIMD
targets; this change adds no new computation-kernel assembly.

[Adaptive Factorization Using Linear-Chained Hash Tables](https://www.vldb.org/cidrdb/papers/2025/p21-gro.pdf)
(Groß, ten Wolde, Boncz, CIDR 2025) studies duplicate chains and adaptive
factorization. For us this suggests measuring delayed duplicate expansion and
eligible join-to-aggregate execution. It is a later experiment: preserving
floating-point evaluation order, outer/residual semantics, and general output
requires explicit eligibility and fallback.

## Memory limits and evidence needed for competitiveness

The second round preserves spill boundaries with stable sort runs, group/DISTINCT
lanes and bounded single-join partitions/chunks. Temporary records serialize
values and strings instead of raw German-string pointers. Extend their adaptive
admission and pipeline eligibility only with measured budgets and skew.
Independently measure requested live bytes, reusable slab capacity,
Go heap, and a clearly named Linux process metric. Allocator budget is not RSS;
maintenance still requires quiescence.

Extend the benchmark suite with multi-file scans, encoded/dictionary/plain
strings, sparse selections, shared prefixes, sorted inputs, wide joins,
many-to-many joins, high-cardinality grouping, memory pressure, and concurrent
queries. Add native arm64 execution and worker scaling beyond 1/4. Benchmark
stage-level counters and end-to-end queries separately, with frozen binaries,
checksummed fixtures, result validation, and reversed measurement order.

The existing comparator pins DuckDB 1.5.6 and Polars 1.34.0 and has no DataFusion
runner. Add a separately versioned contemporary comparison rather than silently
changing historical reports. The
[Polars 2.0 announcement](https://pola.rs/posts/announcing-polars-2/)
is a September 2026 pre-release describing a planned default streaming engine;
its stated join/group ordering changes must not weaken our own semantics.
Published engine speedups and microbenchmark results are not peGosus claims.

The first and second five-step rounds are delivered within the eligibility in
[HANDOFF.md](../HANDOFF.md). First-round measurements remain in the
[library-efficiency report](../scripts/benchmark-results/2026-10-02-library-efficiency-summary.txt);
second-round evidence is being assembled in
[its own report](../scripts/benchmark-results/2026-10-02-efficiency-round2-summary.json).
Remaining work includes adaptive spill selection, broader spilled join tails,
additional blocking-stage combinations, richer runtime filters, reduced generic
state/merge costs, factorized joins, and measured hash-cache activation. These
are boundaries to investigate against the current representations, not grounds
to assume every pipeline scales or every query now completes under any budget.
