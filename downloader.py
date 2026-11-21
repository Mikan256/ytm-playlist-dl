import os
import re
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from urllib.parse import parse_qs, urlencode, urlparse, urlunparse

from config import AUDIO_EXTENSIONS
from models import DownloadResult

TEMPLATE_MAP = {
    "num_title": "%(playlist_index)02d_%(title)s.%(ext)s",
    "num_artist_title": "%(playlist_index)02d_%(artist,uploader)s_%(title)s.%(ext)s",
}
FORMAT_EXTENSIONS = {"flac": ".flac", "mp3": ".mp3", "original": None}

_INVALID_FILENAME_CHARS = re.compile(r'[\\/:*?"<>|\x00-\x1f]')
_PARTIAL_SUFFIX = re.compile(r"\.part(?:-Frag\d+)?$")


class _ErrorCollector:
    """yt-dlp logger that keeps error messages for partial-failure reports."""

    def __init__(self):
        self.errors = []

    def debug(self, message):
        pass

    def info(self, message):
        pass

    def warning(self, message):
        pass

    def error(self, message):
        self.errors.append(str(message).strip())


def _is_partial_filename(name):
    return bool(_PARTIAL_SUFFIX.search(name)) or name.endswith(".ytdl")


def sanitize(name):
    """Make a string safe to use as a single file/directory name."""
    name = _INVALID_FILENAME_CHARS.sub("", str(name))
    name = re.sub(r"\s+", " ", name).strip().strip(".")
    return name or "Unknown"


def _load_yt_dlp():
    try:
        import yt_dlp
    except ImportError as exc:  # pragma: no cover - depends on the environment
        raise RuntimeError(
            "yt-dlp is not installed. Install it with: "
            "pip install -r requirements.txt"
        ) from exc
    return yt_dlp


def normalise_url(url):
    """Return a canonical youtube.com playlist URL.

    Extra query parameters (radio, tracking, timestamps, ...) are discarded so
    that yt-dlp always receives just the playlist id (and video id for /watch).
    """
    if not url or not str(url).strip():
        raise ValueError("Playlist URL is required")

    url = str(url).strip()
    if "://" not in url:
        url = "https://" + url

    parsed = urlparse(url)
    host = (parsed.hostname or "").lower()
    video_id = None
    if host in {"music.youtube.com", "m.youtube.com"}:
        host = "youtube.com"
    elif host == "www.youtube.com":
        host = "youtube.com"
    elif host == "youtu.be":
        video_id = parsed.path.strip("/")
        host = "youtube.com"
    elif host != "youtube.com":
        raise ValueError(f"Unsupported playlist URL host: {parsed.hostname or url}")

    params = parse_qs(parsed.query)
    list_ids = params.get("list")
    if not list_ids or not list_ids[0].strip():
        raise ValueError("URL must contain a playlist id ('list' parameter)")

    query_params = {"list": list_ids[0].strip()}
    if not video_id and parsed.path.rstrip("/") == "/watch":
        video_ids = params.get("v")
        if video_ids and video_ids[0].strip():
            video_id = video_ids[0].strip()
    if video_id:
        query_params["v"] = video_id
        path = "/watch"
    else:
        path = parsed.path or "/playlist"
    query = urlencode(query_params)
    return urlunparse(("https", host, path, "", query, ""))


def get_playlist_info(url, ydl_factory=None):
    try:
        url = normalise_url(url)
    except ValueError as exc:
        raise RuntimeError(str(exc)) from exc

    opts = {
        "quiet": True,
        "no_warnings": True,
        "extract_flat": True,
    }
    ydl_cls = ydl_factory or _load_yt_dlp().YoutubeDL
    try:
        with ydl_cls(opts) as ydl:
            info = ydl.extract_info(url, download=False)
    except Exception as exc:
        raise RuntimeError(f"Failed to get playlist info: {exc}") from exc

    if not info:
        raise RuntimeError("Failed to get playlist info: empty response")

    entries = []
    for entry in info.get("entries") or []:
        if not entry:
            continue
        index = entry.get("playlist_index") or len(entries) + 1
        entries.append({
            "index": index,
            "id": entry.get("id"),
            "title": sanitize(entry.get("title") or "Unknown"),
            "url": entry.get("url") or entry.get("webpage_url"),
            "duration": entry.get("duration"),
        })

    return {
        "title": sanitize(info.get("title") or "Unknown"),
        "total": len(entries),
        "url": url,
        "entries": entries,
    }


def build_output_template(output_dir, template, template_string=None):
    template_string = template_string or TEMPLATE_MAP.get(
        template, TEMPLATE_MAP["num_title"]
    )
    return os.path.join(output_dir, template_string)


def natural_sort_key(name):
    """Sort names containing numbers in human order (2 before 10)."""
    return [
        int(part) if part.isdigit() else part.casefold()
        for part in re.split(r"(\d+)", name)
    ]


def cleanup_partial_files(output_dir):
    """Remove stale yt-dlp partial downloads before starting/resuming."""
    if not os.path.isdir(output_dir):
        return
    for name in os.listdir(output_dir):
        if _is_partial_filename(name):
            path = os.path.join(output_dir, name)
            if os.path.isfile(path):
                os.unlink(path)


def collect_output_files(output_dir, fmt="flac"):
    """Collect downloaded audio files in playlist order."""
    if fmt not in FORMAT_EXTENSIONS:
        raise ValueError(f"Unknown output format: {fmt}")

    expected_ext = FORMAT_EXTENSIONS[fmt]
    files = []
    for name in os.listdir(output_dir):
        path = os.path.join(output_dir, name)
        if not os.path.isfile(path) or name.startswith("."):
            continue
        if _is_partial_filename(name):
            continue
        ext = os.path.splitext(name)[1].lower()
        if expected_ext is not None:
            if ext != expected_ext:
                continue
        elif ext not in AUDIO_EXTENSIONS:
            continue
        files.append(path)

    return sorted(files, key=lambda path: natural_sort_key(os.path.basename(path)))


def _playlist_items(start, end):
    if start is None and end is None:
        return None
    start = 1 if start is None else start
    if end is None:
        return f"{start}:"
    if start == end:
        return str(start)
    return f"{start}-{end}"


def _postprocessors(fmt, audio_quality):
    if fmt not in {"flac", "mp3"}:
        return []
    postprocessor = {
        "key": "FFmpegExtractAudio",
        "preferredcodec": fmt,
    }
    if fmt == "mp3" and audio_quality:
        postprocessor["preferredquality"] = str(audio_quality)
    return [postprocessor]


def _build_ydl_opts(output_dir, fmt, template, template_string, start, end,
                    keep_going, audio_quality, embed_metadata, embed_thumbnail,
                    concurrent_fragments, retries, logger):
    opts = {
        "format": "bestaudio/best",
        "outtmpl": build_output_template(output_dir, template, template_string),
        "quiet": True,
        "noprogress": True,
        "no_warnings": True,
        "ignoreerrors": bool(keep_going),
        "retries": retries,
        "fragment_retries": retries,
        "concurrent_fragment_downloads": concurrent_fragments,
        "logger": logger,
    }
    items = _playlist_items(start, end)
    if items:
        opts["playlist_items"] = items
    if embed_metadata:
        opts["addmetadata"] = True
    if embed_thumbnail:
        opts["writethumbnail"] = True
        opts["embedthumbnail"] = True
    postprocessors = _postprocessors(fmt, audio_quality)
    if postprocessors:
        opts["postprocessors"] = postprocessors
    return opts


def _make_hook(progress_callback, downloaded, total, lock):
    if not progress_callback:
        return None

    def hook(data):
        if data.get("status") != "finished":
            return
        info = data.get("info_dict") or {}
        with lock:
            downloaded["count"] += 1
            count = downloaded["count"]
        index = info.get("playlist_index") or count
        total_count = total or info.get("playlist_count") or count
        progress_callback(index, total_count, info.get("title") or "Unknown")

    return hook


def _download_once(ydl_cls, opts, url):
    with ydl_cls(opts) as ydl:
        return ydl.download([url])


def _download_serial(ydl_cls, opts, url, retries, pause, keep_going):
    last_error = None
    for attempt in range(1, retries + 1):
        try:
            retcode = _download_once(ydl_cls, opts, url)
            if retcode not in (0, None) and not keep_going:
                raise RuntimeError(f"yt-dlp returned exit code {retcode}")
            return
        except Exception as exc:
            last_error = exc
            if attempt < retries:
                pause(min(2 ** (attempt - 1), 10))
    raise RuntimeError(
        f"Download failed after {retries} attempts: {last_error}"
    ) from last_error


def _download_parallel(ydl_cls, base_opts, url, total, jobs, start, end,
                       retries, pause, keep_going, progress_callback,
                       collector):
    first = start or 1
    last = end or total
    if not last:
        raise ValueError("Cannot run parallel jobs without a known track count")
    indices = list(range(first, last + 1))
    downloaded = {"count": 0}
    lock = threading.Lock()

    def worker(index):
        opts = dict(base_opts)
        opts["playlist_items"] = str(index)
        local = _ErrorCollector()
        opts["logger"] = local
        hook = _make_hook(progress_callback, downloaded, total, lock)
        if hook:
            opts["progress_hooks"] = [hook]
        try:
            _download_serial(
                ydl_cls, opts, url, retries, pause, keep_going=True
            )
        except Exception as exc:
            local.error(str(exc))
        return local.errors

    errors = []
    with ThreadPoolExecutor(max_workers=jobs) as executor:
        futures = [executor.submit(worker, index) for index in indices]
        for future in as_completed(futures):
            errors.extend(future.result())

    if errors:
        with lock:
            collector.errors.extend(errors)
        if not keep_going:
            raise RuntimeError(
                "Download failed for "
                f"{len(errors)} track(s): {errors[0]}"
            )


def download_playlist(url, output_dir, fmt="flac", template="num_title",
                      progress_callback=None, total=None, retries=3, sleep=None,
                      *, start=None, end=None, keep_going=False,
                      template_string=None, audio_quality=None,
                      embed_metadata=False, embed_thumbnail=False,
                      concurrent_fragments=1, jobs=1):
    if fmt not in FORMAT_EXTENSIONS:
        raise ValueError(f"Unknown output format: {fmt}")

    url = normalise_url(url)
    retries = max(1, int(retries))
    jobs = max(1, int(jobs))
    concurrent_fragments = max(1, int(concurrent_fragments))
    os.makedirs(output_dir, exist_ok=True)
    cleanup_partial_files(output_dir)
    existing = set(collect_output_files(output_dir, fmt))

    collector = _ErrorCollector()
    ydl_opts = _build_ydl_opts(
        output_dir, fmt, template, template_string, start, end,
        keep_going, audio_quality, embed_metadata, embed_thumbnail,
        concurrent_fragments, retries, collector,
    )

    downloaded = {"count": 0}
    lock = threading.Lock()
    hook = _make_hook(progress_callback, downloaded, total, lock)
    if hook:
        ydl_opts["progress_hooks"] = [hook]

    ydl_cls = _load_yt_dlp().YoutubeDL
    pause = sleep if sleep is not None else time.sleep

    if jobs > 1:
        if total is None:
            total = get_playlist_info(url)["total"]
        _download_parallel(
            ydl_cls, ydl_opts, url, total, jobs, start, end,
            retries, pause, keep_going, progress_callback, collector,
        )
    else:
        try:
            _download_serial(
                ydl_cls, ydl_opts, url, retries, pause, keep_going
            )
        except Exception as exc:
            if not keep_going:
                raise
            collector.errors.append(str(exc))

    files = collect_output_files(output_dir, fmt)
    failed = list(dict.fromkeys(collector.errors))
    if not files and not (keep_going and failed):
        raise RuntimeError("Download finished but no tracks were saved")
    if failed and not keep_going:
        raise RuntimeError(f"Download failed: {failed[0]}")

    skipped = [path for path in files if path in existing]
    return DownloadResult(
        files=files,
        failed=failed,
        total=total or len(files),
        skipped=skipped,
    )
