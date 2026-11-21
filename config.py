DEFAULT_FORMAT = "flac"
DEFAULT_LOUDNESS = -14
DEFAULT_OUTPUT = "./downloads"
DEFAULT_TEMPLATE = "num_title"

LOUDNESS_MIN = -30
LOUDNESS_MAX = -5

FORMAT_CHOICES = ["flac", "mp3", "original"]
FORMAT_DESCRIPTIONS = {
    "flac": "FLAC (可逆圧縮、高音質)",
    "mp3": "MP3 (汎用性が高い)",
    "original": "元の形式のまま",
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
