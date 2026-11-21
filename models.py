"""Small result objects shared by the downloader, normalizer and service."""

from dataclasses import dataclass, field


@dataclass
class DownloadResult:
    files: list
    failed: list = field(default_factory=list)
    total: int = 0
    skipped: list = field(default_factory=list)

    def as_dict(self):
        return {
            "files": list(self.files),
            "failed": list(self.failed),
            "total": self.total,
            "skipped": list(self.skipped),
        }


@dataclass
class NormalizationResult:
    files: list
    normalized: list = field(default_factory=list)
    skipped: list = field(default_factory=list)
    failed: list = field(default_factory=list)
    album_loudness: float | None = None
    gain_db: float | None = None
    mode: str = "album"

    def as_dict(self):
        return {
            "files": list(self.files),
            "normalized": list(self.normalized),
            "skipped": list(self.skipped),
            "failed": list(self.failed),
            "album_loudness": self.album_loudness,
            "gain_db": self.gain_db,
            "mode": self.mode,
        }
