import os
import tempfile
import unittest
from unittest import mock

import downloader


class SanitizeTests(unittest.TestCase):
    def test_removes_invalid_filename_characters(self):
        self.assertEqual(
            downloader.sanitize('A/B\\C:D*E?F"G<H>I|J'),
            "ABCDEFGHIJ",
        )

    def test_collapses_whitespace_and_strips_dots(self):
        self.assertEqual(downloader.sanitize("  Song   Title.  "), "Song Title")
        self.assertEqual(downloader.sanitize("... "), "Unknown")

    def test_keeps_japanese_text(self):
        self.assertEqual(downloader.sanitize("夜に駆ける"), "夜に駆ける")


class NormaliseUrlTests(unittest.TestCase):
    def test_music_url_is_canonicalised_and_tracking_params_removed(self):
        self.assertEqual(
            downloader.normalise_url(
                "https://music.youtube.com/playlist?list=PL123&si=tracking"
            ),
            "https://youtube.com/playlist?list=PL123",
        )

    def test_scheme_is_added_and_www_is_removed(self):
        self.assertEqual(
            downloader.normalise_url("www.youtube.com/playlist?list=PL123&index=4"),
            "https://youtube.com/playlist?list=PL123",
        )

    def test_watch_path_is_preserved(self):
        self.assertEqual(
            downloader.normalise_url(
                "http://m.youtube.com/watch?v=abc&list=PL123&t=10"
            ),
            "https://youtube.com/watch?list=PL123&v=abc",
        )

    def test_short_share_url_is_converted_to_watch_url(self):
        self.assertEqual(
            downloader.normalise_url("https://youtu.be/abc?list=PL123"),
            "https://youtube.com/watch?list=PL123&v=abc",
        )

    def test_empty_url_is_rejected(self):
        with self.assertRaises(ValueError):
            downloader.normalise_url("")

    def test_missing_list_parameter_is_rejected(self):
        with self.assertRaises(ValueError):
            downloader.normalise_url("https://youtube.com/watch?v=abc")

    def test_unsupported_host_is_rejected(self):
        with self.assertRaises(ValueError):
            downloader.normalise_url("https://example.com/playlist?list=PL123")


class TemplateAndSortingTests(unittest.TestCase):
    def test_template_map(self):
        self.assertEqual(
            downloader.build_output_template("/out", "num_title"),
            os.path.join("/out", "%(playlist_index)02d_%(title)s.%(ext)s"),
        )
        self.assertIn(
            "%(artist,uploader)s",
            downloader.build_output_template("/out", "num_artist_title"),
        )

    def test_unknown_template_falls_back_to_default(self):
        self.assertEqual(
            downloader.build_output_template("/out", "unknown"),
            downloader.build_output_template("/out", "num_title"),
        )

    def test_natural_sort_handles_more_than_99_tracks(self):
        names = ["100_d.flac", "2_b.flac", "11_c.flac", "1_a.flac"]
        self.assertEqual(
            sorted(names, key=downloader.natural_sort_key),
            ["1_a.flac", "2_b.flac", "11_c.flac", "100_d.flac"],
        )


class FileCollectionTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.dir = self._tmp.name

    def _touch(self, name):
        path = os.path.join(self.dir, name)
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("x")
        return path

    def test_cleanup_removes_only_partial_downloads(self):
        partial = self._touch("01_song.flac.part")
        fragmented = self._touch("01_song.flac.part-Frag1")
        metadata = self._touch("01_song.flac.ytdl")
        audio = self._touch("01_song.flac")

        downloader.cleanup_partial_files(self.dir)

        self.assertFalse(os.path.exists(partial))
        self.assertFalse(os.path.exists(fragmented))
        self.assertFalse(os.path.exists(metadata))
        self.assertTrue(os.path.exists(audio))

    def test_collect_flac_files_excludes_partials_and_sidecars(self):
        for name in [
            "01_a.flac", "02_b.flac", "02_b.mp3", "03_c.flac.part",
            "100_d.flac", "11_e.flac", "cover.jpg",
        ]:
            self._touch(name)

        files = downloader.collect_output_files(self.dir, "flac")

        self.assertEqual(
            [os.path.basename(path) for path in files],
            ["01_a.flac", "02_b.flac", "11_e.flac", "100_d.flac"],
        )

    def test_collect_original_format_only_returns_audio(self):
        for name in ["01_a.webm", "02_b.m4a", "cover.jpg", "03_c.part"]:
            self._touch(name)

        files = downloader.collect_output_files(self.dir, "original")

        self.assertEqual(
            [os.path.basename(path) for path in files],
            ["01_a.webm", "02_b.m4a"],
        )

    def test_collect_rejects_unknown_format(self):
        with self.assertRaises(ValueError):
            downloader.collect_output_files(self.dir, "ogg")


class PlaylistInfoTests(unittest.TestCase):
    class FakeYDL:
        def __init__(self, options):
            self.options = options

        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, tb):
            return False

        def extract_info(self, url, download=False):
            self.url = url
            return {
                "title": "My: Playlist",
                "entries": [{"id": "a"}, None, {"id": "b"}],
            }

    def test_returns_sanitized_title_and_skips_empty_entries(self):
        info = downloader.get_playlist_info(
            "https://music.youtube.com/playlist?list=PL123",
            ydl_factory=self.FakeYDL,
        )
        self.assertEqual(info, {"title": "My Playlist", "total": 2})

    def test_extract_errors_are_wrapped(self):
        class BrokenYDL(self.FakeYDL):
            def extract_info(self, url, download=False):
                raise RuntimeError("boom")

        with self.assertRaisesRegex(RuntimeError, "Failed to get playlist info"):
            downloader.get_playlist_info(
                "https://youtube.com/playlist?list=PL123",
                ydl_factory=BrokenYDL,
            )

    def test_invalid_url_is_reported_as_runtime_error(self):
        with self.assertRaisesRegex(RuntimeError, "playlist id"):
            downloader.get_playlist_info(
                "https://youtube.com/watch?v=abc",
                ydl_factory=self.FakeYDL,
            )


class DownloadPlaylistTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.dir = self._tmp.name
        self.url = "https://music.youtube.com/playlist?list=PL123"

    def _fake_ytdlp(self, ydl_cls):
        return mock.patch.object(
            downloader, "_load_yt_dlp",
            return_value=mock.Mock(YoutubeDL=ydl_cls),
        )

    def test_downloads_files_reports_progress_and_builds_options(self):
        progress = []
        options = {}

        class FakeYDL:
            def __init__(self, opts):
                options.update(opts)

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def download(self, urls):
                self.assert_url = urls
                for index, title in ((1, "A"), (2, "B")):
                    path = os.path.join(self_dir, f"{index:02d}_{title}.flac")
                    with open(path, "w", encoding="utf-8") as handle:
                        handle.write("x")
                hook = options["progress_hooks"][0]
                hook({"status": "finished", "info_dict": {
                    "playlist_index": 1, "title": "A"}})
                hook({"status": "finished", "info_dict": {
                    "playlist_index": 2, "title": "B"}})

        self_dir = self.dir
        with self._fake_ytdlp(FakeYDL):
            files = downloader.download_playlist(
                self.url, self.dir, fmt="flac", template="num_title",
                progress_callback=lambda i, total, title: progress.append(
                    (i, total, title)
                ),
                total=2,
            )

        self.assertEqual(
            [os.path.basename(path) for path in files],
            ["01_A.flac", "02_B.flac"],
        )
        self.assertEqual(progress, [(1, 2, "A"), (2, 2, "B")])
        self.assertIn("%(playlist_index)02d_%(title)s", options["outtmpl"])
        self.assertEqual(options["retries"], 3)
        self.assertEqual(
            options["postprocessors"][0]["preferredcodec"], "flac"
        )
        self.assertFalse(options["ignoreerrors"])

    def test_retries_after_failure_then_succeeds(self):
        attempts = {"count": 0}
        sleeps = []

        class FlakyYDL:
            def __init__(self, opts):
                pass

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def download(self, urls):
                attempts["count"] += 1
                if attempts["count"] == 1:
                    raise RuntimeError("network down")
                with open(os.path.join(self_dir, "01_A.flac"), "w") as handle:
                    handle.write("x")

        self_dir = self.dir
        with self._fake_ytdlp(FlakyYDL):
            files = downloader.download_playlist(
                self.url, self.dir, retries=2, sleep=sleeps.append,
            )

        self.assertEqual(attempts["count"], 2)
        self.assertEqual(len(sleeps), 1)
        self.assertEqual(len(files), 1)

    def test_raises_after_all_retries_fail(self):
        class BrokenYDL:
            def __init__(self, opts):
                pass

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def download(self, urls):
                raise RuntimeError("network down")

        with self._fake_ytdlp(BrokenYDL):
            with self.assertRaisesRegex(RuntimeError, "after 2 attempts"):
                downloader.download_playlist(
                    self.url, self.dir, retries=2, sleep=lambda _: None,
                )

    def test_raises_when_no_files_were_saved(self):
        class EmptyYDL:
            def __init__(self, opts):
                pass

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def download(self, urls):
                return None

        with self._fake_ytdlp(EmptyYDL):
            with self.assertRaisesRegex(RuntimeError, "no tracks"):
                downloader.download_playlist(self.url, self.dir)

    def test_invalid_format_is_rejected(self):
        with self.assertRaises(ValueError):
            downloader.download_playlist(self.url, self.dir, fmt="ogg")


if __name__ == "__main__":
    unittest.main()
