import contextlib
import io
import json
import unittest
from unittest import mock

import config
import main
import service
from models import DownloadResult, NormalizationResult
from options import RunOptions


INFO = {
    "title": "My Playlist",
    "total": 2,
    "entries": [
        {"index": 1, "title": "One", "duration": 10},
        {"index": 2, "title": "Two", "duration": 20},
    ],
}


def make_result(options, files=None, failed=None, normalization=None,
                planned=None, exit_code=0):
    files = list(files or [])
    download = DownloadResult(files=files, failed=list(failed or []), total=2)
    return service.RunResult(
        options=options,
        info=INFO,
        output_dir="/tmp/out/My Playlist",
        download=download,
        normalization=normalization,
        report={"exit_code": exit_code},
        planned=list(planned or []),
    )


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
        self.assertEqual(args.normalize_mode, "album")
        self.assertFalse(args.keep_going)
        self.assertEqual(args.jobs, 1)

    def test_all_options_are_accepted(self):
        args = main.build_parser().parse_args([
            "https://youtube.com/playlist?list=PL123",
            "--format", "mp3",
            "--template", "num_artist_title",
            "--template-string", "%(title)s.%(ext)s",
            "--loudness", "-23",
            "--output", "/tmp/music",
            "--replaygain",
            "--force-normalize",
            "--start", "2",
            "--end", "5",
            "--keep-going",
            "--audio-quality", "320",
            "--embed-metadata",
            "--embed-thumbnail",
            "--jobs", "3",
            "--concurrent-fragments", "4",
            "--dry-run",
            "--report-file", "report.json",
            "--log-file", "run.log",
            "--json",
        ])
        self.assertEqual(args.fmt, "mp3")
        self.assertEqual(args.template, "num_artist_title")
        self.assertEqual(args.template_string, "%(title)s.%(ext)s")
        self.assertEqual(args.loudness, -23.0)
        self.assertEqual(args.output, "/tmp/music")
        self.assertEqual(args.normalize_mode, "replaygain")
        self.assertTrue(args.force_normalize)
        self.assertEqual((args.start, args.end), (2, 5))
        self.assertTrue(args.keep_going)
        self.assertEqual(args.audio_quality, "320")
        self.assertTrue(args.embed_metadata)
        self.assertTrue(args.embed_thumbnail)
        self.assertEqual((args.jobs, args.concurrent_fragments), (3, 4))
        self.assertTrue(args.dry_run)
        self.assertEqual(args.report_file, "report.json")
        self.assertEqual(args.log_file, "run.log")
        self.assertTrue(args.json)

    def test_no_normalize_shortcut(self):
        args = main.build_parser().parse_args(["url", "--no-normalize"])
        self.assertEqual(args.normalize_mode, "off")

    def test_config_defaults_are_used(self):
        parser = main.build_parser({"format": "mp3", "jobs": 4,
                                    "normalize_mode": "replaygain"})
        args = parser.parse_args(["url"])
        self.assertEqual(args.fmt, "mp3")
        self.assertEqual(args.jobs, 4)
        self.assertEqual(args.normalize_mode, "replaygain")

    def test_url_is_optional_for_interactive_or_web_mode(self):
        self.assertIsNone(main.build_parser().parse_args([]).url)

    def test_out_of_range_loudness_is_rejected(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                main.build_parser().parse_args(["url", "--loudness", "-100"])

    def test_invalid_format_is_rejected(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                main.build_parser().parse_args(["url", "--format", "ogg"])

    def test_invalid_range_is_rejected(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                main.build_parser().parse_args(
                    ["url", "--start", "5", "--end", "2"]
                )


class RunTests(unittest.TestCase):
    def _options(self, **overrides):
        values = {"url": "https://youtube.com/playlist?list=PL123"}
        values.update(overrides)
        return RunOptions(**values)

    def _run(self, options, **kwargs):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = main.run(options, info=INFO, **kwargs)
        return code, out.getvalue(), err.getvalue()

    def test_successful_download_and_normalise(self):
        options = self._options()
        files = ["/tmp/out/My Playlist/01_a.flac"]
        norm = NormalizationResult(
            files=files, normalized=files, mode="album", gain_db=6.0
        )
        result = make_result(options, files=files, normalization=norm)
        with mock.patch.object(main.service, "execute", return_value=result) as execute:
            code, out, _ = self._run(options)

        self.assertEqual(code, 0)
        self.assertIn("Downloaded 1/2 tracks", out)
        self.assertIn("Done!", out)
        self.assertIn("Normalised 1 tracks", out)
        execute.assert_called_once()
        self.assertEqual(execute.call_args.args[0], options)

    def test_no_normalize_is_passed_to_service(self):
        options = self._options(normalize_mode="off")
        result = make_result(options, files=["/tmp/a.flac"])
        with mock.patch.object(main.service, "execute", return_value=result) as execute:
            code, _, _ = self._run(options)
        self.assertEqual(code, 0)
        self.assertEqual(execute.call_args.args[0].normalize_mode, "off")

    def test_playlist_info_is_fetched_when_not_supplied(self):
        options = self._options()
        result = make_result(options, files=["/tmp/a.flac"])
        with mock.patch.object(main, "get_playlist_info", return_value=INFO) as info:
            with mock.patch.object(main.service, "execute", return_value=result):
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    code = main.run(options)
        self.assertEqual(code, 0)
        info.assert_called_once()
        self.assertIn("Playlist: My Playlist", out.getvalue())

    def test_invalid_playlist_returns_error(self):
        options = self._options()
        with mock.patch.object(
            main, "get_playlist_info", side_effect=RuntimeError("bad url")
        ):
            out, err = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                code = main.run(options, info=None)
        self.assertEqual(code, 1)
        self.assertIn("bad url", err.getvalue())

    def test_service_failure_returns_error(self):
        options = self._options()
        with mock.patch.object(
            main.service, "execute", side_effect=RuntimeError("network down")
        ):
            code, _, err = self._run(options)
        self.assertEqual(code, 1)
        self.assertIn("network down", err)

    def test_partial_failure_returns_exit_code_2(self):
        options = self._options()
        result = make_result(
            options, files=["/tmp/a.flac"], failed=["track 2 unavailable"],
            exit_code=2,
        )
        with mock.patch.object(main.service, "execute", return_value=result):
            code, _, err = self._run(options)
        self.assertEqual(code, 2)
        self.assertIn("track 2 unavailable", err)
        self.assertIn("Some tracks failed", err)

    def test_confirmation_can_cancel_download(self):
        options = self._options()
        with mock.patch.object(main, "prompt_bool", return_value=False):
            with mock.patch.object(main.service, "execute") as execute:
                code, out, _ = self._run(options, confirm=True)
        self.assertEqual(code, 0)
        self.assertIn("Cancelled", out)
        execute.assert_not_called()

    def test_list_tracks_output(self):
        options = self._options(list_tracks=True)
        result = make_result(options)
        with mock.patch.object(main.service, "execute", return_value=result):
            code, out, _ = self._run(options)
        self.assertEqual(code, 0)
        self.assertIn("01. One", out)
        self.assertIn("02. Two", out)

    def test_dry_run_output(self):
        options = self._options(dry_run=True)
        result = make_result(options, planned=["/out/01_One.flac"])
        with mock.patch.object(main.service, "execute", return_value=result):
            code, out, _ = self._run(options)
        self.assertEqual(code, 0)
        self.assertIn("Dry run: 1 track(s)", out)
        self.assertIn("/out/01_One.flac", out)

    def test_json_output(self):
        options = self._options()
        result = make_result(options, files=["/tmp/a.flac"])
        with mock.patch.object(main.service, "execute", return_value=result):
            code, out, _ = self._run(options, json_output=True)
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(out)["info"]["title"], "My Playlist")


class MainDispatchTests(unittest.TestCase):
    def test_url_runs_cli_mode(self):
        with mock.patch.object(main.config, "load_config", return_value={}):
            with mock.patch.object(main, "run", return_value=7) as run:
                code = main.main([
                    "https://youtube.com/playlist?list=PL123",
                    "--format", "mp3",
                    "--no-normalize",
                ])
        self.assertEqual(code, 7)
        options = run.call_args.args[0]
        self.assertEqual(options.fmt, "mp3")
        self.assertEqual(options.normalize_mode, "off")

    def test_no_url_runs_interactive_mode(self):
        with mock.patch.object(main.config, "load_config", return_value={}):
            with mock.patch.object(main, "interactive", return_value=3) as interactive:
                code = main.main([])
        self.assertEqual(code, 3)
        interactive.assert_called_once()

    def test_keyboard_interrupt_returns_error(self):
        with mock.patch.object(main.config, "load_config", return_value={}):
            with mock.patch.object(main, "run", side_effect=KeyboardInterrupt):
                with contextlib.redirect_stdout(io.StringIO()):
                    code = main.main(["https://youtube.com/playlist?list=PL123"])
        self.assertEqual(code, 1)

    def test_save_config(self):
        with mock.patch.object(main.config, "load_config", return_value={}):
            with mock.patch.object(main.config, "save_config") as save:
                with mock.patch.object(main, "interactive", return_value=0):
                    with contextlib.redirect_stdout(io.StringIO()):
                        code = main.main(["--format", "mp3", "--save-config"])
        self.assertEqual(code, 0)
        self.assertEqual(save.call_args.args[0]["format"], "mp3")

    def test_web_flag_starts_server(self):
        with mock.patch.object(main.config, "load_config", return_value={}):
            with mock.patch("webui.run_server", return_value=5) as server:
                code = main.main([
                    "--web", "--host", "0.0.0.0", "--port", "9000",
                    "--open-browser",
                ])
        self.assertEqual(code, 5)
        server.assert_called_once_with("0.0.0.0", 9000, open_browser=True)


class InteractiveTests(unittest.TestCase):
    def test_interactive_collects_options_and_delegates_to_run(self):
        answers = [
            "https://youtube.com/playlist?list=PL123",  # URL
            "",                                          # format: flac
            "2",                                         # template: artist
            "off",                                       # normalization
            "/tmp/out",                                  # output directory
            "",                                          # start download
        ]
        out = io.StringIO()
        with mock.patch("builtins.input", side_effect=answers):
            with mock.patch.object(main, "get_playlist_info", return_value=INFO):
                with mock.patch.object(main, "run", return_value=0) as run:
                    with contextlib.redirect_stdout(out):
                        code = main.interactive(config.default_config())

        self.assertEqual(code, 0)
        self.assertIn("My Playlist", out.getvalue())
        options = run.call_args.args[0]
        self.assertEqual(options.fmt, "flac")
        self.assertEqual(options.template, "num_artist_title")
        self.assertEqual(options.normalize_mode, "off")
        self.assertEqual(options.output, "/tmp/out")

    def test_interactive_requires_url(self):
        with mock.patch("builtins.input", return_value=""):
            with contextlib.redirect_stdout(io.StringIO()):
                code = main.interactive(config.default_config())
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
