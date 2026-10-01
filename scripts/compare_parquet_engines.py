"""Compare prepared peGosus queries with DuckDB and Polars on one Parquet file.

Example (DuckDB 1.5.6 and Polars 1.34.0 must be installed in the Python environment):
    python3 scripts/compare_parquet_engines.py --repeat 64 --runs 7

The source is the ignored paired fixture produced by prepare_io_testdata.py.
This is a local microbenchmark, not an official ClickBench run.
peGosus uses explicit Select stages to enable scan projection pruning.
"""

import argparse
import json
import math
import os
import pathlib
import platform
import statistics
import subprocess
import tempfile
import time


ROOT = pathlib.Path(__file__).resolve().parents[1]
SOURCE = ROOT / "testdata" / "paired" / "scan_snappy.parquet"


def check_results(reference, candidate, name):
    assert len(reference) == len(candidate), (name, reference, candidate)
    for expected, observed in zip(reference, candidate):
        assert len(expected) == len(observed), (name, expected, observed)
        for x, y in zip(expected, observed):
            if isinstance(x, float) and isinstance(y, float):
                assert math.isclose(x, y, rel_tol=1e-10, abs_tol=1e-7), (name, x, y)
            else:
                assert x == y, (name, reference, candidate)


def measure(name, operation, warmup, runs):
    samples = []
    rows = None
    for index in range(warmup + runs):
        start = time.perf_counter_ns()
        rows = [list(row) for row in operation()]
        duration = time.perf_counter_ns() - start
        if index >= warmup:
            samples.append(duration)
    return {"name": name, "times_ns": samples, "rows": rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=pathlib.Path, default=SOURCE)
    parser.add_argument("--repeat", type=int, default=64)
    parser.add_argument("--runs", type=int, default=7)
    parser.add_argument("--warmup", type=int, default=2)
    parser.add_argument("--threads", type=int, default=1)
    parser.add_argument("--scratch", type=pathlib.Path)
    parser.add_argument("--memory", action="store_true", help="measure peGosus heap and allocator reuse in separate untimed passes")
    parser.add_argument("--retain", type=int, default=4, help="result retention window for memory measurements")
    parser.add_argument("--json", type=pathlib.Path, help="save raw timings and results as JSON")
    parser.add_argument("--profile-query", choices=["filter_count", "selective_count", "group_sum", "top50", "string_filter_sum", "square_sum"], help="profile one peGosus query after timing")
    args = parser.parse_args()
    if args.repeat < 1 or args.runs < 1 or args.warmup < 0 or args.threads < 1 or args.retain < 1:
        parser.error("repeat, runs, and threads must be positive")
    os.environ["POLARS_MAX_THREADS"] = str(args.threads)
    import duckdb
    import polars as pl

    if not args.source.is_file():
        parser.error("run scripts/prepare_io_testdata.py to create the paired Parquet source")
    with tempfile.TemporaryDirectory(prefix="peg-compare-", dir=args.scratch) as directory:
        output = pathlib.Path(directory) / "bench.parquet"
        binary = pathlib.Path(directory) / "peg-bench"
        connection = duckdb.connect()
        connection.execute(f"PRAGMA threads={args.threads}")
        source_rows = connection.execute("SELECT count(*) FROM read_parquet(?)", [str(args.source)]).fetchone()[0]
        output_sql = str(output).replace("'", "''")
        connection.execute(
            f"""COPY (
                SELECT (id + rep * ?)::INTEGER AS id, seq, measure, score,
                       active, day, observed, category, message
                FROM read_parquet(?) CROSS JOIN range(?) AS r(rep)
                ORDER BY id
            ) TO '{output_sql}' (FORMAT PARQUET, COMPRESSION SNAPPY, ROW_GROUP_SIZE 65536)""",
            [source_rows, str(args.source), args.repeat],
        )
        rows = source_rows * args.repeat
        print(f"Data: {rows:,} rows, {output.stat().st_size / (1 << 20):.1f} MiB Parquet", flush=True)
        subprocess.run(["go", "build", "-o", str(binary), "./scripts/compare_parquet_peg.go"], cwd=ROOT, check=True)
        peg_report = json.loads(subprocess.check_output([
            str(binary), "-parquet", str(output), "-rows", str(rows), "-runs", str(args.runs),
            "-warmup", str(args.warmup), "-workers", str(args.threads),
            *(["-memory", "-retain", str(args.retain)] if args.memory else []),
        ], cwd=ROOT, text=True))

        file_sql = "read_parquet('" + str(output).replace("'", "''") + "')"
        sql = {
            "filter_count": f"SELECT count(*) FROM {file_sql} WHERE id >= 0 AND active",
            "selective_count": f"SELECT count(*) FROM {file_sql} WHERE id >= {rows - 4000} AND active",
            "group_sum": f"SELECT category, sum(id) FROM {file_sql} WHERE id >= {rows // 4} GROUP BY category ORDER BY category ASC NULLS LAST",
            "top50": f"SELECT id, score FROM {file_sql} WHERE score IS NOT NULL ORDER BY score DESC NULLS LAST, id ASC LIMIT 50",
            "string_filter_sum": f"SELECT sum(id) FROM {file_sql} WHERE category = 'north' AND score > -23.0",
            "square_sum": f"SELECT sum(score * score) FROM {file_sql} WHERE score IS NOT NULL",
        }
        duck_report = {
            "engine": "DuckDB", "version": duckdb.__version__, "threads": args.threads,
            "queries": [measure(name, lambda query=query: connection.execute(query).fetchall(), args.warmup, args.runs)
                        for name, query in sql.items()],
        }

        scan = pl.scan_parquet(output)
        lazy = {
            "filter_count": scan.filter((pl.col("id") >= 0) & pl.col("active")).select(pl.len()),
            "selective_count": scan.filter((pl.col("id") >= rows - 4000) & pl.col("active")).select(pl.len()),
            "group_sum": scan.filter(pl.col("id") >= rows // 4).group_by("category").agg(pl.col("id").cast(pl.Int64).sum()).sort("category", nulls_last=True),
            "top50": scan.select("id", "score").filter(pl.col("score").is_not_null()).sort(["score", "id"], descending=[True, False], nulls_last=True).head(50),
            "string_filter_sum": scan.filter((pl.col("category") == "north") & (pl.col("score") > -23.0)).select(pl.col("id").cast(pl.Int64).sum()),
            "square_sum": scan.filter(pl.col("score").is_not_null()).select((pl.col("score") * pl.col("score")).sum()),
        }
        polars_report = {
            "engine": "Polars", "version": pl.__version__, "threads": pl.thread_pool_size(),
            "queries": [measure(name, lambda query=query: query.collect().rows(), args.warmup, args.runs)
                        for name, query in lazy.items()],
        }
        reports = [peg_report, duck_report, polars_report]
        expected = {item["name"]: item["rows"] for item in duck_report["queries"]}
        failures = {}
        for report in reports:
            for item in report["queries"]:
                try:
                    check_results(expected[item["name"]], item["rows"], f"{report['engine']}/{item['name']}")
                except AssertionError as error:
                    failures[(report["engine"], item["name"])] = str(error)

        print(f"Machine: {platform.platform()} | threads/worker: {args.threads} | warmups: {args.warmup} | runs: {args.runs}")
        if failures:
            print("Result mismatches; invalid timings are marked INVALID:")
            for (engine, name), detail in failures.items():
                print(f"  {engine}/{name}: {detail}")
        else:
            print("Validated equal results for all queries (floating results compared with 1e-10 relative tolerance).")
        print(f"{'query':22} {'peGosus':>12} {'DuckDB':>12} {'Polars':>12}")
        for name in sql:
            medians = [statistics.median(next(q for q in report["queries"] if q["name"] == name)["times_ns"]) / 1e6 for report in reports]
            formatted = ["INVALID" if (report["engine"], name) in failures else f"{median:.2f} ms" for report, median in zip(reports, medians)]
            print(f"{name:22} {formatted[0]:>12} {formatted[1]:>12} {formatted[2]:>12}")
        print("peGosus phases (median milliseconds; total above includes result collection and release):")
        print(f"{'query':22} {'prepare':>10} {'execute':>10} {'collect':>10} {'release':>10}")
        for item in peg_report["queries"]:
            phases = [item["prepare_ns"] / 1e6] + [statistics.median(item[key]) / 1e6 for key in ("exec_ns", "collect_ns", "release_ns")]
            print(f"{item['name']:22}" + "".join(f"{value:10.3f}" for value in phases))
        if args.memory:
            print("peGosus memory (separate passes; heap bytes include slab storage; live is segment capacity after release, not payload or peak bytes):")
            print(f"{'query/mode':30} {'alloc KiB/op':>13} {'allocs/op':>11} {'GCs':>5} {'post-GC MiB':>12} {'slab MiB':>10} {'live KiB':>10}")
            for item in peg_report["queries"]:
                for entry in item["memory"]:
                    allocated = statistics.median(sample["allocated_bytes"] for sample in entry["samples"]) / 1024
                    allocations = statistics.median(sample["allocations"] for sample in entry["samples"])
                    gcs = sum(sample["gcs"] for sample in entry["samples"])
                    post_gc = entry["after_release_gc"]["heap_alloc_bytes"] / (1 << 20)
                    usage = entry["after_release_allocator"]
                    slab = (usage["GeneralCapacity"] + usage["ScratchCapacity"]) / (1 << 20)
                    live = (usage["GeneralUsed"] + usage["ScratchUsed"]) / 1024
                    print(f"{item['name'] + '/' + entry['mode']:30} {allocated:13.1f} {allocations:11.0f} {gcs:5} {post_gc:12.2f} {slab:10.2f} {live:10.1f}")
        print("Versions:", ", ".join(f"{report['engine']} {report['version']}" for report in reports))
        if args.json:
            args.json.write_text(json.dumps({"source_rows": rows, "parquet_bytes": output.stat().st_size, "reports": reports, "failures": {f"{engine}/{name}": detail for (engine, name), detail in failures.items()}}, indent=2) + "\n")
            print("Raw timings and results:", args.json)
        if args.profile_query:
            profile = pathlib.Path(directory) / (args.profile_query + ".prof")
            subprocess.run([str(binary), "-parquet", str(output), "-rows", str(rows), "-only", args.profile_query, "-runs", "30", "-warmup", "2", "-workers", str(args.threads), "-cpuprofile", str(profile)], cwd=ROOT, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["go", "tool", "pprof", "-top", "-nodecount=15", str(profile)], cwd=ROOT, check=True)
        if failures:
            raise SystemExit(1)


if __name__ == "__main__":
    main()
