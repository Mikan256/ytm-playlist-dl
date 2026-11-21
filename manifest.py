"""Persistent record of already-normalised files."""

import json
import os
import time

MANIFEST_NAME = ".ytm-playlist-dl.manifest.json"


def manifest_path_for_dir(directory):
    return os.path.join(os.path.abspath(directory), MANIFEST_NAME)


def manifest_path_for_files(files):
    if not files:
        return None
    paths = [os.path.abspath(path) for path in files]
    base = os.path.commonpath(paths)
    if not os.path.isdir(base):
        base = os.path.dirname(base)
    return manifest_path_for_dir(base)


def fingerprint(path):
    stat = os.stat(path)
    return {"size": stat.st_size, "mtime_ns": stat.st_mtime_ns}


class Manifest:
    def __init__(self, path):
        self.path = path
        self.data = {"version": 1, "normalized": {}}
        self._load()

    def _load(self):
        if not self.path:
            return
        try:
            with open(self.path, encoding="utf-8") as handle:
                data = json.load(handle)
        except (OSError, ValueError):
            return
        if isinstance(data, dict) and isinstance(data.get("normalized"), dict):
            self.data = data
            self.data.setdefault("version", 1)
            self.data.setdefault("normalized", {})

    def _key(self, filepath):
        base = os.path.dirname(os.path.abspath(self.path))
        return os.path.relpath(os.path.abspath(filepath), base)

    def is_current(self, filepath, mode, target_loudness):
        if not self.path or not os.path.exists(filepath):
            return False
        record = self.data["normalized"].get(self._key(filepath))
        if not isinstance(record, dict):
            return False
        try:
            current = fingerprint(filepath)
        except OSError:
            return False
        return (
            record.get("size") == current["size"]
            and record.get("mtime_ns") == current["mtime_ns"]
            and record.get("mode") == mode
            and abs(float(record.get("target", 0)) - float(target_loudness)) < 1e-6
        )

    def gain_for(self, filepath, mode, target_loudness):
        if not self.is_current(filepath, mode, target_loudness):
            return None
        record = self.data["normalized"].get(self._key(filepath)) or {}
        gain = record.get("gain_db")
        try:
            return float(gain) if gain is not None else None
        except (TypeError, ValueError):
            return None

    def mark(self, filepath, mode, target_loudness, gain_db=None):
        if not self.path:
            return
        try:
            current = fingerprint(filepath)
        except OSError:
            return
        self.data["normalized"][self._key(filepath)] = {
            "size": current["size"],
            "mtime_ns": current["mtime_ns"],
            "mode": mode,
            "target": float(target_loudness),
            "gain_db": gain_db,
            "normalized_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }

    def save(self):
        if not self.path:
            return
        parent = os.path.dirname(self.path)
        os.makedirs(parent, exist_ok=True)
        tmp = self.path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as handle:
            json.dump(self.data, handle, ensure_ascii=False, indent=2)
            handle.write("\n")
        os.replace(tmp, self.path)
