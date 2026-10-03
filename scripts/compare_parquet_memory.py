"""Compare isolated peGosus/DuckDB/Polars process memory on macOS or Linux.

On macOS, uses libproc's resettable physical-footprint peak, including
submillisecond queries. Linux records /proc RSS at 5 ms intervals plus stage
endpoints; sampled peaks are lower bounds and lifetime VmHWM stays separate. Each engine/query/thread/round gets a fresh child process. Results
stay native until validation after all memory snapshots. Python/native-library
startup is reported separately from query growth. Run with the same Python
environment as compare_parquet_engines.py; no monitoring dependency is needed.

Example:
    python scripts/compare_parquet_memory.py --parquet bench.parquet --rows 1048768
"""

import argparse
import ctypes
import contextlib
import gc
import hashlib
import json
import os
import pathlib
import platform
import select
import statistics
import subprocess
import sys
import tempfile
import time

from compare_parquet_engines import check_results


ROOT = pathlib.Path(__file__).resolve().parents[1]
QUERIES = ("filter_count", "selective_count", "group_sum", "top50",
           "string_filter_sum", "square_sum", "group_day_sum")
ENGINES = ("peGosus", "DuckDB", "Polars")
STAGES = ("cold", "release", "warmup", "steady", "retain", "release_all", "gc")
RUSAGE_FIELDS = (
    "user_time system_time pkg_idle_wkups interrupt_wkups pageins wired_size "
    "resident_size phys_footprint proc_start_abstime proc_exit_abstime "
    "child_user_time child_system_time child_pkg_idle_wkups child_interrupt_wkups "
    "child_pageins child_elapsed_abstime diskio_bytesread diskio_byteswritten "
    "cpu_time_qos_default cpu_time_qos_maintenance cpu_time_qos_background "
    "cpu_time_qos_utility cpu_time_qos_legacy cpu_time_qos_user_initiated "
    "cpu_time_qos_user_interactive billed_system_time serviced_system_time "
    "logical_writes lifetime_max_phys_footprint instructions cycles billed_energy "
    "serviced_energy interval_max_phys_footprint runnable_time"
).split()


class RUsage(ctypes.Structure):
    _fields_ = [("uuid", ctypes.c_uint8 * 16)] + [
        (name, ctypes.c_uint64) for name in RUSAGE_FIELDS]


class ProcessMemory:
    current_key = "phys_footprint"
    peak_key = "interval_max_phys_footprint"
    metric = "phys_footprint"
    description = "macOS libproc physical footprint and resettable interval peak; resident size also recorded"

    def __new__(cls):
        if platform.system() == "Linux":
            from process_memory_linux import LinuxProcessMemory
            return LinuxProcessMemory()
        return super().__new__(cls)

    @contextlib.contextmanager
    def interval(self, pid):
        self.reset(pid)
        yield None

    def __init__(self):
        if platform.system() != "Darwin":
            raise RuntimeError("this harness requires macOS libproc footprint counters")
        self.lib = ctypes.CDLL("/usr/lib/libproc.dylib", use_errno=True)
        self.lib.proc_pid_rusage.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_void_p]
        self.lib.proc_pid_rusage.restype = ctypes.c_int
        self.lib.proc_reset_footprint_interval.argtypes = [ctypes.c_int]
        self.lib.proc_reset_footprint_interval.restype = ctypes.c_int

    def snapshot(self, pid):
        usage = RUsage()
        if self.lib.proc_pid_rusage(pid, 4, ctypes.byref(usage)) != 0:
            raise OSError(ctypes.get_errno(), "proc_pid_rusage failed")
        return {name: getattr(usage, name) for name in (
            "resident_size", "phys_footprint", "interval_max_phys_footprint",
            "lifetime_max_phys_footprint", "diskio_bytesread", "diskio_byteswritten")}

    def reset(self, pid):
        if self.lib.proc_reset_footprint_interval(pid) != 0:
            raise OSError(ctypes.get_errno(), "proc_reset_footprint_interval failed")


def native_worker(args):
    path = str(args.parquet).replace("'", "''")
    scan = f"read_parquet('{path}')"
    if args.worker_engine == "DuckDB":
        import duckdb
        connection = duckdb.connect()
        connection.execute(f"SET threads={args.worker_threads}")
        connection.execute("SET memory_limit='512MiB'")
        sql = {
            "filter_count": f"SELECT count(*) FROM {scan} WHERE id >= 0 AND active",
            "selective_count": f"SELECT count(*) FROM {scan} WHERE id >= {args.rows - 4000} AND active",
            "group_sum": f"SELECT category, sum(id) FROM {scan} WHERE id >= {args.rows // 4} GROUP BY category ORDER BY category ASC NULLS LAST",
            "top50": f"SELECT id, score FROM {scan} WHERE score IS NOT NULL ORDER BY score DESC NULLS LAST, id ASC LIMIT 50",
            "string_filter_sum": f"SELECT sum(id) FROM {scan} WHERE category = 'north' AND score > -23.0",
            "square_sum": f"SELECT sum(score * score) FROM {scan} WHERE score IS NOT NULL",
            "group_day_sum": f"SELECT datediff('day', DATE '1970-01-01', day), sum(id) FROM {scan} WHERE id >= {args.rows // 4} GROUP BY day ORDER BY day ASC NULLS LAST",
        }
        operation = lambda: connection.execute(sql[args.worker_query]).fetchall()
        convert = lambda result: [list(row) for row in result]
        version = duckdb.__version__
    else:
        import polars as pl
        scan = pl.scan_parquet(args.parquet)
        plans = {
            "filter_count": scan.filter((pl.col("id") >= 0) & pl.col("active")).select(pl.len()),
            "selective_count": scan.filter((pl.col("id") >= args.rows - 4000) & pl.col("active")).select(pl.len()),
            "group_sum": scan.filter(pl.col("id") >= args.rows // 4).group_by("category").agg(pl.col("id").cast(pl.Int64).sum()).sort("category", nulls_last=True),
            "top50": scan.select("id", "score").filter(pl.col("score").is_not_null()).sort(["score", "id"], descending=[True, False], nulls_last=True).head(50),
            "string_filter_sum": scan.filter((pl.col("category") == "north") & (pl.col("score") > -23.0)).select(pl.col("id").cast(pl.Int64).sum()),
            "square_sum": scan.filter(pl.col("score").is_not_null()).select((pl.col("score") * pl.col("score")).sum()),
            "group_day_sum": scan.filter(pl.col("id") >= args.rows // 4).group_by("day").agg(pl.col("id").cast(pl.Int64).sum()).sort("day", nulls_last=True).with_columns(pl.col("day").cast(pl.Int32)),
        }
        operation = plans[args.worker_query].collect
        convert = lambda result: [list(row) for row in result.rows()]
        version = pl.__version__
        if pl.thread_pool_size() != args.worker_threads:
            raise RuntimeError("Polars thread count differs from the requested count")
    held = []
    gc.collect()
    print(json.dumps({"stage": "ready", "version": version, "held_results": 0}), flush=True)
    for line in sys.stdin:
        stage = line.strip()
        if stage == "cold":
            held.append(operation())
        elif stage in ("release", "release_all"):
            held.clear()
        elif stage in ("warmup", "steady"):
            for _ in range(args.warmup if stage == "warmup" else args.runs):
                result = operation()
                del result
            if stage == "warmup":
                gc.collect()
        elif stage == "retain":
            for _ in range(args.retain):
                held.append(operation())
        elif stage == "gc":
            gc.collect()
        elif stage == "validate":
            result = operation()
            print(json.dumps({"stage": stage, "rows": convert(result)}), flush=True)
            del result
            continue
        elif stage == "quit":
            return
        else:
            raise RuntimeError(f"invalid worker command {stage!r}")
        print(json.dumps({"stage": stage, "held_results": len(held)}), flush=True)
    raise RuntimeError("process-memory controller disconnected")


def read_event(child, stage):
    if not select.select([child.stdout], [], [], 60)[0]:
        raise TimeoutError(f"worker did not finish {stage}")
    line = child.stdout.readline()
    if not line:
        raise RuntimeError(f"worker ended during {stage}: {child.stderr.read()}")
    event = json.loads(line)
    if event.get("stage") != stage:
        raise RuntimeError(f"expected {stage}, got {event}")
    return event


def measure_process(args, binary, counter, engine, query, threads):
    environment = {**os.environ, "GOMAXPROCS": str(threads), "POLARS_MAX_THREADS": str(threads)}
    if engine == "peGosus":
        command = [str(binary), "-parquet", str(args.parquet), "-rows", str(args.rows),
                   "-only", query, "-workers", str(threads), "-process-memory",
                   "-runs", str(args.runs), "-warmup", str(args.warmup), "-retain", str(args.retain)]
    else:
        command = [sys.executable, str(pathlib.Path(__file__).resolve()),
                   "--parquet", str(args.parquet), "--rows", str(args.rows),
                   "--worker-engine", engine, "--worker-query", query,
                   "--worker-threads", str(threads), "--runs", str(args.runs),
                   "--warmup", str(args.warmup), "--retain", str(args.retain)]
    child = subprocess.Popen(command, cwd=ROOT, env=environment, stdin=subprocess.PIPE,
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    try:
        ready = read_event(child, "ready")
        record = {"engine": engine, "query": query, "threads": threads,
                  "version": ready["version"], "snapshots": {
                      "ready": {"process": counter.snapshot(child.pid), "event": ready}}}
        for stage in STAGES:
            with counter.interval(child.pid) as sampled:
                baseline = counter.snapshot(child.pid)
                start = time.monotonic_ns()
                child.stdin.write(stage + "\n")
                child.stdin.flush()
                event = read_event(child, stage)
                if stage == "gc":
                    time.sleep(0.05)
                observation = counter.snapshot(child.pid)
            interval_metrics = sampled.metrics() if sampled is not None else {}
            peak = max(interval_metrics.get(counter.peak_key, observation.get(counter.peak_key, 0)),
                       baseline[counter.current_key], observation[counter.current_key])
            record["snapshots"][stage] = {
                "baseline": baseline, "process": observation, "event": event,
                "peak_memory_bytes": peak, "memory_metric": counter.metric,
                "peak_growth_bytes": peak - baseline[counter.current_key],
                "elapsed_ns": time.monotonic_ns() - start, **interval_metrics,
            }
            if counter.metric == "phys_footprint":
                record["snapshots"][stage]["peak_footprint_bytes"] = peak
        child.stdin.write("validate\n")
        child.stdin.flush()
        record["rows"] = read_event(child, "validate")["rows"]
        child.stdin.write("quit\n")
        child.stdin.flush()
        child.stdin.close()
        if child.wait(timeout=30) != 0:
            raise RuntimeError(child.stderr.read())
        return record
    finally:
        if child.poll() is None:
            child.kill()
            child.wait()
        child.stdout.close()
        child.stderr.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--parquet", type=pathlib.Path, required=True)
    parser.add_argument("--rows", type=int, required=True)
    parser.add_argument("--threads", type=int, nargs="+", default=[1, 4])
    parser.add_argument("--rounds", type=int, default=2)
    parser.add_argument("--warmup", type=int, default=5)
    parser.add_argument("--runs", type=int, default=20)
    parser.add_argument("--retain", type=int, default=4)
    parser.add_argument("--queries", nargs="+", choices=QUERIES, default=list(QUERIES))
    parser.add_argument("--json", type=pathlib.Path)
    parser.add_argument("--worker-engine", choices=("DuckDB", "Polars"), help=argparse.SUPPRESS)
    parser.add_argument("--worker-query", choices=QUERIES, help=argparse.SUPPRESS)
    parser.add_argument("--worker-threads", type=int, help=argparse.SUPPRESS)
    args = parser.parse_args()
    args.parquet = args.parquet.resolve()
    if not args.parquet.is_file() or args.rows <= 0 or min(args.threads) < 1 or args.rounds < 1 or args.warmup < 0 or args.runs < 1 or args.retain < 1:
        parser.error("invalid source, row count, or measurement counts")
    if args.worker_engine:
        native_worker(args)
        return
    counter = ProcessMemory()
    records = []
    with tempfile.TemporaryDirectory(prefix="peg-memory-") as directory:
        binary = pathlib.Path(directory) / "peg"
        subprocess.run(["go", "build", "-o", str(binary), "./scripts/compare_parquet_peg.go"], cwd=ROOT, check=True)
        for round_ in range(args.rounds):
            for threads in args.threads if round_ % 2 == 0 else reversed(args.threads):
                for query in args.queries:
                    engines = ENGINES if round_ % 2 == 0 else reversed(ENGINES)
                    for engine in engines:
                        record = measure_process(args, binary, counter, engine, query, threads)
                        record["round"] = round_ + 1
                        records.append(record)
                    reference = next(r for r in records if r["round"] == round_+1 and r["threads"] == threads and r["query"] == query and r["engine"] == "DuckDB")["rows"]
                    for record in records[-3:]:
                        check_results(reference, record["rows"], f"{record['engine']}/{query}")
                    print(f"Validated round {round_+1}, threads {threads}, {query}", flush=True)
    summary = []
    for threads in args.threads:
        print(f"\n{threads} threads/workers, MiB {counter.metric} (median of {args.rounds} fresh processes):")
        print(f"{'query/engine':30} {'startup':>9} {'cold peak':>10} {'warm peak':>10} {'warm grow':>10} {'released':>10}")
        for query in args.queries:
            for engine in ENGINES:
                selected = [r["snapshots"] for r in records if r["threads"] == threads and r["query"] == query and r["engine"] == engine]
                metrics = {
                    "startup_footprint_bytes": statistics.median(s["ready"]["process"][counter.current_key] for s in selected),
                    "cold_peak_footprint_bytes": statistics.median(s["cold"]["peak_memory_bytes"] for s in selected),
                    "warm_baseline_footprint_bytes": statistics.median(s["warmup"]["process"][counter.current_key] for s in selected),
                    "warm_peak_footprint_bytes": statistics.median(s["steady"]["peak_memory_bytes"] for s in selected),
                    "warm_peak_above_startup_bytes": statistics.median(s["steady"]["peak_memory_bytes"] - s["ready"]["process"][counter.current_key] for s in selected),
                    "cold_peak_above_startup_bytes": statistics.median(s["cold"]["peak_memory_bytes"] - s["ready"]["process"][counter.current_key] for s in selected),
                    "warm_peak_growth_bytes": statistics.median(s["steady"]["peak_growth_bytes"] for s in selected),
                    "four_results_footprint_bytes": statistics.median(s["retain"]["process"][counter.current_key] for s in selected),
                    "released_footprint_bytes": statistics.median(s["gc"]["process"][counter.current_key] for s in selected),
                }
                if counter.metric != "phys_footprint":
                    metrics = {key.replace("footprint", "rss"): value for key, value in metrics.items()}
                summary.append({"query": query, "engine": engine, "threads": threads, "memory_metric": counter.metric, **metrics})
                metric_keys = ("startup_footprint_bytes", "cold_peak_footprint_bytes", "warm_peak_footprint_bytes", "warm_peak_growth_bytes", "released_footprint_bytes")
                if counter.metric != "phys_footprint":
                    metric_keys = tuple(key.replace("footprint", "rss") for key in metric_keys)
                columns = [metrics[k]/(1<<20) for k in metric_keys]
                print(f"{query+'/'+engine:30}" + "".join(f"{v:10.2f}" for v in columns))
    result = {
        "machine": platform.platform(), "rows": args.rows, "parquet_bytes": args.parquet.stat().st_size,
        "parquet_sha256": hashlib.sha256(args.parquet.read_bytes()).hexdigest(),
        "git_head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
        "source_sha256": {
            str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in sorted([path for path in (ROOT / "pkg").rglob("*.go") if not path.name.endswith("_test.go")] + [
                ROOT / "scripts/compare_parquet_memory.py", ROOT / "scripts/compare_parquet_peg.go",
                ROOT / "scripts/compare_parquet_engines.py", ROOT / "scripts/process_memory_linux.py"])
        },
        "rounds": args.rounds, "runs": args.runs, "warmup": args.warmup, "retain": args.retain,
        "method": {
            "metric": counter.description,
            "isolation": "one fresh process per engine/query/thread/round; engines/threads reverse in second round",
            "baseline": "engine/library startup and lazy/prepared query construction, followed by language GC; no query has executed",
            "cold": "one execution with result held; warm file cache, not cold disk",
            "steady": "five warmups by default followed by language GC, then twenty executions with immediate result release; independent peak interval per phase",
            "retention": "four independently executed results held, then released, then language GC; engine remains alive and retains reusable caches/slabs",
            "results": "peGosus native Result, DuckDB fetchall Python row list, Polars native DataFrame; all outputs <=367 rows on this fixture; validation/conversion after memory snapshots",
            "limits": "peGosus and DuckDB 512 MiB query/engine limits; Polars has no equivalent configured limit; no intentional trimming of reusable native allocator caches",
            "interpretation": "absolute process memory includes language/native-library costs; above_startup subtracts the fresh ready measurement; growth subtracts the pre-phase measurement; both include runtime/cache growth across repeated executions, not just operator payload or per-query allocation counts",
            "planning": "peGosus prepares once; DuckDB connection.execute plans each execution; Polars reuses a lazy plan with collect per execution",
            "retention_limits": "outputs are small; four-result retention does not characterize large materialized outputs, spills, or long-running convergence",
            "peak_source": "https://github.com/apple-oss-distributions/xnu/blob/main/libsyscall/wrappers/libproc/libproc.c" if counter.metric == "phys_footprint" else "https://docs.kernel.org/filesystems/proc.html",
        },
        "summary": summary, "records": records,
    }
    if args.json:
        args.json.write_text(json.dumps(result, indent=2)+"\n")
        print("Raw snapshots:", args.json)


if __name__ == "__main__":
    main()
