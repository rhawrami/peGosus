"""Run isolated, validated synthetic BI workloads on peGosus/DuckDB/Polars.

Examples:
  python scripts/compare_workload_engines.py --json /tmp/workloads.json
  python scripts/compare_workload_engines.py --cases balanced:65536 --source table
  python scripts/compare_workload_engines.py --cases balanced:65536 --memory --only wide_materialize,join_string_report

Requires Go, DuckDB, Polars; optional DataFusion uses requirements-datafusion.txt.
Process memory uses macOS physical footprint or Linux sampled RSS.
No network or fixture downloads are needed. Fixtures persist in --scratch.
"""
import argparse
import gc
import hashlib
import json
import os
import pathlib
import platform
import statistics
import subprocess
import sys
import time

from workload_suite import CASES, PAPER, QUERIES, fingerprint, make_fixture, polars_query, quote, sql_query

ROOT = pathlib.Path(__file__).resolve().parents[1]
ENGINES = ("peGosus", "DuckDB", "Polars")
LIMITATIONS = [
    "Synthetic workloads informed by the Tableau Public study, not a replay or a representative production query distribution.",
    "Warm filesystem cache; compilation, source loading and peGosus Prepare excluded. DuckDB Python execute includes per-execution SQL planning; Polars collect includes optimization.",
    "Native materialization: peGosus owned Result, Polars DataFrame, DuckDB Python fetchall rows. DuckDB Python conversion is included in execution and process memory; fingerprints are outside timing windows.",
    "Output release measured separately; process startup and retained-table input loading excluded from timing and included in process-memory baseline.",
    "No collation, dirty string-to-number/date casts, window functions, percentile, spilling, many-to-many join stress, or cancellation timing; no official TPC/ClickBench score.",
    "Dyadic numeric values permit exact fingerprints but exclude floating-point reduction rounding stress, NaN and infinity performance.",
    "Go and DuckDB query memory budget 1 GiB; Polars has no equivalent configured hard budget. Thread counts cap runtimes but not every operator parallelizes.",
    "Unordered wide output uses a multiset fingerprint; unordered LIMIT validates row membership and uniqueness rather than equal subsets.",
    "peGosus has a configurable per-execution guard deadline (default 10s). Native whole-worker watchdog is 1800s. Failures and cancellations have no comparison timing; completed partial samples are not ranked.",
]


def native_setup(args, fixture):
    started = time.perf_counter_ns()
    if args.worker_engine == "DuckDB":
        import duckdb
        connection = duckdb.connect()
        connection.execute(f"SET threads={args.worker_threads}")
        connection.execute("SET memory_limit='1GiB'")
        for table, file in (("facts", "parquet"), ("dimension", "dimension")):
            if args.source == "csv":
                fields = fixture["fields" if table == "facts" else "dimension_fields"]
                types = {"int32": "INTEGER", "int64": "BIGINT", "float64": "DOUBLE", "string": "VARCHAR", "bool": "BOOLEAN", "date": "DATE"}
                columns = "{" + ",".join(f"{quote(f['name'])}:{quote(types[f['kind']])}" for f in fields) + "}"
                source = f"read_csv({quote(fixture['csv' if table == 'facts' else 'dimension_csv'])},header=true,nullstr='\\N',columns={columns})"
            else:
                source = f"read_parquet({quote(fixture[file])})"
            kind = "TABLE" if args.source == "table" else "VIEW"
            connection.execute(f"CREATE TEMP {kind} {table} AS SELECT * FROM {source}")
        def make_operation(name):
            sql = sql_query(name, fixture["rows"])
            return lambda: connection.execute(sql).fetchall()
        return make_operation, lambda result: iter(result), duckdb.__version__, time.perf_counter_ns()-started
    if args.worker_engine == "DataFusion":
        from workload_datafusion import datafusion_setup
        make_operation, convert, version = datafusion_setup(args, fixture)
        return make_operation, convert, version, time.perf_counter_ns()-started
    import polars as pl
    if pl.thread_pool_size() != args.worker_threads:
        raise RuntimeError("Polars thread pool mismatch")
    def load(table):
        path = fixture["parquet" if table == "facts" else "dimension"]
        if args.source == "table":
            return pl.read_parquet(path).lazy()
        if args.source == "csv":
            fields = fixture["fields" if table == "facts" else "dimension_fields"]
            kinds = {"int32": pl.Int32, "int64": pl.Int64, "float64": pl.Float64, "string": pl.String, "bool": pl.Boolean, "date": pl.Date}
            return pl.scan_csv(fixture["csv" if table == "facts" else "dimension_csv"], schema={f['name']: kinds[f['kind']] for f in fields}, null_values="\\N", try_parse_dates=True)
        return pl.scan_parquet(path)
    facts, dimension = load("facts"), load("dimension")
    def make_operation(name):
        plan = polars_query(name, facts, dimension, fixture["rows"])
        return plan.collect
    return make_operation, lambda result: result.iter_rows(), pl.__version__, time.perf_counter_ns()-started


def native_worker(args):
    fixture = json.loads(args.manifest.read_text())
    make_operation, convert, version, load_ns = native_setup(args, fixture)
    gc.collect()
    if args.memory:
        operation = make_operation(args.only)
        held = []
        print(json.dumps({"stage": "ready", "version": version, "held_results": 0}), flush=True)
        for command in sys.stdin:
            stage = command.strip()
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
                held.extend(operation() for _ in range(args.retain))
            elif stage == "gc":
                gc.collect()
            elif stage == "validate":
                result = operation()
                print(json.dumps({"stage": stage, "signature": fingerprint(convert(result), args.only)}), flush=True)
                del result
                continue
            elif stage == "quit":
                return
            else:
                raise ValueError(stage)
            print(json.dumps({"stage": stage, "held_results": len(held)}), flush=True)
        raise RuntimeError("memory controller disconnected")
    measurements = []
    for name in args.only.split(","):
        entry = {"name": name, "times_ns": [], "release_ns": []}
        try:
            start = time.perf_counter_ns()
            operation = make_operation(name)
            entry["build_ns"] = time.perf_counter_ns()-start
            for iteration in range(args.warmup + args.runs):
                start = time.perf_counter_ns()
                result = operation()
                finish = time.perf_counter_ns()
                del result
                release = time.perf_counter_ns()
                if iteration >= args.warmup:
                    entry["times_ns"].append(finish-start)
                    entry["release_ns"].append(release-finish)
            result = operation()
            entry["signature"] = fingerprint(convert(result), name)
            del result
        except Exception as error:
            entry["error"] = f"{type(error).__name__}: {error}"
        measurements.append(entry)
    print(json.dumps({"engine": args.worker_engine, "version": version, "source": args.source,
                      "workers": args.worker_threads, "load_ns": load_ns, "queries": measurements}))


def command(args, binary, engine, manifest, workers, names):
    if engine == "peGosus":
        return [str(binary), "-manifest", str(manifest), "-source", args.source,
                "-workers", str(workers), "-only", names, "-runs", str(args.runs),
                "-warmup", str(args.warmup), "-timeout", f"{args.peg_timeout}s", *(["-process-memory", "-retain", str(args.retain)] if args.memory else [])]
    return [sys.executable, str(pathlib.Path(__file__).resolve()), "--manifest", str(manifest),
            "--source", args.source, "--worker-engine", engine, "--worker-threads", str(workers),
            "--only", names, "--runs", str(args.runs), "--warmup", str(args.warmup),
            *(["--memory", "--retain", str(args.retain)] if args.memory else [])]


def check_preview(signature, fixture):
    import duckdb
    sample = signature.get("sample", [])
    ids = [row[0] for row in sample]
    if signature["rows"] != min(100, fixture["rows"]) or len(ids) != signature["rows"] or len(set(ids)) != len(ids):
        raise ValueError("invalid LIMIT row count or duplicate IDs")
    connection = duckdb.connect()
    try:
        expected = dict((row[0], list(row)) for row in connection.execute(f"SELECT id,category,score FROM read_parquet({quote(fixture['parquet'])}) WHERE id IN ({','.join(str(int(i)) for i in ids)})").fetchall())
    finally:
        connection.close()
    if any(expected.get(row[0]) != row for row in sample):
        raise ValueError("LIMIT returned a row not present in the source")


def validate(records, fixture, names):
    errors = []
    for name in names:
        entries = [(record["engine"], query) for record in records for query in record["queries"] if query["name"] == name]
        if len(entries) != len(records):
            errors.append(f"{name}: missing query result")
            continue
        reference = None
        for engine, entry in entries:
            if entry.get("error"):
                continue
            try:
                signature = entry["signature"]
                if name == "preview_limit":
                    check_preview(signature, fixture)
                elif reference is None:
                    reference = signature
                elif signature != reference:
                    raise ValueError(f"result fingerprint differs: {signature} != {reference}")
            except Exception as error:
                errors.append(f"{name}/{engine}: {error}")
    return errors


def measure_memory(args, binary, manifest, engine, workers, name):
    from compare_parquet_memory import ProcessMemory, STAGES, read_event
    counter = ProcessMemory()
    env = {**os.environ, "POLARS_MAX_THREADS": str(workers), "GOMAXPROCS": str(workers)}
    child = subprocess.Popen(command(args, binary, engine, manifest, workers, name), cwd=ROOT,
                             env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    try:
        ready = read_event(child, "ready")
        record = {"engine": engine, "workers": workers, "query": name, "version": ready["version"],
                  "snapshots": {"ready": {"process": counter.snapshot(child.pid), "event": ready}}}
        record["memory_method"] = counter.description
        for stage in (*STAGES, "validate"):
            with counter.interval(child.pid) as sampled:
                before = counter.snapshot(child.pid)
                start = time.monotonic_ns()
                child.stdin.write(stage + "\n")
                child.stdin.flush()
                event = read_event(child, stage)
                after = counter.snapshot(child.pid)
            interval_metrics = sampled.metrics() if sampled is not None else {}
            peak = max(interval_metrics.get(counter.peak_key, after.get(counter.peak_key, 0)),
                       before[counter.current_key], after[counter.current_key])
            record["snapshots"][stage] = {"before": before, "process": after, "event": event,
                                          "memory_metric": counter.metric, "peak_memory_bytes": peak,
                                          "peak_growth_bytes": peak-before[counter.current_key],
                                          "elapsed_ns": time.monotonic_ns()-start, **interval_metrics}
            if stage == "validate":
                record["queries"] = [{"name": name, "signature": event["signature"]}]
        child.stdin.write("quit\n")
        child.stdin.flush()
        child.communicate(timeout=30)
        if child.returncode:
            raise RuntimeError(f"worker exit {child.returncode}")
        return record
    finally:
        if child.poll() is None:
            child.kill()
        child.communicate()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scratch", type=pathlib.Path, default=pathlib.Path("/tmp/peg-expanded-workloads"))
    parser.add_argument("--cases", default=",".join(CASES))
    parser.add_argument("--threads", default="1,4")
    parser.add_argument("--rounds", type=int, default=2)
    parser.add_argument("--runs", type=int, default=5)
    parser.add_argument("--warmup", type=int, default=2)
    parser.add_argument("--source", choices=("parquet", "table", "csv"), default="parquet")
    parser.add_argument("--only", default=",".join(QUERIES))
    parser.add_argument("--json", type=pathlib.Path)
    parser.add_argument("--binary", type=pathlib.Path, default=pathlib.Path("/tmp/peg-expanded-bench"))
    parser.add_argument("--skip-build", action="store_true")
    parser.add_argument("--peg-timeout", type=float, default=10.0)
    parser.add_argument("--engines", default=",".join(ENGINES), help="comma-separated engines; DataFusion is optional")
    parser.add_argument("--memory", action="store_true")
    parser.add_argument("--retain", type=int, default=2)
    parser.add_argument("--manifest", type=pathlib.Path)
    parser.add_argument("--worker-engine", choices=("DuckDB", "Polars", "DataFusion"))
    parser.add_argument("--worker-threads", type=int, default=1)
    args = parser.parse_args()
    if args.worker_engine:
        native_worker(args)
        return
    names = args.only.split(",")
    selected_engines = tuple(args.engines.split(","))
    if args.source == "csv" and "DataFusion" in selected_engines:
        parser.error("DataFusion 54.0.0 CSV null marker handling is unsupported; use parquet/table or omit DataFusion")
    if len(set(selected_engines)) != len(selected_engines) or not selected_engines or set(selected_engines)-set((*ENGINES, "DataFusion")):
        parser.error("invalid engine selection")
    workers = [int(item) for item in args.threads.split(",")]
    if set(names) - set(QUERIES) or min(workers) < 1 or args.rounds < 1 or args.runs < 1 or args.warmup < 0 or args.retain < 1 or args.peg_timeout < 0:
        parser.error("invalid query or measurement options")
    if not args.skip_build:
        subprocess.run(["go", "build", "-o", str(args.binary), "./scripts/workload_peg"], cwd=ROOT, check=True)
    limitations = list(LIMITATIONS)
    if "DataFusion" in selected_engines:
        limitations.append("DataFusion Python collects native Arrow record batches; row conversion is excluded from timing and included only in validation. Each execution includes SQL planning and collect. Target partitions cap query partitioning, not every runtime thread; its fair spill pool is 1 GiB and disk spilling is enabled.")
    report = {"paper": PAPER, "limitations": limitations, "source": args.source, "engines": selected_engines,
              "platform": platform.platform(), "processor": platform.machine(),
              "warmup": args.warmup, "runs": args.runs, "rounds": args.rounds,
              "peg_timeout_seconds": args.peg_timeout,
              "source_hashes": {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest() for path in (pathlib.Path(__file__).resolve(), ROOT/'scripts/workload_suite.py', ROOT/'scripts/workload_peg/main.go', ROOT/'scripts/workload_datafusion.py', ROOT/'scripts/compare_parquet_memory.py', ROOT/'scripts/process_memory_linux.py')},
              "fixtures": [], "records": [], "validation_errors": [], "query_failures": []}
    def save():
        if args.json:
            args.json.parent.mkdir(parents=True, exist_ok=True)
            args.json.write_text(json.dumps(report, indent=2) + "\n")
    for case in args.cases.split(","):
        manifest = make_fixture(args.scratch, case)
        fixture = json.loads(manifest.read_text())
        report["fixtures"].append(fixture)
        print(f"Fixture {case}: {fixture['files']['parquet']['bytes']/(1<<20):.2f} MiB; source={args.source}", flush=True)
        for round_index in range(args.rounds):
            engines = selected_engines if round_index % 2 == 0 else tuple(reversed(selected_engines))
            order = workers if round_index % 2 == 0 else list(reversed(workers))
            for count in order:
                records = []
                for engine in engines:
                    print(f"  round {round_index+1}, {count} workers, {engine}" + (" memory" if args.memory else ""), flush=True)
                    if args.memory:
                        for name in names:
                            record = measure_memory(args, args.binary, manifest, engine, count, name)
                            record.update({"case": case, "round": round_index+1})
                            records.append(record)
                    else:
                        env = {**os.environ, "POLARS_MAX_THREADS": str(count), "GOMAXPROCS": str(count)}
                        result = subprocess.run(command(args, args.binary, engine, manifest, count, args.only), cwd=ROOT, env=env, text=True, capture_output=True, timeout=1800, check=True)
                        record = json.loads(result.stdout)
                        record.update({"case": case, "round": round_index+1})
                        records.append(record)
                if args.memory:
                    errors = []
                    for name in names:
                        errors.extend(validate([r for r in records if r["query"] == name], fixture, [name]))
                else:
                    errors = validate(records, fixture, names)
                report["records"].extend(records)
                for record in records:
                    for entry in record["queries"]:
                        if entry.get("error"):
                            failure = {"case": case, "workers": count, "round": round_index+1, "engine": record["engine"], "query": entry["name"], "error": entry["error"], "code": entry.get("error_code")}
                            report["query_failures"].append(failure)
                            print(f"  QUERY FAILURE: {entry['name']}/{record['engine']}: {entry['error']} (code {entry.get('error_code')})", flush=True)
                report["validation_errors"].extend({"case": case, "workers": count, "round": round_index+1, "error": error} for error in errors)
                for error in errors:
                    print("  VALIDATION ERROR: " + error, flush=True)
                save()
    if not args.memory:
        for case in args.cases.split(","):
            for count in workers:
                print(f"\n{case}, {count} workers: milliseconds {' / '.join(selected_engines)}", flush=True)
                for name in names:
                    medians = []
                    for engine in selected_engines:
                        samples = [sample for record in report["records"] if record['case'] == case and record['workers'] == count and record['engine'] == engine for query in record['queries'] if query['name'] == name and not query.get('error') for sample in query['times_ns']]
                        medians.append(f"{statistics.median(samples)/1e6:.3f}" if samples else "ERROR")
                    print(f"  {name:25s} {' / '.join(medians)}", flush=True)
    save()
    print(f"Validation: {len(report['validation_errors'])} mismatches; {len(report['query_failures'])} query failures; {len(report['records'])} worker records", flush=True)
    if report["validation_errors"]:
        raise SystemExit(1)
    if report["query_failures"]:
        raise SystemExit(2)


if __name__ == "__main__":
    main()
