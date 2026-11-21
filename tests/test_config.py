import os
import tempfile
import unittest

import config


class ConfigTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.path = os.path.join(self._tmp.name, "config.json")

    def test_default_config_has_all_keys(self):
        defaults = config.default_config()
        for key in config.CONFIG_KEYS:
            self.assertIn(key, defaults)

    def test_save_and_load_round_trip(self):
        written = config.save_config(
            {"format": "mp3", "jobs": 4, "unknown": "ignored"}, self.path
        )
        self.assertEqual(written, config.config_path(self.path))
        loaded = config.load_config(self.path)
        self.assertEqual(loaded, {"format": "mp3", "jobs": 4})

    def test_missing_and_invalid_files_yield_empty_dict(self):
        self.assertEqual(config.load_config(self.path), {})
        with open(self.path, "w", encoding="utf-8") as handle:
            handle.write("{not json")
        self.assertEqual(config.load_config(self.path), {})

    def test_env_override(self):
        with unittest.mock.patch.dict(
            os.environ, {"YTM_PLAYLIST_DL_CONFIG": self.path}
        ):
            self.assertEqual(config.config_path(), config.config_path(self.path))


if __name__ == "__main__":
    unittest.main()
