"""Alternate two frozen peGosus runners over identical checksummed fixtures."""
import argparse
import datetime
import hashlib
import json
import os
import pathlib
import platform
import subprocess

from workload_suite import CASES, QUERIES, make_fixture

ROOT = pathlib.Path(__file__).resolve().parents[1]
DEFAULT_QUERIES = "join_int_report,join_string_report,join_left_report,join_semi_count,join_anti_count,grouped_self_join,full_sort_numeric,full_sort_string,distinct_strings,count_distinct,multi_group_report,group_sum,top50,wide_materialize"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def source_hashes():
    paths = [ROOT / "go.mod", ROOT / "go.sum"]
    paths.extend(p for base in (ROOT / "pkg", ROOT / "scripts") for p in base.rglob("*")
                 if p.is_file() and p.suffix in (".go", ".s", ".py", ".b64") and "__pycache__" not in p.parts)
    return {str(p.relative_to(ROOT)): sha(p) for p in sorted(paths)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", required=True, type=pathlib.Path)
    parser.add_argument("--after", required=True, type=pathlib.Path)
    parser.add_argument("--scratch", required=True, type=pathlib.Path)
    parser.add_argument("--json", required=True, type=pathlib.Path)
    parser.add_argument("--cases", default=",".join(CASES))
    parser.add_argument("--threads", default="1,4")
    parser.add_argument("--rounds", type=int, default=2)
    parser.add_argument("--warmup", type=int, default=2)
    parser.add_argument("--runs", type=int, default=5)
    parser.add_argument("--only", default=DEFAULT_QUERIES)
    parser.add_argument("--table-million", action="store_true")
    args = parser.parse_args()
    names = args.only.split(",")
    workers = [int(item) for item in args.threads.split(",")]
    if set(names) - set(QUERIES) or min(workers) < 1 or min(args.runs, args.rounds) < 1 or args.warmup < 0:
        parser.error("invalid measurement options")
    binaries = {"before": args.before.resolve(), "after": args.after.resolve()}
    initial_sources = source_hashes()
    report = {"started_at_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "platform": platform.platform(), "binary_hashes": {v: sha(p) for v, p in binaries.items()},
              "source_hashes": initial_sources, "rounds": args.rounds, "warmup": args.warmup,
              "runs": args.runs, "records": [], "validation_errors": [], "query_failures": [],
              "limitations": ["Synthetic warm-cache native Result workloads; prepare/loading/hash validation excluded.",
                              "Variants and worker order reverse between rounds; query order is fixed.",
                              "Pure Go, GOAMD64=v1, normal build; 1 GiB query budget; ten-second per-query deadline.",
                              "This suite does not activate disk spilling or experimental hash controls."]}

    def save():
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(json.dumps(report, indent=2) + "\n")

    cases = [(case, "parquet") for case in args.cases.split(",")]
    if args.table_million:
        cases.append(("balanced:1048576", "table"))
    for case, source in cases:
        manifest = make_fixture(args.scratch, case)
        for round_index in range(args.rounds):
            for count in workers if round_index % 2 == 0 else workers[::-1]:
                pair = []
                for variant in binaries if round_index % 2 == 0 else list(binaries)[::-1]:
                    print(f"{case} {source} round={round_index+1} workers={count} {variant}", flush=True)
                    command = [str(binaries[variant]), "-manifest", str(manifest), "-source", source,
                               "-workers", str(count), "-only", args.only, "-runs", str(args.runs),
                               "-warmup", str(args.warmup), "-budget", str(1 << 30), "-timeout", "10s"]
                    result = subprocess.run(command, cwd=ROOT, env={**os.environ, "GOMAXPROCS": str(count)},
                                            text=True, capture_output=True, check=True, timeout=1800)
                    record = json.loads(result.stdout)
                    record.update(variant=variant, case=case, round=round_index+1, command=command,
                                  manifest_sha256=sha(manifest))
                    pair.append(record)
                    report["records"].append(record)
                    for query in record["queries"]:
                        if query.get("error"):
                            report["query_failures"].append({"case": case, "source": source, "workers": count,
                                                             "variant": variant, **query})
                    save()
                by_variant = {record["variant"]: {q["name"]: q for q in record["queries"]} for record in pair}
                for name in names:
                    before, after = by_variant["before"][name], by_variant["after"][name]
                    if not before.get("error") and not after.get("error") and before["signature"] != after["signature"]:
                        report["validation_errors"].append({"case": case, "source": source, "workers": count,
                                                            "round": round_index+1, "name": name})
                save()
    report["source_unchanged"] = source_hashes() == initial_sources
    report["finished_at_utc"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    save()
    print(f"{len(report['records'])} records; {len(report['validation_errors'])} mismatches; {len(report['query_failures'])} failures", flush=True)
    if report["validation_errors"] or report["query_failures"] or not report["source_unchanged"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
