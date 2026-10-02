import datetime
import json
import pathlib
import tempfile
import unittest

from compare_workload_engines import check_preview, validate
from workload_suite import QUERIES, fingerprint, make_fixture, polars_query, quote, sql_query


class WorkloadTest(unittest.TestCase):
    def test_fingerprint_preserves_null_type_and_multiplicity(self):
        rows = [[1, None, 'Café', True, -0.0, datetime.date(2022, 1, 8)],
                [2, '', '東京', False, 1.25, datetime.date(2022, 1, 9)]]
        self.assertNotEqual(fingerprint(rows, 'ordered'), fingerprint(rows[::-1], 'ordered'))
        self.assertEqual(fingerprint(rows, 'wide_materialize'), fingerprint(rows[::-1], 'wide_materialize'))
        self.assertNotEqual(fingerprint(rows, 'wide_materialize'), fingerprint(rows + rows, 'wide_materialize'))
        self.assertNotEqual(fingerprint([[None]], 'x'), fingerprint([['']], 'x'))
        self.assertNotEqual(fingerprint([[True]], 'x'), fingerprint([[1]], 'x'))
        self.assertEqual(fingerprint([[-0.0]], 'x'), fingerprint([[0.0]], 'x'))
        self.assertEqual(fingerprint([[datetime.date(2022, 1, 8)]], 'x'), fingerprint([[19000]], 'x'))

    def test_fixtures_native_differential_and_preview(self):
        import duckdb
        import polars as pl
        with tempfile.TemporaryDirectory(prefix='peg-workload-test-') as scratch:
            for case in ('balanced:512', 'skewed_nulls:512', 'plain_strings:512'):
                fixture = json.loads(make_fixture(scratch, case).read_text())
                connection = duckdb.connect()
                try:
                    connection.execute(f"CREATE VIEW facts AS SELECT * FROM read_parquet({quote(fixture['parquet'])})")
                    connection.execute(f"CREATE VIEW dimension AS SELECT * FROM read_parquet({quote(fixture['dimension'])})")
                    facts, dimension = pl.scan_parquet(fixture['parquet']), pl.scan_parquet(fixture['dimension'])
                    for name in QUERIES:
                        with self.subTest(case=case, query=name):
                            duck = connection.execute(sql_query(name, 512)).fetchall()
                            pol = polars_query(name, facts, dimension, 512).collect().iter_rows()
                            self.assertEqual(fingerprint(duck, name), fingerprint(pol, name))
                    preview = fingerprint(connection.execute(sql_query('preview_limit', 512)).fetchall(), 'preview_limit')
                    check_preview(preview, fixture)
                    bad = {**preview, 'sample': [list(row) for row in preview['sample']]}
                    bad['sample'][0][1] = 'wrong source value'
                    with self.assertRaises(ValueError):
                        check_preview(bad, fixture)
                    bad = {**preview, 'sample': preview['sample'][:-1] + [preview['sample'][0]]}
                    with self.assertRaises(ValueError):
                        check_preview(bad, fixture)
                    if case.startswith('plain'):
                        for column, encodings, _ in fixture['encodings']:
                            if column in ('category', 'message', 'customer_code'):
                                self.assertTrue(all('DICTIONARY' not in encoding for encoding in encodings))
                    self.assertEqual(connection.execute('SELECT count(*) FROM facts').fetchone()[0], 512)
                finally:
                    connection.close()

    def test_mismatches_and_missing_results_are_rejected(self):
        result = {'name': 'count_distinct', 'signature': fingerprint([[12, 8]], 'count_distinct')}
        wrong = {'name': 'count_distinct', 'signature': fingerprint([[11, 8]], 'count_distinct')}
        records = [{'engine': 'a', 'queries': [result]}, {'engine': 'b', 'queries': [wrong]}]
        self.assertTrue(validate(records, {}, ['count_distinct']))
        records[1]['queries'] = []
        self.assertTrue(validate(records, {}, ['count_distinct']))

    def test_fixture_cache_preserves_old_generator_and_checks_content(self):
        with tempfile.TemporaryDirectory(prefix='peg-workload-cache-test-') as scratch:
            first = make_fixture(scratch, 'balanced:512')
            original = json.loads(first.read_text())
            old_data = pathlib.Path(original['parquet']).read_bytes()
            first.write_text(json.dumps({**original, 'generator_sql': 'old generator'}))
            second = make_fixture(scratch, 'balanced:512')
            self.assertNotEqual(first, second)
            self.assertEqual(pathlib.Path(original['parquet']).read_bytes(), old_data)
            self.assertEqual(make_fixture(scratch, 'balanced:512'), second)
            current = json.loads(second.read_text())
            pathlib.Path(current['parquet']).write_bytes(b'corrupt')
            with self.assertRaisesRegex(ValueError, 'cached fixture content changed'):
                make_fixture(scratch, 'balanced:512')


if __name__ == '__main__':
    unittest.main()
