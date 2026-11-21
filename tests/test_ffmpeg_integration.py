import contextlib
import io
import os
import stat
import tempfile
import unittest
from unittest import mock

import normalizer


FFMPEG_SCRIPT = """#!/bin/sh
printf '%s\\n' "$*" >> "$FAKE_FFMPEG_LOG"
case "$*" in
  *loudnorm*)
    cat >&2 <<'JSON'
{
    "input_i" : "-20.00",
    "input_tp" : "-3.00",
    "input_lra" : "5.00",
    "input_thresh" : "-30.00"
}
JSON
    exit 0
    ;;
esac
input=""
previous=""
last=""
for argument in "$@"; do
  if [ "$previous" = "-i" ]; then input="$argument"; fi
  previous="$argument"
  last="$argument"
done
if [ -n "$input" ]; then cp "$input" "$last"; fi
exit 0
"""

FFPROBE_SCRIPT = """#!/bin/sh
echo 120.0
"""


@unittest.skipUnless(os.name == "posix", "requires a POSIX shell")
class FakeFfmpegIntegrationTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self._stdout = contextlib.redirect_stdout(io.StringIO())
        self._stdout.__enter__()
        self.addCleanup(self._stdout.__exit__, None, None, None)
        self.bin_dir = os.path.join(self._tmp.name, "bin")
        os.makedirs(self.bin_dir)
        self.log = os.path.join(self._tmp.name, "ffmpeg.log")
        for name, content in (
            ("ffmpeg", FFMPEG_SCRIPT),
            ("ffprobe", FFPROBE_SCRIPT),
        ):
            path = os.path.join(self.bin_dir, name)
            with open(path, "w", encoding="utf-8") as handle:
                handle.write(content)
            os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR)
        self.files = []
        for name in ("01_a.flac", "02_b.flac"):
            path = os.path.join(self._tmp.name, name)
            with open(path, "wb") as handle:
                handle.write(b"audio")
            self.files.append(path)

    def _environment(self):
        old_path = os.environ.get("PATH", "")
        return {
            "PATH": self.bin_dir + os.pathsep + old_path,
            "FAKE_FFMPEG_LOG": self.log,
        }

    def _log_text(self):
        if not os.path.exists(self.log):
            return ""
        with open(self.log, encoding="utf-8") as handle:
            return handle.read()

    def test_full_album_normalisation_command_flow(self):
        with mock.patch.dict(os.environ, self._environment()):
            result = normalizer.normalize_album(self.files, target_loudness=-14)

        self.assertEqual(result.normalized, self.files)
        self.assertAlmostEqual(result.gain_db, 6.0)
        log = self._log_text()
        self.assertIn("loudnorm", log)
        self.assertIn("volume=6.000000dB", log)
        self.assertIn("-c:a flac", log)
        self.assertTrue(os.path.exists(
            os.path.join(self._tmp.name, ".ytm-playlist-dl.manifest.json")
        ))

    def test_full_replaygain_command_flow(self):
        with mock.patch.dict(os.environ, self._environment()):
            result = normalizer.normalize_album(
                self.files, target_loudness=-14, mode="replaygain"
            )

        self.assertEqual(result.normalized, self.files)
        log = self._log_text()
        self.assertIn("REPLAYGAIN_TRACK_GAIN=+6.00 dB", log)
        self.assertIn("REPLAYGAIN_ALBUM_GAIN=+6.00 dB", log)
        self.assertIn("-c copy", log)


if __name__ == "__main__":
    unittest.main()
