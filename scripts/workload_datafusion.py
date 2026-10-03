"""Optional DataFusion adapter; native Arrow collection is the timed output.

Install requirements-datafusion.txt separately. SQL planning occurs on each
execution; Arrow-to-Python row conversion is only used during validation.
"""
from workload_suite import sql_query


def datafusion_setup(args, fixture):
    import datafusion
    import pyarrow as pa

    config = datafusion.SessionConfig().with_target_partitions(args.worker_threads)
    runtime = datafusion.RuntimeEnvBuilder().with_disk_manager_os().with_fair_spill_pool(1 << 30)
    context = datafusion.SessionContext(config, runtime)
    for table, path_key in (("facts", "parquet"), ("dimension", "dimension")):
        path = fixture[path_key]
        if args.source == "table":
            batches = context.read_parquet(path).collect()
            # Preserve source order within each partition. Batch boundaries and
            # partition count affect the scan but cannot alter SQL row semantics.
            partitions = [batches[i::args.worker_threads] for i in range(args.worker_threads)]
            context.register_record_batches(table, [part for part in partitions if part])
        elif args.source == "csv":
            raise ValueError("DataFusion 54.0.0 CSV null_regex does not preserve this fixture's \\N null marker; select parquet/table or omit DataFusion")
        else:
            context.register_parquet(table, path)

    def make_operation(name):
        sql = datafusion_sql(name, fixture["rows"])
        return lambda: context.sql(sql).collect()

    def convert(batches):
        for batch in batches:
            columns = [batch.column(i).to_pylist() for i in range(batch.num_columns)]
            yield from zip(*columns)

    version = f"{datafusion.__version__} (pyarrow {pa.__version__})"
    return make_operation, convert, version


def datafusion_sql(name, rows):
    sql = sql_query(name, rows).replace(" SEMI JOIN ", " LEFT SEMI JOIN ").replace(" ANTI JOIN ", " LEFT ANTI JOIN ")
    if name == "date_expression_group":
        sql = sql.replace("year(day)", "CAST(EXTRACT(YEAR FROM day) AS BIGINT)")
        sql = sql.replace("month(day)", "CAST(EXTRACT(MONTH FROM day) AS BIGINT)")
    return sql
