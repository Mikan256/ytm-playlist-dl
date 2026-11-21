"""Validated options for one playlist run (CLI, web API and service)."""

from dataclasses import asdict, dataclass, fields
from typing import Optional

import config


def _as_bool(value, default=False):
    if value is None:
        return default
    if isinstance(value, bool):
        return value
    return str(value).strip().lower() in {"1", "true", "yes", "on", "y"}


def _as_optional_int(value):
    if value is None or value == "":
        return None
    return int(value)


def _as_int(value, default):
    if value is None or value == "":
        return default
    return int(value)


def _as_optional_str(value):
    if value is None:
        return None
    text = str(value).strip()
    return text or None


@dataclass
class RunOptions:
    url: str
    fmt: str = config.DEFAULT_FORMAT
    template: str = config.DEFAULT_TEMPLATE
    template_string: Optional[str] = None
    loudness: float = config.DEFAULT_LOUDNESS
    output: str = config.DEFAULT_OUTPUT
    normalize_mode: str = config.DEFAULT_NORMALIZE_MODE
    force_normalize: bool = False
    start: Optional[int] = None
    end: Optional[int] = None
    keep_going: bool = False
    audio_quality: Optional[str] = None
    embed_metadata: bool = False
    embed_thumbnail: bool = False
    concurrent_fragments: int = config.DEFAULT_CONCURRENT_FRAGMENTS
    jobs: int = config.DEFAULT_JOBS
    dry_run: bool = False
    list_tracks: bool = False
    log_file: Optional[str] = None
    report_file: Optional[str] = None

    @classmethod
    def from_mapping(cls, data, url=None):
        """Build options from a JSON request or config dict."""
        data = dict(data or {})
        values = {}
        for field in fields(cls):
            if field.name in data:
                values[field.name] = data[field.name]
            elif field.name == "fmt" and "format" in data:
                values[field.name] = data["format"]
        if url is not None:
            values["url"] = url
        values["fmt"] = str(values.get("fmt", cls.fmt)).lower()
        values["template"] = str(values.get("template", cls.template))
        values["template_string"] = _as_optional_str(values.get("template_string"))
        values["loudness"] = float(values.get("loudness", cls.loudness))
        values["output"] = str(values.get("output", cls.output) or cls.output)
        values["normalize_mode"] = str(
            values.get("normalize_mode", cls.normalize_mode)
        ).lower()
        values["force_normalize"] = _as_bool(values.get("force_normalize"))
        values["start"] = _as_optional_int(values.get("start"))
        values["end"] = _as_optional_int(values.get("end"))
        values["keep_going"] = _as_bool(values.get("keep_going"))
        values["audio_quality"] = _as_optional_str(values.get("audio_quality"))
        values["embed_metadata"] = _as_bool(values.get("embed_metadata"))
        values["embed_thumbnail"] = _as_bool(values.get("embed_thumbnail"))
        values["concurrent_fragments"] = _as_int(
            values.get("concurrent_fragments"), config.DEFAULT_CONCURRENT_FRAGMENTS
        )
        values["jobs"] = _as_int(values.get("jobs"), config.DEFAULT_JOBS)
        values["dry_run"] = _as_bool(values.get("dry_run"))
        values["list_tracks"] = _as_bool(values.get("list_tracks"))
        values["log_file"] = _as_optional_str(values.get("log_file"))
        values["report_file"] = _as_optional_str(values.get("report_file"))
        values["url"] = str(values.get("url", "") or "").strip()
        return cls(**values)

    def validate(self):
        if not self.url:
            raise ValueError("Playlist URL is required")
        if self.fmt not in config.FORMAT_CHOICES:
            raise ValueError(f"Unknown output format: {self.fmt}")
        if self.template not in config.TEMPLATE_CHOICES:
            raise ValueError(f"Unknown template: {self.template}")
        if self.normalize_mode not in config.NORMALIZE_MODES:
            raise ValueError(f"Unknown normalize mode: {self.normalize_mode}")
        if not config.LOUDNESS_MIN <= self.loudness <= config.LOUDNESS_MAX:
            raise ValueError(
                f"Loudness must be between {config.LOUDNESS_MIN:g} and "
                f"{config.LOUDNESS_MAX:g} LUFS"
            )
        if self.start is not None and self.start < 1:
            raise ValueError("--start must be 1 or greater")
        if self.end is not None and self.end < 1:
            raise ValueError("--end must be 1 or greater")
        if self.start is not None and self.end is not None and self.end < self.start:
            raise ValueError("--end must be greater than or equal to --start")
        if self.jobs < 1:
            raise ValueError("--jobs must be 1 or greater")
        if self.concurrent_fragments < 1:
            raise ValueError("--concurrent-fragments must be 1 or greater")
        if self.audio_quality is not None and not self.audio_quality:
            raise ValueError("--audio-quality must not be empty")
        return self

    def to_dict(self):
        return asdict(self)

    def to_config(self):
        values = asdict(self)
        return {
            "format": values["fmt"],
            "template": values["template"],
            "template_string": values["template_string"],
            "loudness": values["loudness"],
            "output": values["output"],
            "normalize_mode": values["normalize_mode"],
            "force_normalize": values["force_normalize"],
            "keep_going": values["keep_going"],
            "audio_quality": values["audio_quality"],
            "embed_metadata": values["embed_metadata"],
            "embed_thumbnail": values["embed_thumbnail"],
            "jobs": values["jobs"],
            "concurrent_fragments": values["concurrent_fragments"],
        }
