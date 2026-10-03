# Library efficiency research and implementation priorities

Research checked October 2, 2026. The goal is an efficient, embeddable, pure-Go
analytical engine across supported architectures. The amd64 baseline is one
measurement source; representation, scheduling, ownership, and avoiding work
matter across machines. The string-prefix foundation and the next five execution/scan steps are now
implemented; remaining work and eligibility limits are identified below.

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

## Implemented follow-on: join output and parallel blocking operators

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

Start with allocator-backed control bytes and concrete key/payload arrays;
compare against the current specialized hash paths. Sweep load factor,
cardinality, duplicates, key width, cache residency, and batch selection density.
Then use Go 1.27.1 `simd/archsimd` behind `GOEXPERIMENT=simd`, separate amd64 AVX2
and arm64 NEON implementations, detected CPU features, scalar fallbacks, and
random differential tests. Keep IEEE behavior and the kernel ABI intact.
CSV byte classification and Parquet decoding are separate measurable SIMD
targets; this change adds no new computation-kernel assembly.

[Adaptive Factorization Using Linear-Chained Hash Tables](https://www.vldb.org/cidrdb/papers/2025/p21-gro.pdf)
(Groß, ten Wolde, Boncz, CIDR 2025) studies duplicate chains and adaptive
factorization. For us this suggests measuring delayed duplicate expansion and
eligible join-to-aggregate execution. It is a later experiment: preserving
floating-point evaluation order, outer/residual semantics, and general output
requires explicit eligibility and fallback.

## Memory limits and evidence needed for competitiveness

Preserve spill boundaries while implementing bounded local states. A future
spill format should use page-relative offsets and serialized strings, not raw
German-string pointers. Start with sort runs, then partitioned aggregation and
hash joins. Independently measure requested live bytes, reusable slab capacity,
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

The five-step follow-on is delivered: bounded typed join gathers and batched
residuals, immutable shared join build/private probing, stable parallel compact
sorts, partitioned standalone DISTINCT and eligible global reductions, and
runtime integer TopN/inner/semi Parquet filters. Exact eligibility and ownership
are recorded in [HANDOFF.md](../HANDOFF.md). Measurements are in the
[library-efficiency report](../scripts/benchmark-results/2026-10-02-library-efficiency-summary.txt).
Multiple/grouped DISTINCT aggregates, multi-join parallel pipelines, join-to-TopN
parallel execution, late payload materialization, SIMD hash metadata, and
spilling remain open. The next experiments should measure those boundaries
against these representations, rather than assume every pipeline now scales.
