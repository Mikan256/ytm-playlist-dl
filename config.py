DEFAULT_FORMAT = "flac"
DEFAULT_LOUDNESS = -14
DEFAULT_OUTPUT = "./downloads"
DEFAULT_TEMPLATE = "num_title"

FORMAT_CHOICES = ["flac", "mp3", "original"]
FORMAT_DESCRIPTIONS = {
    "flac": "FLAC (可逆圧縮、高音質)",
    "mp3": "MP3 (汎用性が高い)",
    "original": "元の形式のまま",
}

TEMPLATE_CHOICES = ["num_title", "num_artist_title"]
TEMPLATE_LABELS = {
    "num_title": "{番号}_{曲名}",
    "num_artist_title": "{番号}_{アーティスト}_{曲名}",
}
