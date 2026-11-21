"""JSON run reports and optional text logs."""

import json
import os
import time


def _timestamp():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _options_dict(options):
    if hasattr(options, "to_dict"):
        return options.to_dict()
    return dict(options or {})


def build_report(options, info=None, output_dir=None, download=None,
                 normalization=None, error=None):
    failed = []
    if download is not None:
        failed.extend(download.failed)
    if normalization is not None:
        failed.extend(normalization.failed)
    exit_code = 1 if error else (2 if failed else 0)
    return {
        "app": "ytm-playlist-dl",
        "generated_at": _timestamp(),
        "options": _options_dict(options),
        "playlist": info or {},
        "output_dir": output_dir,
        "download": download.as_dict() if download is not None else None,
        "normalization": (
            normalization.as_dict() if normalization is not None else None
        ),
        "failed": failed,
        "error": error,
        "exit_code": exit_code,
    }


def write_report(path, report):
    target = os.path.expanduser(path)
    parent = os.path.dirname(os.path.abspath(target))
    os.makedirs(parent, exist_ok=True)
    tmp = target + ".tmp"
    with open(tmp, "w", encoding="utf-8") as handle:
        json.dump(report, handle, ensure_ascii=False, indent=2)
        handle.write("\n")
    os.replace(tmp, target)
    return target


def append_log(path, message):
    if not path:
        return
    target = os.path.expanduser(path)
    parent = os.path.dirname(os.path.abspath(target))
    os.makedirs(parent, exist_ok=True)
    with open(target, "a", encoding="utf-8") as handle:
        handle.write(f"[{_timestamp()}] {message}\n")
