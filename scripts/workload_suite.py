"""Synthetic BI workloads informed by Get Real (DBTest 2018).

Values use dyadic arithmetic so exact cross-engine result fingerprints are
independent of floating-point reduction order. This deliberately excludes
NaN/Inf and does not measure their performance; package tests cover semantics.
"""
import datetime
import hashlib
import json
import pathlib
import struct

PAPER = "https://www.cs.cit.tum.de/fileadmin/w00cfj/dis/papers/getreal.pdf"
QUERIES = (
    "filter_count", "selective_count", "group_sum", "top10", "top50", "top1000",
    "string_filter_sum", "square_sum", "group_day_sum", "full_sort_numeric",
    "full_sort_string", "wide_materialize", "preview_limit", "range_revenue",
    "multi_group_report", "high_card_int_group", "high_card_string_group",
    "distinct_strings", "count_distinct", "string_clean_group", "string_key_report",
    "case_coalesce_sum", "date_expression_group", "empty_filter_sum",
    "selectivity_1", "selectivity_50", "selectivity_99", "join_int_report",
    "join_string_report", "join_left_report", "join_semi_count", "join_anti_count",
    "grouped_self_join", "expression_width_8", "expression_width_32",
    "expression_width_128", "repeated_expression_32",
)
CASES = ("balanced:512", "balanced:65536", "balanced:1048576",
         "skewed_nulls:65536", "plain_strings:65536")
WIDE = ("id", "category", "customer_code", "message", "flag_text", "fiscal_year",
        "price", "quantity", "score")
EPOCH = datetime.date(1970, 1, 1)
FIELDS = [("id", "int32", False), ("score", "float64", True),
          ("active", "bool", True), ("day", "date", False),
          ("category", "string", True), ("message", "string", True),
          ("customer_key", "int32", True), ("customer_code", "string", True),
          ("segment_key", "int32", True), ("segment_code", "string", True),
          ("flag_text", "string", True), ("fiscal_year", "string", False),
          ("price", "float64", True), ("quantity", "int32", True),
          ("discount", "float64", True), ("selectivity", "int32", False)]
DIM_FIELDS = [("segment_key", "int32", False), ("segment_code", "string", False),
              ("region", "string", False), ("enabled", "bool", False)]


def quote(value):
    return "'" + str(value).replace("'", "''") + "'"


def fingerprint(rows, name):
    digest = hashlib.sha256()
    total = xor = count = 0
    sample = []
    for row in rows:
        count += 1
        h = hashlib.sha256() if name == "wide_materialize" else digest
        h.update(b"[")
        for value in row:
            if value is None:
                token = b"\0"
            elif isinstance(value, str):
                data = value.encode("utf-8")
                token = b"s" + struct.pack("<Q", len(data)) + data
            elif isinstance(value, bool):
                token = b"b" + bytes([value])
            elif isinstance(value, float):
                token = b"f" + struct.pack("<d", 0.0 if value == 0 else value)
            else:
                if isinstance(value, datetime.datetime):
                    value = int((value - datetime.datetime(1970, 1, 1, tzinfo=value.tzinfo)).total_seconds() * 1000000)
                elif isinstance(value, datetime.date):
                    value = (value - EPOCH).days
                token = b"i" + str(value).encode("ascii") + b"\n"
            h.update(token)
        h.update(b"]")
        if name == "wide_materialize":
            number = int.from_bytes(h.digest(), "little")
            total = (total + number) % (1 << 256)
            xor ^= number
        if name == "preview_limit":
            sample.append(list(row))
    if name == "wide_materialize":
        digest.update(total.to_bytes(32, "little") + xor.to_bytes(32, "little") + struct.pack("<Q", count))
    result = {"rows": count, "sha256": digest.hexdigest()}
    if sample:
        result["sample"] = sample
    return result


def make_fixture(directory, case):
    import duckdb
    profile, row_count = case.split(":")
    rows = int(row_count)
    if profile not in ("balanced", "skewed_nulls", "plain_strings") or not 1 <= rows <= 100000000:
        raise ValueError("invalid fixture case")
    directory = pathlib.Path(directory).resolve() / f"{profile}-{rows}"
    directory.mkdir(parents=True, exist_ok=True)
    manifest_path = directory / "manifest.json"
    null_pct = 40 if profile == "skewed_nulls" else 5
    cardinality = rows * 3 // 4 + 1
    def nullable(expression, offset):
        return f"CASE WHEN (i*17+{offset})%100 < {null_pct} THEN NULL ELSE {expression} END"
    customer = f"(i*48271)%{cardinality}"
    segment = "(i*17)%536"
    cat = "(['north','south','east','west','central','island','Café','東京'])[(i%8)+1]"
    if profile == "skewed_nulls":
        cat = f"CASE WHEN i%100 < 85 THEN 'north' ELSE {cat} END"
    message = "'Event-' || printf('%04d', i%128) || ' Café 東京'"
    code = f"'cust-' || printf('%08d', {customer})"
    if profile == "plain_strings":
        message += " || ',quoted \"text\"' || chr(10) || repeat('Long UTF8 payload naïve 東京 ', 4)"
        code = "'Customer Café 東京 identifier-' || printf('%08d', " + customer + ")"
    columns = [
        "i::INTEGER AS id", nullable("((i*104729)%65536-32768)::DOUBLE/128", 3) + " AS score",
        nullable("i%3=0", 7) + " AS active", "DATE '1970-01-01' + (19000+(i*13)%365)::INTEGER AS day",
        nullable(cat, 11) + " AS category", nullable(message, 19) + " AS message",
        nullable(f"({customer})::INTEGER", 23) + " AS customer_key",
        nullable(code, 23) + " AS customer_code", nullable(f"({segment})::INTEGER", 31) + " AS segment_key",
        nullable(f"'seg-' || printf('%04d', {segment})", 31) + " AS segment_code",
        nullable("CASE WHEN i%29=0 THEN '?' WHEN i%2=0 THEN 'Y' ELSE 'N' END", 37) + " AS flag_text",
        "printf('%d/%02d', 2017+i%6, (18+i%6)) AS fiscal_year",
        nullable("(i%1024+1)::DOUBLE/8", 41) + " AS price",
        nullable("(CASE WHEN i%127=0 THEN 0 ELSE (1 << (i%4)) END)::INTEGER", 43) + " AS quantity",
        nullable("(i%4)::DOUBLE/8", 47) + " AS discount", "((i*37)%100)::INTEGER AS selectivity",
    ]
    sql = "SELECT " + ", ".join(columns) + f" FROM range({rows}) AS t(i)"
    dimension_sql = "SELECT i::INTEGER AS segment_key, 'seg-' || printf('%04d', i) AS segment_code, 'region-' || (i%8)::VARCHAR AS region, i%5!=0 AS enabled FROM range(512) AS t(i)"
    if manifest_path.exists():
        cached = json.loads(manifest_path.read_text())
        if cached.get("generator_sql") != sql or cached.get("dimension_sql") != dimension_sql:
            directory = directory.parent / (directory.name + "-" + hashlib.sha256((sql + dimension_sql).encode()).hexdigest()[:12])
            directory.mkdir(parents=True, exist_ok=True)
            manifest_path = directory / "manifest.json"
    if manifest_path.exists():
        cached = json.loads(manifest_path.read_text())
        for key, expected in cached["files"].items():
            if hashlib.sha256(pathlib.Path(cached[key]).read_bytes()).hexdigest() != expected["sha256"]:
                raise ValueError("cached fixture content changed; choose a fresh --scratch directory")
        return manifest_path
    paths = {"parquet": directory / "facts.parquet", "dimension": directory / "dimension.parquet",
             "csv": directory / "facts.csv", "dimension_csv": directory / "dimension.csv"}
    connection = duckdb.connect()
    connection.execute("SET threads=1")
    for key, query in (("parquet", sql), ("dimension", dimension_sql), ("csv", sql), ("dimension_csv", dimension_sql)):
        options = "FORMAT CSV, HEADER TRUE, NULL '\\N'" if key.endswith("csv") else "FORMAT PARQUET, COMPRESSION SNAPPY, ROW_GROUP_SIZE 65536"
        if profile == "plain_strings" and not key.endswith("csv"):
            options += ", STRING_DICTIONARY_PAGE_SIZE_LIMIT 1"
        connection.execute(f"COPY ({query}) TO {quote(paths[key])} ({options})")
    encodings = connection.execute("SELECT path_in_schema, list(distinct encodings), sum(num_values) FROM parquet_metadata(?) GROUP BY path_in_schema ORDER BY path_in_schema", [str(paths["parquet"])]).fetchall()
    result = {"case": case, "rows": rows, "profile": profile, "null_percentage": null_pct,
              "customer_domain": cardinality, "generator_sql": sql, "dimension_sql": dimension_sql,
              "fields": [{"name": name, "kind": kind, "nullable": nullable} for name, kind, nullable in FIELDS],
              "dimension_fields": [{"name": name, "kind": kind, "nullable": nullable} for name, kind, nullable in DIM_FIELDS],
              "encodings": encodings, "files": {key: {"bytes": value.stat().st_size, "sha256": hashlib.sha256(value.read_bytes()).hexdigest()} for key, value in paths.items()},
              **{key: str(value) for key, value in paths.items()}}
    connection.close()
    manifest_path.write_text(json.dumps(result, indent=2) + "\n")
    return manifest_path


def sql_query(name, rows):
    rev = "price * CAST(quantity AS DOUBLE) * (1.0 - discount)"
    queries = {
        "filter_count": "SELECT count(*) FROM facts WHERE id>=0 AND active",
        "selective_count": f"SELECT count(*) FROM facts WHERE id>={rows-4000} AND active",
        "group_sum": f"SELECT category, sum(id) FROM facts WHERE id>={rows//4} GROUP BY category ORDER BY category NULLS LAST",
        "string_filter_sum": "SELECT sum(id) FROM facts WHERE category='north' AND score > -23.0",
        "square_sum": "SELECT sum(score*score) FROM facts WHERE score IS NOT NULL",
        "group_day_sum": f"SELECT day, sum(id) FROM facts WHERE id>={rows//4} GROUP BY day ORDER BY day NULLS LAST",
        "full_sort_numeric": "SELECT id,score FROM facts WHERE score IS NOT NULL ORDER BY score DESC,id",
        "full_sort_string": "SELECT id,customer_code FROM facts ORDER BY customer_code NULLS LAST,id",
        "wide_materialize": "SELECT " + ",".join(WIDE) + " FROM facts",
        "preview_limit": "SELECT id,category,score FROM facts LIMIT 100",
        "range_revenue": f"SELECT sum({rev}),count(quantity),min(price),max(price) FROM facts WHERE day BETWEEN DATE '2022-01-08' AND DATE '2022-03-09'",
        "multi_group_report": f"SELECT category,flag_text,count(*),sum({rev}),avg(price),count(quantity) FROM facts GROUP BY category,flag_text ORDER BY category NULLS LAST,flag_text NULLS LAST",
        "distinct_strings": "SELECT count(*) FROM (SELECT DISTINCT customer_code FROM facts)",
        "count_distinct": "SELECT count(DISTINCT customer_code),count(DISTINCT category) FROM facts",
        "string_clean_group": "SELECT substr(replace(lower(coalesce(message,'missing')),'event','visit'),1,12) AS clean,sum(quantity),count(*) FROM facts GROUP BY clean ORDER BY clean",
        "string_key_report": f"SELECT fiscal_year,category,sum({rev}),count(*) FROM facts WHERE flag_text='Y' OR contains(category,'th') GROUP BY fiscal_year,category ORDER BY fiscal_year,category NULLS LAST",
        "case_coalesce_sum": "SELECT sum(coalesce(CASE WHEN quantity=0 OR quantity IS NULL THEN NULL ELSE price/CAST(quantity AS DOUBLE) END,0.0)) FROM facts",
        "date_expression_group": f"SELECT year(day),month(day),sum({rev}),count(*) FROM facts GROUP BY year(day),month(day) ORDER BY year(day),month(day)",
        "empty_filter_sum": "SELECT sum(quantity),count(*) FROM facts WHERE id<0",
        "grouped_self_join": "SELECT count(*),sum(f.quantity),sum(g.group_total) FROM facts f JOIN (SELECT category,sum(quantity) AS group_total FROM facts GROUP BY category) g ON f.category=g.category",
    }
    if name in queries:
        return queries[name]
    if name.startswith("top"):
        return queries["full_sort_numeric"] + " LIMIT " + name[3:]
    if name.startswith("high_card_"):
        key = "customer_key" if name == "high_card_int_group" else "customer_code"
        return f"SELECT count(*),sum(total) FROM (SELECT {key},sum(quantity) AS total FROM facts GROUP BY {key})"
    if name.startswith("selectivity_"):
        return f"SELECT sum({rev}),count(*) FROM facts WHERE selectivity<{int(name.split('_')[-1])}"
    if name.startswith("join_"):
        key = "segment_code" if name == "join_string_report" else "segment_key"
        kind = {"join_left_report": "LEFT", "join_semi_count": "SEMI", "join_anti_count": "ANTI"}.get(name, "INNER")
        base = f"FROM facts f {kind} JOIN (SELECT * FROM dimension WHERE enabled) d ON f.{key}=d.{key}"
        if kind in ("SEMI", "ANTI"):
            return "SELECT count(*) " + base
        return "SELECT d.region,sum(f.quantity),count(*) " + base + " GROUP BY d.region ORDER BY d.region NULLS LAST"
    if name.startswith(("expression_width_", "repeated_expression_")):
        width = int(name.split("_")[-1])
        expressions = [rev if name.startswith("repeated") else f"CASE WHEN quantity>={i%8+1} THEN ({rev})*{float(i%5+1)} ELSE coalesce({rev},0.0)+{i/8} END" for i in range(width)]
        return "SELECT " + ",".join(f"sum({expr}) AS m{i}" for i, expr in enumerate(expressions)) + " FROM facts"
    raise ValueError(name)


def polars_query(name, scan, dimension, rows):
    import polars as pl
    c = pl.col
    rev = c("price") * c("quantity").cast(pl.Float64) * (1.0 - c("discount"))
    def total(expr, alias="total"):
        if isinstance(expr, str):
            expr = c(expr).cast(pl.Int64)
        return pl.when(expr.count() > 0).then(expr.sum()).otherwise(None).alias(alias)
    if name in ("filter_count", "selective_count"):
        return scan.filter((c("id") >= (0 if name == "filter_count" else rows-4000)) & c("active")).select(pl.len())
    if name in ("group_sum", "group_day_sum"):
        key = "category" if name == "group_sum" else "day"
        return scan.filter(c("id") >= rows//4).group_by(key).agg(total("id")).sort(key, nulls_last=True)
    if name.startswith("top") or name == "full_sort_numeric":
        plan = scan.filter(c("score").is_not_null()).select("id", "score").sort(["score", "id"], descending=[True, False], nulls_last=True)
        return plan.head(int(name[3:])) if name.startswith("top") else plan
    if name == "full_sort_string":
        return scan.select("id", "customer_code").sort(["customer_code", "id"], nulls_last=True)
    if name == "wide_materialize":
        return scan.select(*WIDE)
    if name == "preview_limit":
        return scan.select("id", "category", "score").head(100)
    if name == "string_filter_sum":
        return scan.filter((c("category") == "north") & (c("score") > -23.0)).select(total("id"))
    if name == "square_sum":
        return scan.filter(c("score").is_not_null()).select(total(c("score") * c("score")))
    if name == "range_revenue":
        return scan.filter(c("day").is_between(EPOCH + datetime.timedelta(days=19000), EPOCH + datetime.timedelta(days=19060))).select(total(rev, "revenue"), c("quantity").count(), c("price").min().alias("min_price"), c("price").max().alias("max_price"))
    if name == "multi_group_report":
        return scan.group_by("category", "flag_text").agg(pl.len(), total(rev, "revenue"), c("price").mean(), c("quantity").count()).sort(["category", "flag_text"], nulls_last=True)
    if name.startswith("high_card_"):
        key = "customer_key" if name == "high_card_int_group" else "customer_code"
        return scan.group_by(key).agg(total("quantity")).select(pl.len(), total(c("total")))
    if name == "distinct_strings":
        return scan.select("customer_code").unique().select(pl.len())
    if name == "count_distinct":
        return scan.select(c("customer_code").drop_nulls().n_unique(), c("category").drop_nulls().n_unique())
    if name == "string_clean_group":
        clean = c("message").fill_null("missing").str.to_lowercase().str.replace_all("event", "visit", literal=True).str.slice(0, 12).alias("clean")
        return scan.select(clean, c("quantity")).group_by("clean").agg(total("quantity"), pl.len()).sort("clean")
    if name == "string_key_report":
        return scan.filter((c("flag_text") == "Y") | c("category").str.contains("th", literal=True)).group_by("fiscal_year", "category").agg(total(rev), pl.len()).sort(["fiscal_year", "category"], nulls_last=True)
    if name == "case_coalesce_sum":
        expr = pl.when((c("quantity") == 0) | c("quantity").is_null()).then(None).otherwise(c("price") / c("quantity").cast(pl.Float64)).fill_null(0.0)
        return scan.select(total(expr))
    if name == "date_expression_group":
        return scan.with_columns(c("day").dt.year().cast(pl.Int64).alias("year"), c("day").dt.month().alias("month")).group_by("year", "month").agg(total(rev), pl.len()).sort(["year", "month"])
    if name == "empty_filter_sum":
        return scan.filter(c("id") < 0).select(total("quantity"), pl.len())
    if name.startswith("selectivity_"):
        return scan.filter(c("selectivity") < int(name.split("_")[-1])).select(total(rev), pl.len())
    if name.startswith("join_"):
        key = "segment_code" if name == "join_string_report" else "segment_key"
        how = {"join_left_report": "left", "join_semi_count": "semi", "join_anti_count": "anti"}.get(name, "inner")
        joined = scan.select(key, "quantity").join(dimension.filter(c("enabled")).select(key, "region"), on=key, how=how)
        return joined.select(pl.len()) if how in ("semi", "anti") else joined.group_by("region").agg(total("quantity"), pl.len()).sort("region", nulls_last=True)
    if name == "grouped_self_join":
        grouped = scan.group_by("category").agg(total("quantity", "group_total"))
        return scan.select("category", "quantity").join(grouped, on="category").select(pl.len(), total("quantity", "quantity"), total(c("group_total"), "joined_total"))
    if name.startswith(("expression_width_", "repeated_expression_")):
        width = int(name.split("_")[-1])
        expressions = [rev if name.startswith("repeated") else pl.when(c("quantity") >= i%8+1).then(rev*(i%5+1)).otherwise(rev.fill_null(0.0)+i/8) for i in range(width)]
        return scan.select([total(expr, f"m{i}") for i, expr in enumerate(expressions)])
    raise ValueError(name)
