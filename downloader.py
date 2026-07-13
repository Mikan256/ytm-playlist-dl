import os
import re
import yt_dlp
from urllib.parse import urlparse, parse_qs, urlencode, urlunparse


def sanitize(name):
    return re.sub(r'[\\/*?:"<>|]', "", name).strip()


def normalise_url(url):
    parsed = urlparse(url)
    host = parsed.hostname or ""
    if "music.youtube.com" in host:
        host = host.replace("music.", "")
    params = parse_qs(parsed.query)
    list_id = params.get("list")
    if list_id:
        query = urlencode({"list": list_id[0]})
    else:
        query = ""
    return urlunparse(("https", host, parsed.path, parsed.params, query, ""))


def get_playlist_info(url):
    url = normalise_url(url)
    opts = {
        "quiet": True,
        "no_warnings": True,
        "extract_flat": True,
    }
    with yt_dlp.YoutubeDL(opts) as ydl:
        try:
            info = ydl.extract_info(url, download=False)
            entries = info.get("entries") or []
            total = sum(1 for _ in entries)
            return {
                "title": sanitize(info.get("title", "Unknown")),
                "total": total,
            }
        except Exception as e:
            raise RuntimeError(f"Failed to get playlist info: {e}")


def download_playlist(url, output_dir, fmt="flac", template="num_title",
                      progress_callback=None):
    url = normalise_url(url)
    os.makedirs(output_dir, exist_ok=True)

    template_map = {
        "num_title": "%(playlist_index)02d_%(title)s.%(ext)s",
        "num_artist_title":
            "%(playlist_index)02d_%(uploader)s_%(title)s.%(ext)s",
    }
    outtmpl = os.path.join(
        output_dir, template_map.get(template, template_map["num_title"])
    )

    codec_map = {"flac": "flac", "mp3": "mp3"}
    preferredcodec = codec_map.get(fmt)

    ydl_opts = {
        "format": "bestaudio/best",
        "outtmpl": outtmpl,
        "quiet": True,
        "noprogress": True,
        "no_warnings": True,
        "ignoreerrors": True,
        "embedmetadata": True,
    }

    downloaded = {"count": 0}

    if progress_callback:
        def hook(d):
            if d["status"] == "finished":
                downloaded["count"] += 1
                info = d.get("info_dict") or {}
                progress_callback(
                    info.get("playlist_index", downloaded["count"]),
                    info.get("title", "Unknown"),
                )
        ydl_opts["progress_hooks"] = [hook]

    if preferredcodec:
        ydl_opts["postprocessors"] = [{
            "key": "FFmpegExtractAudio",
            "preferredcodec": preferredcodec,
        }]

    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        ydl.download([url])

    ext_filter = {"flac": ".flac", "mp3": ".mp3", "original": None}
    expected_ext = ext_filter.get(fmt)
    files = sorted(
        os.path.join(output_dir, f) for f in os.listdir(output_dir)
        if os.path.isfile(os.path.join(output_dir, f))
        and (expected_ext is None or f.endswith(expected_ext))
    )
    return files
