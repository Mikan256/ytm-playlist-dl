import os
import tempfile
import unittest
from unittest import mock

import service
from models import DownloadResult, NormalizationResult
from options import RunOptions


INFO = {
    "title": "My Playlist",
    "total": 3,
    "entries": [
        {"index": 1, "title": "One", "duration": 10},
        {"index": 2, "title": "Two", "duration": 20},
        {"index": 3, "title": "Three", "duration": 30},
    ],
}


class ServiceTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.output = self._tmp.name

    def _options(self, **overrides):
        values = {
            "url": "https://youtube.com/playlist?list=PL1",
            "output": self.output,
            "normalize_mode": "album",
        }
        values.update(overrides)
        return RunOptions(**values)

    def test_list_tracks_does_not_download(self):
        with mock.patch.object(service.downloader, "get_playlist_info",
                               return_value=INFO):
            with mock.patch.object(service.downloader, "download_playlist") as dl:
                result = service.execute(self._options(list_tracks=True))

        self.assertEqual(result.info["total"], 3)
        self.assertEqual(result.files, [])
        dl.assert_not_called()

    def test_dry_run_returns_planned_paths_for_range(self):
        with mock.patch.object(service.downloader, "get_playlist_info",
                               return_value=INFO):
            with mock.patch.object(service.downloader, "download_playlist") as dl:
                result = service.execute(
                    self._options(dry_run=True, start=2, end=3)
                )

        self.assertEqual(len(result.planned), 2)
        self.assertIn("02_Two.flac", result.planned[0])
        self.assertIn("03_Three.flac", result.planned[1])
        dl.assert_not_called()

    def test_download_and_normalise(self):
        files = [os.path.join(self.output, "01_One.flac")]
        download = DownloadResult(files=files, total=3)
        normalization = NormalizationResult(
            files=files, normalized=files, mode="album", gain_db=6.0
        )
        with mock.patch.object(service.downloader, "get_playlist_info",
                               return_value=INFO):
            with mock.patch.object(
                service.downloader, "download_playlist", return_value=download
            ) as dl:
                with mock.patch.object(
                    service.normalizer, "normalize_album",
                    return_value=normalization,
                ) as norm:
                    result = service.execute(self._options())

        self.assertEqual(result.exit_code, 0)
        self.assertEqual(result.files, files)
        self.assertEqual(dl.call_args.kwargs["start"], None)
        self.assertEqual(dl.call_args.kwargs["jobs"], 1)
        self.assertEqual(norm.call_args.kwargs["mode"], "album")
        self.assertEqual(norm.call_args.kwargs["force"], False)

    def test_normalize_off_is_skipped(self):
        files = [os.path.join(self.output, "01_One.flac")]
        download = DownloadResult(files=files, total=3)
        with mock.patch.object(service.downloader, "get_playlist_info",
                               return_value=INFO):
            with mock.patch.object(service.downloader, "download_playlist",
                                   return_value=download):
                with mock.patch.object(service.normalizer, "normalize_album") as norm:
                    result = service.execute(self._options(normalize_mode="off"))
        self.assertIsNone(result.normalization)
        norm.assert_not_called()

    def test_partial_failure_sets_exit_code_2_and_writes_report(self):
        files = [os.path.join(self.output, "01_One.flac")]
        download = DownloadResult(
            files=files, failed=["track 2 unavailable"], total=3
        )
        report_path = os.path.join(self.output, "report.json")
        with mock.patch.object(service.downloader, "get_playlist_info",
                               return_value=INFO):
            with mock.patch.object(service.downloader, "download_playlist",
                                   return_value=download):
                with mock.patch.object(service.normalizer, "normalize_album",
                                       return_value=NormalizationResult(files=files)):
                    result = service.execute(
                        self._options(report_file=report_path)
                    )

        self.assertEqual(result.exit_code, 2)
        self.assertTrue(os.path.exists(report_path))
        self.assertEqual(result.report["failed"], ["track 2 unavailable"])

    def test_missing_info_is_fetched(self):
        with mock.patch.object(
            service.downloader, "get_playlist_info", return_value=INFO
        ) as info:
            with mock.patch.object(
                service.downloader, "download_playlist",
                return_value=DownloadResult(files=[], total=3),
            ):
                with mock.patch.object(
                    service.normalizer, "normalize_album",
                    return_value=NormalizationResult(files=[]),
                ):
                    service.execute(self._options(normalize_mode="off"))
        info.assert_called_once()

    def test_progress_events_are_emitted(self):
        events = []
        files = [os.path.join(self.output, "01_One.flac")]

        def fake_download(*args, **kwargs):
            kwargs["progress_callback"](1, 3, "One")
            return DownloadResult(files=files, total=3)

        def fake_normalize(files_arg, **kwargs):
            kwargs["progress_callback"](1, 1, "One", "normalising")
            return NormalizationResult(files=files, normalized=files)

        with mock.patch.object(service.downloader, "get_playlist_info",
                               return_value=INFO):
            with mock.patch.object(service.downloader, "download_playlist",
                                   side_effect=fake_download):
                with mock.patch.object(service.normalizer, "normalize_album",
                                       side_effect=fake_normalize):
                    service.execute(self._options(), progress=events.append)

        types = [event["type"] for event in events]
        self.assertIn("download", types)
        self.assertIn("normalize", types)
        self.assertEqual(events[-1]["type"], "status")


if __name__ == "__main__":
    unittest.main()
