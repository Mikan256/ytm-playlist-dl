import json
import os
import tempfile
import unittest

import report
from models import DownloadResult, NormalizationResult
from options import RunOptions


class ReportTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.options = RunOptions(url="https://youtube.com/playlist?list=PL1")

    def test_build_report_marks_partial_failure(self):
        download = DownloadResult(files=["a.flac"], failed=["bad track"], total=2)
        built = report.build_report(
            self.options, {"title": "P", "total": 2}, "/out", download
        )
        self.assertEqual(built["exit_code"], 2)
        self.assertEqual(built["failed"], ["bad track"])
        self.assertEqual(built["playlist"]["title"], "P")

    def test_write_report_and_append_log(self):
        path = os.path.join(self._tmp.name, "report.json")
        log = os.path.join(self._tmp.name, "run.log")
        built = report.build_report(
            self.options, {"title": "P"}, "/out",
            NormalizationResult(files=[], mode="off"),
        )
        written = report.write_report(path, built)
        self.assertEqual(written, path)
        with open(path, encoding="utf-8") as handle:
            self.assertEqual(json.load(handle)["app"], "ytm-playlist-dl")

        report.append_log(log, "hello")
        report.append_log(log, "world")
        with open(log, encoding="utf-8") as handle:
            content = handle.read()
        self.assertIn("hello", content)
        self.assertIn("world", content)


if __name__ == "__main__":
    unittest.main()
