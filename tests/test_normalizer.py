import contextlib
import io
import math
import os
import tempfile
import unittest
from unittest import mock

import normalizer


LOUDNORM_OUTPUT = """
[Parsed_loudnorm_0 @ 0x1234] 
{
    "input_i" : "-23.45",
    "input_tp" : "-3.10",
    "input_lra" : "7.20",
    "input_thresh" : "-33.90",
    "output_i" : "-14.00",
    "output_tp" : "-1.00",
    "output_lra" : "7.20",
    "output_thresh" : "-24.50",
    "normalization_type" : "dynamic",
    "target_offset" : "0.00"
}
"""


class LoudnormParsingTests(unittest.TestCase):
    def test_parses_json_after_ffmpeg_log_lines(self):
        data = normalizer.parse_loudnorm_json(LOUDNORM_OUTPUT)
        self.assertIsNotNone(data)
        self.assertEqual(data["input_i"], "-23.45")

    def test_returns_none_without_json(self):
        self.assertIsNone(normalizer.parse_loudnorm_json("nothing here"))
        self.assertIsNone(normalizer.parse_loudnorm_json(""))
        self.assertIsNone(normalizer.parse_loudnorm_json("{broken"))

    def test_measurement_converts_values_and_gets_duration(self):
        completed = mock.Mock(returncode=0, stderr=LOUDNORM_OUTPUT)
        with mock.patch.object(normalizer.subprocess, "run", return_value=completed):
            with mock.patch.object(normalizer, "_get_duration", return_value=123.4):
                measurement = normalizer._measure_loudness("song.flac")

        self.assertEqual(measurement["input_i"], -23.45)
        self.assertEqual(measurement["input_lra"], 7.2)
        self.assertEqual(measurement["input_tp"], -3.1)
        self.assertEqual(measurement["input_thresh"], -33.9)
        self.assertEqual(measurement["duration"], 123.4)

    def test_measurement_returns_none_when_ffmpeg_fails(self):
        completed = mock.Mock(returncode=1, stderr="")
        with mock.patch.object(normalizer.subprocess, "run", return_value=completed):
            self.assertIsNone(normalizer._measure_loudness("song.flac"))

    def test_non_finite_loudness_is_rejected(self):
        completed = mock.Mock(
            returncode=0,
            stderr='{"input_i": "-inf", "input_lra": "0", "input_tp": "0"}',
        )
        with mock.patch.object(normalizer.subprocess, "run", return_value=completed):
            self.assertIsNone(normalizer._measure_loudness("silence.flac"))


class AlbumLoudnessTests(unittest.TestCase):
    def test_equal_duration_tracks_use_energy_average(self):
        album = normalizer.compute_album_loudness([
            {"input_i": -20, "duration": 10},
            {"input_i": -10, "duration": 10},
        ])
        expected = 10 * math.log10((10 ** -2.0 + 10 ** -1.0) / 2)
        self.assertAlmostEqual(album, expected)

    def test_longer_tracks_are_weighted_more(self):
        album = normalizer.compute_album_loudness([
            {"input_i": -20, "duration": 9},
            {"input_i": -10, "duration": 1},
        ])
        expected = 10 * math.log10((9 * 10 ** -2.0 + 1 * 10 ** -1.0) / 10)
        self.assertAlmostEqual(album, expected)
        self.assertLess(album, -12.5)

    def test_invalid_measurements_are_ignored(self):
        self.assertIsNone(normalizer.compute_album_loudness([]))
        self.assertIsNone(normalizer.compute_album_loudness([
            {"input_i": -20, "duration": 0},
            {"input_i": None, "duration": 10},
            None,
        ]))


class CodecAndFfmpegTests(unittest.TestCase):
    def test_codec_mapping_covers_original_formats(self):
        self.assertEqual(normalizer.codec_for_path("a.flac"), "flac")
        self.assertEqual(normalizer.codec_for_path("a.MP3"), "libmp3lame")
        self.assertEqual(normalizer.codec_for_path("a.m4a"), "aac")
        self.assertEqual(normalizer.codec_for_path("a.mp4"), "aac")
        self.assertEqual(normalizer.codec_for_path("a.opus"), "libopus")
        self.assertEqual(normalizer.codec_for_path("a.webm"), "libopus")
        self.assertIsNone(normalizer.codec_for_path("a.xyz"))

    def test_ensure_ffmpeg_passes_when_both_tools_exist(self):
        normalizer.ensure_ffmpeg(which=lambda name: f"/usr/bin/{name}")

    def test_ensure_ffmpeg_raises_when_tools_are_missing(self):
        with self.assertRaisesRegex(RuntimeError, "ffmpeg/ffprobe not found"):
            normalizer.ensure_ffmpeg(which=lambda name: None)

    def test_apply_gain_uses_encoder_not_stream_copy(self):
        completed = mock.Mock(returncode=0)
        with mock.patch.object(
            normalizer.subprocess, "run", return_value=completed
        ) as run:
            ok = normalizer._apply_gain("in.flac", "out.flac", 3.0)

        self.assertTrue(ok)
        cmd = run.call_args.args[0]
        self.assertIn("-c:a", cmd)
        self.assertEqual(cmd[cmd.index("-c:a") + 1], "flac")
        self.assertNotIn("copy", cmd)
        audio_filter = cmd[cmd.index("-af") + 1]
        self.assertIn("volume=3.000000dB", audio_filter)
        self.assertIn("alimiter", audio_filter)

    def test_apply_gain_negative_does_not_add_limiter(self):
        completed = mock.Mock(returncode=0)
        with mock.patch.object(
            normalizer.subprocess, "run", return_value=completed
        ) as run:
            normalizer._apply_gain("in.mp3", "out.mp3", -2.0)

        cmd = run.call_args.args[0]
        self.assertEqual(cmd[cmd.index("-c:a") + 1], "libmp3lame")
        self.assertNotIn("alimiter", cmd[cmd.index("-af") + 1])

    def test_apply_gain_refuses_unknown_extension(self):
        with mock.patch.object(normalizer.subprocess, "run") as run:
            self.assertFalse(normalizer._apply_gain("in.xyz", "out.xyz", 1.0))
        run.assert_not_called()

    def test_write_replaygain_tags_uses_stream_copy_and_metadata(self):
        completed = mock.Mock(returncode=0)
        with mock.patch.object(
            normalizer.subprocess, "run", return_value=completed
        ) as run:
            ok = normalizer._write_replaygain_tags(
                "in.flac", "out.flac",
                {"REPLAYGAIN_TRACK_GAIN": "+1.00 dB"},
            )

        self.assertTrue(ok)
        cmd = run.call_args.args[0]
        self.assertEqual(cmd[cmd.index("-c") + 1], "copy")
        self.assertIn("REPLAYGAIN_TRACK_GAIN=+1.00 dB", cmd)


class NormalizeAlbumTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self._stdout = contextlib.redirect_stdout(io.StringIO())
        self._stdout.__enter__()
        self.addCleanup(self._stdout.__exit__, None, None, None)
        self.files = []
        for name in ("01_a.flac", "02_b.flac"):
            path = os.path.join(self._tmp.name, name)
            with open(path, "wb") as handle:
                handle.write(b"audio")
            self.files.append(path)

    def test_applies_same_gain_to_every_track(self):
        progress = []

        def fake_gain(input_path, output_path, gain_db):
            with open(output_path, "wb") as handle:
                handle.write(b"normalised")
            return True

        with mock.patch.object(normalizer, "ensure_ffmpeg"):
            with mock.patch.object(
                normalizer, "_measure_loudness",
                return_value={"input_i": -20.0, "duration": 100.0},
            ):
                with mock.patch.object(
                    normalizer, "_apply_gain", side_effect=fake_gain
                ) as apply_gain:
                    result = normalizer.normalize_album(
                        self.files, target_loudness=-14,
                        progress_callback=lambda *args: progress.append(args),
                    )

        self.assertEqual(result.files, self.files)
        self.assertEqual(result.normalized, self.files)
        self.assertEqual(apply_gain.call_count, 2)
        for call in apply_gain.call_args_list:
            self.assertAlmostEqual(call.args[2], 6.0)
        self.assertEqual([entry[3] for entry in progress],
                         ["analysing", "analysing", "normalising", "normalising"])
        for path in self.files:
            with open(path, "rb") as handle:
                self.assertEqual(handle.read(), b"normalised")

    def test_manifest_prevents_duplicate_normalisation(self):
        with mock.patch.object(normalizer, "ensure_ffmpeg"):
            with mock.patch.object(
                normalizer, "_measure_loudness",
                return_value={"input_i": -20.0, "duration": 100.0},
            ):
                with mock.patch.object(
                    normalizer, "_apply_gain", return_value=True
                ) as apply_gain:
                    first = normalizer.normalize_album(self.files)
                    second = normalizer.normalize_album(self.files)

        self.assertEqual(len(first.normalized), 2)
        self.assertEqual(apply_gain.call_count, 2)
        self.assertEqual(second.normalized, [])
        self.assertEqual(second.skipped, self.files)

    def test_force_normalize_ignores_manifest(self):
        with mock.patch.object(normalizer, "ensure_ffmpeg"):
            with mock.patch.object(
                normalizer, "_measure_loudness",
                return_value={"input_i": -20.0, "duration": 100.0},
            ):
                with mock.patch.object(
                    normalizer, "_apply_gain", return_value=True
                ) as apply_gain:
                    normalizer.normalize_album(self.files)
                    forced = normalizer.normalize_album(self.files, force=True)

        self.assertEqual(len(forced.normalized), 2)
        self.assertEqual(apply_gain.call_count, 4)

    def test_replaygain_mode_writes_tags_without_reencoding(self):
        measurement = {"input_i": -20.0, "duration": 100.0, "input_tp": -3.0}
        with mock.patch.object(normalizer, "ensure_ffmpeg"):
            with mock.patch.object(
                normalizer, "_measure_loudness", return_value=measurement
            ):
                with mock.patch.object(
                    normalizer, "_write_replaygain_tags", return_value=True
                ) as tags:
                    result = normalizer.normalize_album(
                        self.files, target_loudness=-14, mode="replaygain",
                    )

        self.assertEqual(result.mode, "replaygain")
        self.assertEqual(result.normalized, self.files)
        self.assertEqual(tags.call_count, 2)
        written = tags.call_args.args[2]
        self.assertEqual(written["REPLAYGAIN_TRACK_GAIN"], "+6.00 dB")
        self.assertIn("REPLAYGAIN_ALBUM_GAIN", written)
        self.assertIn("REPLAYGAIN_TRACK_PEAK", written)

    def test_skips_when_loudness_cannot_be_measured(self):
        with mock.patch.object(normalizer, "ensure_ffmpeg"):
            with mock.patch.object(normalizer, "_measure_loudness", return_value=None):
                with mock.patch.object(normalizer, "_apply_gain") as apply_gain:
                    result = normalizer.normalize_album(self.files)

        self.assertEqual(result.files, self.files)
        apply_gain.assert_not_called()

    def test_failure_keeps_original_file(self):
        with mock.patch.object(normalizer, "ensure_ffmpeg"):
            with mock.patch.object(
                normalizer, "_measure_loudness",
                return_value={"input_i": -20.0, "duration": 100.0},
            ):
                with mock.patch.object(
                    normalizer, "_apply_gain", return_value=False
                ):
                    result = normalizer.normalize_album(self.files)

        self.assertEqual(result.files, self.files)
        self.assertEqual(result.failed, self.files)
        for path in self.files:
            with open(path, "rb") as handle:
                self.assertEqual(handle.read(), b"audio")

    def test_mode_off_does_nothing(self):
        with mock.patch.object(normalizer, "ensure_ffmpeg") as ensure:
            result = normalizer.normalize_album(self.files, mode="off")
        ensure.assert_not_called()
        self.assertEqual(result.mode, "off")
        self.assertEqual(result.files, self.files)

    def test_ffmpeg_check_is_performed(self):
        with mock.patch.object(
            normalizer, "ensure_ffmpeg", side_effect=RuntimeError("missing")
        ):
            with self.assertRaisesRegex(RuntimeError, "missing"):
                normalizer.normalize_album(self.files)


if __name__ == "__main__":
    unittest.main()
