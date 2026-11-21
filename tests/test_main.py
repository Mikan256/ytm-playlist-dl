import contextlib
import io
import unittest
from unittest import mock

import main


class ParserTests(unittest.TestCase):
    def test_defaults_match_spec(self):
        args = main.build_parser().parse_args(
            ["https://music.youtube.com/playlist?list=PL123"]
        )
        self.assertEqual(args.url, "https://music.youtube.com/playlist?list=PL123")
        self.assertEqual(args.fmt, "flac")
        self.assertEqual(args.template, "num_title")
        self.assertEqual(args.loudness, -14)
        self.assertEqual(args.output, "./downloads")
        self.assertFalse(args.no_normalize)

    def test_all_options_are_accepted(self):
        args = main.build_parser().parse_args([
            "https://youtube.com/playlist?list=PL123",
            "--format", "mp3",
            "--template", "num_artist_title",
            "--loudness", "-23",
            "--output", "/tmp/music",
            "--no-normalize",
        ])
        self.assertEqual(args.fmt, "mp3")
        self.assertEqual(args.template, "num_artist_title")
        self.assertEqual(args.loudness, -23.0)
        self.assertEqual(args.output, "/tmp/music")
        self.assertTrue(args.no_normalize)

    def test_url_is_optional_for_interactive_mode(self):
        args = main.build_parser().parse_args([])
        self.assertIsNone(args.url)

    def test_out_of_range_loudness_is_rejected(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                main.build_parser().parse_args(["url", "--loudness", "-100"])

    def test_invalid_format_is_rejected(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                main.build_parser().parse_args(["url", "--format", "ogg"])


class RunTests(unittest.TestCase):
    def setUp(self):
        self.info = {"title": "My Playlist", "total": 2}

    def _run(self, **overrides):
        kwargs = dict(
            url="https://youtube.com/playlist?list=PL123",
            fmt="flac",
            template="num_title",
            do_normalize=True,
            loudness=-14,
            output_base="/tmp/out",
            info=self.info,
            show_info=False,
        )
        kwargs.update(overrides)
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = main.run(**kwargs)
        return code, out.getvalue(), err.getvalue()

    def test_successful_download_and_normalise(self):
        files = ["/tmp/out/My Playlist/01_a.flac"]
        with mock.patch.object(main, "download_playlist", return_value=files) as dl:
            with mock.patch.object(main, "normalize_album", return_value=files) as norm:
                code, out, err = self._run()

        self.assertEqual(code, 0)
        self.assertIn("Downloaded 1/2 tracks", out)
        self.assertIn("Done!", out)
        self.assertIn("Saved to: /tmp/out/My Playlist", out)
        self.assertEqual(dl.call_args.kwargs["total"], 2)
        self.assertEqual(dl.call_args.kwargs["fmt"], "flac")
        self.assertTrue(callable(dl.call_args.kwargs["progress_callback"]))
        norm.assert_called_once()
        self.assertEqual(norm.call_args.kwargs["target_loudness"], -14)

    def test_no_normalize_skips_normalizer(self):
        files = ["/tmp/out/My Playlist/01_a.flac"]
        with mock.patch.object(main, "download_playlist", return_value=files):
            with mock.patch.object(main, "normalize_album") as norm:
                code, _, _ = self._run(do_normalize=False)

        self.assertEqual(code, 0)
        norm.assert_not_called()

    def test_playlist_info_is_fetched_when_not_supplied(self):
        files = ["/tmp/out/My Playlist/01_a.flac"]
        with mock.patch.object(
            main, "get_playlist_info", return_value=self.info
        ) as info:
            with mock.patch.object(main, "download_playlist", return_value=files):
                with mock.patch.object(main, "normalize_album", return_value=files):
                    code, out, _ = self._run(info=None, show_info=True)

        self.assertEqual(code, 0)
        info.assert_called_once()
        self.assertIn("Playlist: My Playlist", out)

    def test_invalid_playlist_returns_error(self):
        with mock.patch.object(
            main, "get_playlist_info", side_effect=RuntimeError("bad url")
        ):
            code, _, err = self._run(info=None)

        self.assertEqual(code, 1)
        self.assertIn("bad url", err)

    def test_download_failure_returns_error(self):
        with mock.patch.object(
            main, "download_playlist", side_effect=RuntimeError("network down")
        ):
            code, _, err = self._run()

        self.assertEqual(code, 1)
        self.assertIn("Download failed", err)

    def test_normalisation_failure_returns_error(self):
        files = ["/tmp/out/My Playlist/01_a.flac"]
        with mock.patch.object(main, "download_playlist", return_value=files):
            with mock.patch.object(
                main, "normalize_album", side_effect=RuntimeError("ffmpeg missing")
            ):
                code, _, err = self._run()

        self.assertEqual(code, 1)
        self.assertIn("Normalisation failed", err)

    def test_confirmation_can_cancel_download(self):
        with mock.patch.object(main, "prompt_bool", return_value=False):
            with mock.patch.object(main, "download_playlist") as dl:
                code, out, _ = self._run(confirm=True)

        self.assertEqual(code, 0)
        self.assertIn("Cancelled", out)
        dl.assert_not_called()


class MainDispatchTests(unittest.TestCase):
    def test_url_runs_cli_mode(self):
        with mock.patch.object(main, "run", return_value=7) as run:
            code = main.main([
                "https://youtube.com/playlist?list=PL123",
                "--format", "mp3",
                "--no-normalize",
            ])

        self.assertEqual(code, 7)
        self.assertEqual(run.call_args.kwargs["fmt"], "mp3")
        self.assertFalse(run.call_args.kwargs["do_normalize"])

    def test_no_url_runs_interactive_mode(self):
        with mock.patch.object(main, "interactive", return_value=3) as interactive:
            code = main.main([])

        self.assertEqual(code, 3)
        interactive.assert_called_once()

    def test_keyboard_interrupt_returns_error(self):
        with mock.patch.object(main, "run", side_effect=KeyboardInterrupt):
            with contextlib.redirect_stdout(io.StringIO()):
                code = main.main(["https://youtube.com/playlist?list=PL123"])
        self.assertEqual(code, 1)


class InteractiveTests(unittest.TestCase):
    def test_interactive_collects_options_and_delegates_to_run(self):
        info = {"title": "My Playlist", "total": 2}
        answers = [
            "https://youtube.com/playlist?list=PL123",  # URL
            "",                                          # format: flac
            "2",                                         # template: artist
            "n",                                         # no normalisation
            "/tmp/out",                                  # output directory
            "",                                          # start download
        ]
        out = io.StringIO()
        with mock.patch("builtins.input", side_effect=answers):
            with mock.patch.object(main, "get_playlist_info", return_value=info):
                with mock.patch.object(main, "run", return_value=0) as run:
                    with contextlib.redirect_stdout(out):
                        code = main.interactive()

        self.assertEqual(code, 0)
        self.assertIn("My Playlist", out.getvalue())
        self.assertEqual(run.call_args.kwargs["fmt"], "flac")
        self.assertEqual(run.call_args.kwargs["template"], "num_artist_title")
        self.assertFalse(run.call_args.kwargs["do_normalize"])
        self.assertEqual(run.call_args.kwargs["output_base"], "/tmp/out")
        self.assertEqual(run.call_args.kwargs["info"], info)

    def test_interactive_requires_url(self):
        with mock.patch("builtins.input", return_value=""):
            with contextlib.redirect_stdout(io.StringIO()):
                code = main.interactive()
        self.assertEqual(code, 1)


class PromptTests(unittest.TestCase):
    def test_prompt_uses_default_on_empty_input(self):
        with mock.patch("builtins.input", return_value=""):
            self.assertEqual(main.prompt("Format", default="flac"), "flac")

    def test_prompt_retries_until_choice_is_valid(self):
        with mock.patch("builtins.input", side_effect=["bad", "mp3"]):
            with contextlib.redirect_stdout(io.StringIO()):
                value = main.prompt("Format", default="flac", choices=["flac", "mp3"])
        self.assertEqual(value, "mp3")

    def test_prompt_bool(self):
        with mock.patch("builtins.input", side_effect=["", "n", "yes"]):
            self.assertTrue(main.prompt_bool("ok?"))
            self.assertFalse(main.prompt_bool("ok?"))
            self.assertTrue(main.prompt_bool("ok?"))

    def test_prompt_int_retries_after_invalid_and_range_errors(self):
        with mock.patch("builtins.input", side_effect=["abc", "-100", "-14"]):
            with contextlib.redirect_stdout(io.StringIO()):
                value = main.prompt_int("value", default=-14, min_val=-30, max_val=-5)
        self.assertEqual(value, -14)

    def test_select_template(self):
        with mock.patch("builtins.input", side_effect=["", "2"]):
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(main.select_template(), "num_title")
                self.assertEqual(main.select_template(), "num_artist_title")


if __name__ == "__main__":
    unittest.main()
