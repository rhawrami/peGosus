import pathlib
import platform
import subprocess
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from compare_parquet_memory import ProcessMemory, read_event


@unittest.skipUnless(platform.system() == "Darwin", "requires macOS footprint counters")
class FootprintCounterTest(unittest.TestCase):
    def test_interval_peak_captures_released_allocation(self):
        program = """
import gc,json,sys
print(json.dumps({'stage':'ready'}),flush=True)
sys.stdin.readline()
data=bytearray(32<<20)
for offset in range(0,len(data),4096):data[offset]=1
print(json.dumps({'stage':'held'}),flush=True)
sys.stdin.readline()
del data
gc.collect()
print(json.dumps({'stage':'released'}),flush=True)
sys.stdin.readline()
"""
        child = subprocess.Popen([sys.executable, "-c", program], stdin=subprocess.PIPE,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        counter = ProcessMemory()
        try:
            read_event(child, "ready")
            baseline = counter.snapshot(child.pid)["phys_footprint"]
            counter.reset(child.pid)
            child.stdin.write("allocate\n")
            child.stdin.flush()
            read_event(child, "held")
            held = counter.snapshot(child.pid)
            self.assertGreaterEqual(held["phys_footprint"] - baseline, 30 << 20)
            child.stdin.write("release\n")
            child.stdin.flush()
            read_event(child, "released")
            released = counter.snapshot(child.pid)
            self.assertGreaterEqual(released["interval_max_phys_footprint"], held["phys_footprint"])
            self.assertLess(released["phys_footprint"], held["phys_footprint"] - (30 << 20))
            counter.reset(child.pid)
            reset = counter.snapshot(child.pid)
            self.assertLess(reset["interval_max_phys_footprint"], held["phys_footprint"] - (30 << 20))
        finally:
            child.stdin.close()
            child.wait(timeout=30)
            child.stdout.close()
            child.stderr.close()

    def test_mismatched_stage_is_rejected(self):
        child = subprocess.Popen([sys.executable, "-c", "print('{\"stage\":\"wrong\"}')"],
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            with self.assertRaisesRegex(RuntimeError, "expected ready"):
                read_event(child, "ready")
        finally:
            child.wait(timeout=30)
            child.stdout.close()
            child.stderr.close()


if __name__ == "__main__":
    unittest.main()
