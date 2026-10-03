"""Linux process-memory snapshots and sampled RSS intervals.

VmHWM is a lifetime high-water mark; it is never used as a resettable stage
peak. Sampled peaks are lower bounds and can miss short-lived allocations.
"""
import contextlib
import pathlib
import threading
import time


STATUS_FIELDS = {"VmRSS": "resident_size", "VmHWM": "lifetime_max_resident_size",
                 "RssAnon": "anonymous_resident_size", "RssFile": "file_resident_size",
                 "RssShmem": "shared_resident_size"}
ROLLUP_FIELDS = {"Rss": "resident_size_smaps", "Pss": "proportional_set_size", "Private_Clean": "private_clean_size",
                 "Private_Dirty": "private_dirty_size", "Swap": "swap_size"}


def parse_kib_fields(text, fields):
    result = {}
    for line in text.splitlines():
        key, separator, value = line.partition(":")
        if not separator or key not in fields:
            continue
        parts = value.split()
        if len(parts) != 2 or parts[1] != "kB" or not parts[0].isdigit():
            raise ValueError(f"invalid /proc size field {key}: {value!r}")
        result[fields[key]] = int(parts[0]) * 1024
    return result


class LinuxProcessMemory:
    current_key = "resident_size"
    peak_key = "interval_peak_sampled_rss_bytes"
    metric = "sampled_rss"
    description = "Linux /proc approximate VmRSS sampled every 5 ms plus stage endpoints; sampled peaks can miss transient allocations and are not resettable kernel peaks or macOS physical footprint. VmHWM is lifetime-only; precise smaps_rollup RSS/PSS endpoints are also recorded when available."

    def __init__(self, proc_root=pathlib.Path("/proc"), sample_seconds=0.005):
        if sample_seconds <= 0:
            raise ValueError("invalid memory sampling interval")
        self.proc_root = pathlib.Path(proc_root)
        self.sample_seconds = sample_seconds

    def snapshot(self, pid):
        root = self.proc_root / str(pid)
        result = parse_kib_fields((root / "status").read_text(), STATUS_FIELDS)
        if "resident_size" not in result:
            raise RuntimeError(f"RSS unavailable for process {pid}")
        result["memory_metric"] = "rss"
        result["proportional_set_size"] = None
        result["resident_size_smaps"] = None
        try:
            result.update(parse_kib_fields((root / "smaps_rollup").read_text(), ROLLUP_FIELDS))
        except (FileNotFoundError, PermissionError, ProcessLookupError):
            pass
        try:
            io = dict(line.split(":", 1) for line in (root / "io").read_text().splitlines())
            result.update({"diskio_bytesread": int(io["read_bytes"]), "diskio_byteswritten": int(io["write_bytes"])})
        except (FileNotFoundError, PermissionError, ProcessLookupError):
            pass
        return result

    def reset(self, pid):
        # Linux VmHWM cannot provide per-stage peaks without changing /proc state.
        # The interval context owns fresh samples instead.
        return None

    @contextlib.contextmanager
    def interval(self, pid):
        sampler = RSSInterval(self, pid)
        sampler.start()
        try:
            yield sampler
        finally:
            sampler.finish()


class RSSInterval:
    def __init__(self, counter, pid):
        self.counter, self.pid = counter, pid
        self.peak_bytes = 0
        self.samples = 0
        self.max_gap_ns = 0
        self.last_sample_ns = None
        self.errors = []
        self.done = threading.Event()
        self.thread = None

    def sample(self):
        try:
            now = time.monotonic_ns()
            text = (self.counter.proc_root / str(self.pid) / "status").read_text()
            value = parse_kib_fields(text, STATUS_FIELDS)["resident_size"]
            self.peak_bytes = max(self.peak_bytes, value)
            self.samples += 1
            if self.last_sample_ns is not None:
                self.max_gap_ns = max(self.max_gap_ns, now-self.last_sample_ns)
            self.last_sample_ns = now
        except (OSError, ValueError, KeyError) as error:
            if not self.errors:
                self.errors.append(f"{type(error).__name__}: {error}")

    def start(self):
        self.sample()
        def monitor():
            while not self.done.wait(self.counter.sample_seconds):
                self.sample()
        self.thread = threading.Thread(target=monitor, daemon=True)
        self.thread.start()

    def finish(self):
        self.done.set()
        self.thread.join()
        self.sample()

    def metrics(self):
        return {"interval_peak_sampled_rss_bytes": self.peak_bytes,
                "rss_samples": self.samples,
                "requested_sample_period_ns": round(self.counter.sample_seconds*1e9),
                "largest_sample_gap_ns": self.max_gap_ns,
                "sampling_errors": self.errors,
                "peak_is_lower_bound": True}
