import pathlib
import tempfile
import time
import unittest

from process_memory_linux import LinuxProcessMemory, STATUS_FIELDS, parse_kib_fields


class LinuxMemoryTests(unittest.TestCase):
    def test_parser_preserves_rss_and_lifetime_peak_separately(self):
        parsed = parse_kib_fields("Name:\tworker\nVmRSS:\t1024 kB\nVmHWM:\t8192 kB\nRssAnon:\t512 kB\nRssFile:\t500 kB\nRssShmem:\t12 kB\n", STATUS_FIELDS)
        self.assertEqual(parsed["resident_size"], 1 << 20)
        self.assertEqual(parsed["lifetime_max_resident_size"], 8 << 20)
        self.assertEqual(parsed["anonymous_resident_size"] + parsed["file_resident_size"] + parsed["shared_resident_size"], 1 << 20)

    def test_parser_rejects_bad_units_and_negative_sizes(self):
        for value in ("-1 kB", "1 MB", "1.5 kB", "one kB", "1"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                parse_kib_fields("VmRSS: " + value, STATUS_FIELDS)

    def test_snapshot_optional_pss_and_io(self):
        with tempfile.TemporaryDirectory() as directory:
            proc = pathlib.Path(directory) / "123"
            proc.mkdir()
            (proc / "status").write_text("VmRSS: 1024 kB\nVmHWM: 8192 kB\n")
            counter = LinuxProcessMemory(directory)
            snapshot = counter.snapshot(123)
            self.assertEqual(snapshot["resident_size"], 1 << 20)
            self.assertIsNone(snapshot["proportional_set_size"])
            self.assertNotIn("phys_footprint", snapshot)
            (proc / "smaps_rollup").write_text("Rss: 1024 kB\nPss: 768 kB\nPrivate_Clean: 4 kB\nPrivate_Dirty: 508 kB\nSwap: 0 kB\n")
            (proc / "io").write_text("read_bytes: 4096\nwrite_bytes: 8192\n")
            snapshot = counter.snapshot(123)
            self.assertEqual(snapshot["proportional_set_size"], 768*1024)
            self.assertEqual(snapshot["resident_size_smaps"], 1 << 20)
            self.assertEqual(snapshot["diskio_bytesread"], 4096)

    def test_stage_peak_does_not_reuse_lifetime_hwm(self):
        with tempfile.TemporaryDirectory() as directory:
            proc = pathlib.Path(directory) / "123"
            proc.mkdir()
            (proc / "status").write_text("VmRSS: 1024 kB\nVmHWM: 8192 kB\n")
            counter = LinuxProcessMemory(directory, sample_seconds=0.001)
            with counter.interval(123) as sample:
                time.sleep(0.005)
            metrics = sample.metrics()
            self.assertEqual(metrics["interval_peak_sampled_rss_bytes"], 1 << 20)
            self.assertGreaterEqual(metrics["rss_samples"], 2)
            self.assertTrue(metrics["peak_is_lower_bound"])
            self.assertEqual(metrics["sampling_errors"], [])
            self.assertFalse(sample.thread.is_alive())
            with counter.interval(123) as second:
                pass
            self.assertEqual(second.metrics()["interval_peak_sampled_rss_bytes"], 1 << 20)

    def test_sampler_stops_when_worker_operation_raises(self):
        with tempfile.TemporaryDirectory() as directory:
            proc = pathlib.Path(directory) / "123"
            proc.mkdir()
            (proc / "status").write_text("VmRSS: 1024 kB\n")
            counter = LinuxProcessMemory(directory)
            with self.assertRaisesRegex(RuntimeError, "worker failure"):
                with counter.interval(123) as sample:
                    raise RuntimeError("worker failure")
            self.assertFalse(sample.thread.is_alive())

    def test_missing_rss_is_not_zero(self):
        with tempfile.TemporaryDirectory() as directory:
            proc = pathlib.Path(directory) / "123"
            proc.mkdir()
            (proc / "status").write_text("Name: worker\n")
            with self.assertRaisesRegex(RuntimeError, "RSS unavailable"):
                LinuxProcessMemory(directory).snapshot(123)


if __name__ == "__main__":
    unittest.main()
