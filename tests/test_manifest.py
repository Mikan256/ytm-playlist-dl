import os
import tempfile
import unittest

import manifest


class ManifestTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.dir = self._tmp.name
        self.file = os.path.join(self.dir, "01_song.flac")
        with open(self.file, "wb") as handle:
            handle.write(b"audio")
        self.path = manifest.manifest_path_for_dir(self.dir)

    def test_path_for_files_uses_common_directory(self):
        self.assertEqual(manifest.manifest_path_for_files([self.file]), self.path)

    def test_mark_and_is_current(self):
        record = manifest.Manifest(self.path)
        self.assertFalse(record.is_current(self.file, "album", -14))
        record.mark(self.file, "album", -14, 6.0)
        record.save()

        loaded = manifest.Manifest(self.path)
        self.assertTrue(loaded.is_current(self.file, "album", -14))
        self.assertFalse(loaded.is_current(self.file, "replaygain", -14))
        self.assertFalse(loaded.is_current(self.file, "album", -16))
        self.assertAlmostEqual(loaded.gain_for(self.file, "album", -14), 6.0)
        self.assertIsNone(loaded.gain_for(self.file, "album", -16))

    def test_changed_file_is_not_current(self):
        record = manifest.Manifest(self.path)
        record.mark(self.file, "album", -14, 6.0)
        record.save()
        with open(self.file, "ab") as handle:
            handle.write(b"changed")

        loaded = manifest.Manifest(self.path)
        self.assertFalse(loaded.is_current(self.file, "album", -14))

    def test_invalid_manifest_is_ignored(self):
        with open(self.path, "w", encoding="utf-8") as handle:
            handle.write("not json")
        record = manifest.Manifest(self.path)
        self.assertFalse(record.is_current(self.file, "album", -14))


if __name__ == "__main__":
    unittest.main()
