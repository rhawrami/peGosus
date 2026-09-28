"""Prepare ignored Parquet corpus files and paired CSV/Parquet scan data.

Requires DuckDB 1.5.6: python3 -m pip install duckdb==1.5.6
Run from any directory: python3 scripts/prepare_io_testdata.py
"""

import hashlib
import json
import pathlib
import urllib.request

import duckdb


ROOT = pathlib.Path(__file__).resolve().parents[1] / "testdata"
CORPUS_REVISION = "56653c437c8092f704a092d0d1d4e600124cd49f"
CORPUS_FILES = {
    "byte_array_decimal.parquet": "9e3ccb253adc5881521b952f7b621954551df1e48dfda19e9b02126aca9b127d",
    "data_index_bloom_encoding_stats.parquet": "66d53151197819919343972c997837845503ce82665c9c4481c866ef57dde0eb",
    "datapage_v1-corrupt-checksum.parquet": "b337106431c826e3326ab8fecfa5560688aa57549fd46e0fa7cfcf99cd4e2c9e",
    "datapage_v1-snappy-compressed-checksum.parquet": "f06df378ad412ace763d129f317c52236230b2fb24073c32d2c2d5fc1ef9d697",
    "datapage_v2_empty_datapage.snappy.parquet": "c93d4d6ace5ac92d3bc0ba04f44077f6fb7019cbe4f3982f204d666653fc0514",
    "datapage_v2.snappy.parquet": "44f29191b5fa8cfe0ab848495bd8ef89344ac0d8f87b3dff12e267631e2b5c03",
    "rle-dict-snappy-checksum.parquet": "bb9de5fd817da4c403992ce2ee2c9dc93d4444f936e1a64722c65dd98d208578",
}
ROWS = 16387


def sha256(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> None:
    if duckdb.__version__ != "1.5.6":
        raise RuntimeError("fixture writer must be DuckDB 1.5.6")

    corpus = ROOT / "parquet-testing"
    paired = ROOT / "paired"
    corpus.mkdir(parents=True, exist_ok=True)
    paired.mkdir(parents=True, exist_ok=True)
    for name, expected in CORPUS_FILES.items():
        target = corpus / name
        if not target.exists():
            url = f"https://raw.githubusercontent.com/apache/parquet-testing/{CORPUS_REVISION}/data/{name}"
            with urllib.request.urlopen(url, timeout=60) as response:
                target.write_bytes(response.read())
        if sha256(target) != expected:
            raise RuntimeError(f"checksum mismatch: {target}")

    conn = duckdb.connect()
    conn.execute("SET TimeZone='UTC'")
    conn.execute(
        f"""CREATE TABLE source AS SELECT
            i::INTEGER AS id,
            (i * 1000003 - 7000000000)::BIGINT AS seq,
            CASE WHEN i % 29 = 0 THEN NULL ELSE ((i % 1001) - 500)::FLOAT / 16 END AS measure,
            CASE WHEN i % 23 = 0 THEN NULL ELSE ((i % 65521) - 32760)::DOUBLE / 1024 END AS score,
            (i % 3 = 0) AS active,
            DATE '2020-01-01' + (i % 367)::INTEGER AS day,
            TIMESTAMPTZ '2020-01-01 00:00:00+00' + i::INTEGER * INTERVAL '7 microseconds' AS observed,
            CASE WHEN i % 19 = 0 THEN NULL ELSE ['north', 'south', 'east', 'west', 'ø'][1 + (i % 5)] END AS category,
            CASE WHEN i % 17 = 0 THEN NULL
                 WHEN i % 101 = 0 THEN 'quoted, "field"' || chr(10) || repeat('z', 72)
                 ELSE repeat('payload-' || (i % 997)::VARCHAR || ';', 3 + (i % 9)) END AS message
        FROM range({ROWS}) AS t(i)"""
    )

    fields = "id, seq, measure, score, active, day, observed, category, message"
    for codec, filename in (("snappy", "scan_snappy.parquet"), ("uncompressed", "scan_plain.parquet")):
        (paired / filename).unlink(missing_ok=True)
        conn.execute(
            f"COPY (SELECT {fields} FROM source ORDER BY id) TO ? "
            f"(FORMAT PARQUET, COMPRESSION {codec}, ROW_GROUP_SIZE 2048)",
            [str(paired / filename)],
        )
    (paired / "scan.csv").unlink(missing_ok=True)
    conn.execute(
        """COPY (SELECT id, seq, measure, score, active, day,
                   strftime(observed, '%Y-%m-%dT%H:%M:%S.%fZ') AS observed,
                   category, message FROM source ORDER BY id)
           TO ? (FORMAT CSV, HEADER TRUE, NULL '\\N')""",
        [str(paired / "scan.csv")],
    )
    count, total, non_null = conn.execute(
        "SELECT count(*), sum(id), count(message) FROM source WHERE id >= 1000 AND active"
    ).fetchone()
    selective_count, selective_total, selective_non_null = conn.execute(
        "SELECT count(*), sum(id), count(message) FROM source WHERE id >= 16000 AND active"
    ).fetchone()
    manifest = {
        "writer": f"DuckDB {duckdb.__version__}",
        "rows": ROWS,
        "schema": [
            {"name": name, "type": kind}
            for name, kind in zip(
                ["id", "seq", "measure", "score", "active", "day", "observed", "category", "message"],
                ["INT32", "INT64", "FLOAT32", "FLOAT64", "BOOL", "DATE", "TIMESTAMPTZ", "STRING", "STRING"],
            )
        ],
        "filter_id_ge_1000_and_active": {"rows": count, "id_sum": total, "non_null_message": non_null},
        "filter_id_ge_16000_and_active": {"rows": selective_count, "id_sum": selective_total, "non_null_message": selective_non_null},
        "corpus_revision": CORPUS_REVISION,
        "files_sha256": {
            str(path.relative_to(ROOT)): sha256(path)
            for path in (list(corpus / name for name in CORPUS_FILES) + list(paired / name for name in ("scan_snappy.parquet", "scan_plain.parquet", "scan.csv")))
        },
    }
    (ROOT / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {ROWS} paired rows and {len(CORPUS_FILES)} pinned corpus files to {ROOT}")


if __name__ == "__main__":
    main()
