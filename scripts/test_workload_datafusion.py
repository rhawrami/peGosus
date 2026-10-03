import argparse
import importlib.util
import json
import tempfile
import unittest

from workload_datafusion import datafusion_setup, datafusion_sql
from workload_suite import QUERIES, fingerprint, make_fixture, quote, sql_query


@unittest.skipUnless(importlib.util.find_spec("datafusion") and importlib.util.find_spec("duckdb"), "install optional DataFusion and default benchmark requirements")
class DataFusionWorkloadTests(unittest.TestCase):
    def test_all_queries_match_duckdb_on_null_and_string_profiles(self):
        import duckdb
        with tempfile.TemporaryDirectory() as directory:
            for profile in ("balanced", "skewed_nulls", "plain_strings"):
                manifest = make_fixture(directory, profile + ":512")
                fixture = json.loads(manifest.read_text())
                connection = duckdb.connect()
                try:
                    for name, key in (("facts", "parquet"), ("dimension", "dimension")):
                        connection.execute(f"CREATE VIEW {name} AS SELECT * FROM read_parquet({quote(fixture[key])})")
                    expected = {name: fingerprint(connection.execute(sql_query(name, 512)).fetchall(), name) for name in QUERIES}
                    for source in ("parquet", "table"):
                        make, convert, _ = datafusion_setup(argparse.Namespace(worker_threads=4, source=source), fixture)
                        for name in QUERIES:
                            with self.subTest(profile=profile, source=source, query=name):
                                result = make(name)()
                                signature = fingerprint(convert(result), name)
                                if name == "preview_limit":
                                    sample = signature["sample"]
                                    ids = [row[0] for row in sample]
                                    self.assertEqual(len(ids), 100)
                                    self.assertEqual(len(set(ids)), 100)
                                    reference = dict((row[0], list(row)) for row in connection.execute(f"SELECT id,category,score FROM facts WHERE id IN ({','.join(map(str, ids))})").fetchall())
                                    self.assertTrue(all(reference[row[0]] == row for row in sample))
                                else:
                                    self.assertEqual(signature, expected[name])
                finally:
                    connection.close()

    def test_csv_is_rejected_instead_of_ranking_wrong_nulls(self):
        with self.assertRaisesRegex(ValueError, "null_regex"):
            datafusion_setup(argparse.Namespace(worker_threads=1, source="csv"), {"parquet": "unused"})

    def test_sql_dialect_preserves_left_side_semantics(self):
        self.assertIn(" LEFT SEMI JOIN ", datafusion_sql("join_semi_count", 512))
        self.assertIn(" LEFT ANTI JOIN ", datafusion_sql("join_anti_count", 512))


if __name__ == "__main__":
    unittest.main()
