"""Default values and persistent user configuration."""

import json
import os
from pathlib import Path

APP_NAME = "ytm-playlist-dl"

DEFAULT_FORMAT = "flac"
DEFAULT_LOUDNESS = -14
DEFAULT_OUTPUT = "./downloads"
DEFAULT_TEMPLATE = "num_title"
DEFAULT_NORMALIZE_MODE = "album"
DEFAULT_JOBS = 1
DEFAULT_CONCURRENT_FRAGMENTS = 1
DEFAULT_HOST = "127.0.0.1"
DEFAULT_PORT = 8765

LOUDNESS_MIN = -30
LOUDNESS_MAX = -5

FORMAT_CHOICES = ["flac", "mp3", "original"]
FORMAT_DESCRIPTIONS = {
    "flac": "FLAC (可逆圧縮、高音質)",
    "mp3": "MP3 (汎用性が高い)",
    "original": "元の形式のまま",
}

NORMALIZE_MODES = ["album", "replaygain", "off"]
NORMALIZE_MODE_LABELS = {
    "album": "アルバムモード（音声を再エンコードして音量統一）",
    "replaygain": "ReplayGainタグ（再エンコードせずタグのみ付与）",
    "off": "正規化しない",
}

# Extensions that can be considered audio when the original format is kept.
AUDIO_EXTENSIONS = {
    ".aac",
    ".flac",
    ".m4a",
    ".m4b",
    ".mka",
    ".mp3",
    ".mp4",
    ".oga",
    ".ogg",
    ".opus",
    ".wav",
    ".webm",
}

TEMPLATE_CHOICES = ["num_title", "num_artist_title"]
TEMPLATE_LABELS = {
    "num_title": "{番号}_{曲名}",
    "num_artist_title": "{番号}_{アーティスト}_{曲名}",
}

# Keys that can be persisted in the user config file.
CONFIG_KEYS = [
    "format",
    "template",
    "template_string",
    "loudness",
    "output",
    "normalize_mode",
    "force_normalize",
    "keep_going",
    "audio_quality",
    "embed_metadata",
    "embed_thumbnail",
    "jobs",
    "concurrent_fragments",
]


def config_path(path=None):
    """Return the config file location, honouring an explicit path or env var."""
    if path:
        return Path(path).expanduser()
    override = os.environ.get("YTM_PLAYLIST_DL_CONFIG")
    if override:
        return Path(override).expanduser()
    base = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config"))
    return base / APP_NAME / "config.json"


def load_config(path=None):
    """Load a JSON config file. Missing or invalid files yield an empty dict."""
    target = config_path(path)
    try:
        data = json.loads(target.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}
    if not isinstance(data, dict):
        return {}
    return {key: value for key, value in data.items() if key in CONFIG_KEYS}


def save_config(values, path=None):
    """Persist known config keys atomically and return the written path."""
    target = config_path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    clean = {key: values[key] for key in CONFIG_KEYS if key in values}
    tmp = target.with_suffix(target.suffix + ".tmp")
    tmp.write_text(
        json.dumps(clean, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    os.replace(tmp, target)
    return target


def default_config():
    return {
        "format": DEFAULT_FORMAT,
        "template": DEFAULT_TEMPLATE,
        "template_string": None,
        "loudness": DEFAULT_LOUDNESS,
        "output": DEFAULT_OUTPUT,
        "normalize_mode": DEFAULT_NORMALIZE_MODE,
        "force_normalize": False,
        "keep_going": False,
        "audio_quality": None,
        "embed_metadata": False,
        "embed_thumbnail": False,
        "jobs": DEFAULT_JOBS,
        "concurrent_fragments": DEFAULT_CONCURRENT_FRAGMENTS,
    }
