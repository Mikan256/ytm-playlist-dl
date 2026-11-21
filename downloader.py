import os
import re
import time
from urllib.parse import parse_qs, urlencode, urlparse, urlunparse

from config import AUDIO_EXTENSIONS

TEMPLATE_MAP = {
    "num_title": "%(playlist_index)02d_%(title)s.%(ext)s",
    "num_artist_title": "%(playlist_index)02d_%(artist,uploader)s_%(title)s.%(ext)s",
}
FORMAT_EXTENSIONS = {"flac": ".flac", "mp3": ".mp3", "original": None}

_INVALID_FILENAME_CHARS = re.compile(r'[\\/:*?"<>|\x00-\x1f]')
_PARTIAL_SUFFIX = re.compile(r"\.part(?:-Frag\d+)?$")


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
    that yt-dlp always receives just the playlist id.
    """
    if not url or not str(url).strip():
        raise ValueError("Playlist URL is required")

    url = str(url).strip()
    if "://" not in url:
        url = "https://" + url

    parsed = urlparse(url)
    host = (parsed.hostname or "").lower()
    video_id = None
    if host == "music.youtube.com" or host == "m.youtube.com":
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

    entries = info.get("entries") or []
    total = sum(1 for entry in entries if entry)
    return {
        "title": sanitize(info.get("title") or "Unknown"),
        "total": total,
    }


def build_output_template(output_dir, template):
    template_string = TEMPLATE_MAP.get(template, TEMPLATE_MAP["num_title"])
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
    """Collect downloaded audio files in playlist order.

    Only real files are returned: partial downloads, thumbnails and other
    sidecar files are ignored. This also fixes the old lexicographic ordering
    (100_... used to sort before 11_...).
    """
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


def download_playlist(url, output_dir, fmt="flac", template="num_title",
                      progress_callback=None, total=None, retries=3, sleep=None):
    if fmt not in FORMAT_EXTENSIONS:
        raise ValueError(f"Unknown output format: {fmt}")

    url = normalise_url(url)
    os.makedirs(output_dir, exist_ok=True)
    cleanup_partial_files(output_dir)

    codec_map = {"flac": "flac", "mp3": "mp3"}
    preferredcodec = codec_map.get(fmt)

    ydl_opts = {
        "format": "bestaudio/best",
        "outtmpl": build_output_template(output_dir, template),
        "quiet": True,
        "noprogress": True,
        "no_warnings": True,
        "ignoreerrors": False,
        "embedmetadata": True,
        "retries": retries,
        "fragment_retries": retries,
    }

    downloaded = {"count": 0}

    if progress_callback:
        def hook(data):
            if data.get("status") != "finished":
                return
            downloaded["count"] += 1
            info = data.get("info_dict") or {}
            index = info.get("playlist_index") or downloaded["count"]
            total_count = total or info.get("playlist_count") or downloaded["count"]
            progress_callback(index, total_count, info.get("title") or "Unknown")

        ydl_opts["progress_hooks"] = [hook]

    if preferredcodec:
        ydl_opts["postprocessors"] = [{
            "key": "FFmpegExtractAudio",
            "preferredcodec": preferredcodec,
        }]

    ydl_cls = _load_yt_dlp().YoutubeDL
    pause = sleep if sleep is not None else time.sleep
    retries = max(1, int(retries))
    last_error = None
    for attempt in range(1, retries + 1):
        try:
            with ydl_cls(ydl_opts) as ydl:
                ydl.download([url])
            last_error = None
            break
        except Exception as exc:
            last_error = exc
            if attempt < retries:
                pause(min(2 ** (attempt - 1), 10))

    if last_error is not None:
        raise RuntimeError(
            f"Download failed after {retries} attempts: {last_error}"
        ) from last_error

    files = collect_output_files(output_dir, fmt)
    if not files:
        raise RuntimeError("Download finished but no tracks were saved")
    return files
