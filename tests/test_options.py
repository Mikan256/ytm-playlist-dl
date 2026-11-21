import unittest

import config
from options import RunOptions


class RunOptionsTests(unittest.TestCase):
    def test_from_mapping_maps_format_and_coerces_types(self):
        options = RunOptions.from_mapping({
            "url": "https://youtube.com/playlist?list=PL1",
            "format": "mp3",
            "loudness": "-16",
            "start": "2",
            "end": "5",
            "keep_going": "yes",
            "jobs": "3",
            "template_string": "  ",
        })
        self.assertEqual(options.fmt, "mp3")
        self.assertEqual(options.loudness, -16.0)
        self.assertEqual((options.start, options.end), (2, 5))
        self.assertTrue(options.keep_going)
        self.assertEqual(options.jobs, 3)
        self.assertIsNone(options.template_string)

    def test_to_config_uses_config_key_names(self):
        options = RunOptions(url="url", fmt="mp3", jobs=2)
        values = options.to_config()
        self.assertEqual(values["format"], "mp3")
        self.assertEqual(values["jobs"], 2)
        self.assertNotIn("url", values)
        for key in values:
            self.assertIn(key, config.CONFIG_KEYS)

    def test_validate_accepts_valid_options(self):
        RunOptions(
            url="https://youtube.com/playlist?list=PL1",
            start=1, end=10, jobs=2, concurrent_fragments=4,
        ).validate()

    def test_validate_rejects_bad_values(self):
        cases = [
            {"url": ""},
            {"url": "url", "fmt": "ogg"},
            {"url": "url", "template": "bad"},
            {"url": "url", "loudness": -100},
            {"url": "url", "start": 0},
            {"url": "url", "start": 5, "end": 2},
            {"url": "url", "jobs": 0},
            {"url": "url", "concurrent_fragments": 0},
        ]
        for values in cases:
            with self.subTest(values=values):
                with self.assertRaises(ValueError):
                    RunOptions(**values).validate()


if __name__ == "__main__":
    unittest.main()
