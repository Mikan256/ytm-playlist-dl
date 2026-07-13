#!/usr/bin/env python3
import os
import sys

from config import (
    DEFAULT_FORMAT,
    DEFAULT_LOUDNESS,
    DEFAULT_OUTPUT,
    DEFAULT_TEMPLATE,
    FORMAT_CHOICES,
    FORMAT_DESCRIPTIONS,
    TEMPLATE_CHOICES,
    TEMPLATE_LABELS,
)
from downloader import get_playlist_info, download_playlist
from normalizer import normalize_album


def prompt(msg, default=None, choices=None):
    if choices:
        hint = "/".join(c.upper() if c == default else c for c in choices)
        full = f"{msg} [{hint}]: "
    elif default is not None:
        full = f"{msg} [{default}]: "
    else:
        full = f"{msg}: "
    val = input(full).strip()
    if not val and default is not None:
        return default
    if choices and val.lower() not in [c.lower() for c in choices]:
        print(f"  Please choose from: {', '.join(choices)}")
        return prompt(msg, default, choices)
    return val.lower() if choices else val


def prompt_int(msg, default=None, min_val=None, max_val=None):
    val = input(f"{msg} [{default}]: ").strip()
    if not val:
        return default
    try:
        n = int(val)
        if min_val is not None and n < min_val:
            print(f"  Minimum value is {min_val}")
            return prompt_int(msg, default, min_val, max_val)
        if max_val is not None and n > max_val:
            print(f"  Maximum value is {max_val}")
            return prompt_int(msg, default, min_val, max_val)
        return n
    except ValueError:
        print("  Please enter a number")
        return prompt_int(msg, default, min_val, max_val)


def prompt_bool(msg, default=True):
    hint = "Y/n" if default else "y/N"
    val = input(f"{msg} [{hint}]: ").strip().lower()
    if not val:
        return default
    return val.startswith("y")


def select_template():
    print("\nFile naming:")
    for i, key in enumerate(TEMPLATE_CHOICES, 1):
        print(f"  {i}. {TEMPLATE_LABELS[key]}")
    default_idx = 1
    val = input(f"  Select [{default_idx}]: ").strip()
    if not val:
        return TEMPLATE_CHOICES[default_idx - 1]
    try:
        idx = int(val) - 1
        if 0 <= idx < len(TEMPLATE_CHOICES):
            return TEMPLATE_CHOICES[idx]
    except ValueError:
        pass
    print(f"  Please enter a number 1-{len(TEMPLATE_CHOICES)}")
    return select_template()


def download_progress(index, title):
    sys.stderr.write(f"\r  [{index}] Downloaded: {title}\n")
    sys.stderr.flush()


def normalise_progress(current, total, label, phase):
    phase_label = "Analysing" if phase == "analysing" else "Normalising"
    sys.stderr.write(f"\r  [{current}/{total}] {phase_label}: {label}\n")
    sys.stderr.flush()


def interactive():
    print("=" * 50)
    print("  ytm-playlist-dl")
    print("  YouTube Music Playlist Downloader")
    print("=" * 50)

    url = prompt("Enter playlist URL")
    if not url:
        print("URL is required")
        sys.exit(1)

    print("\nFetching playlist info...")
    try:
        info = get_playlist_info(url)
    except RuntimeError as e:
        print(f"Error: {e}")
        sys.exit(1)

    print(f"\nPlaylist: {info['title']}")
    print(f"Tracks:   {info['total']}")
    print()

    fmt = prompt(
        "Output format",
        default=DEFAULT_FORMAT,
        choices=FORMAT_CHOICES,
    )

    template = select_template()

    do_normalize = prompt_bool("Normalise volume (EBU R128 album mode)", default=True)

    loudness = DEFAULT_LOUDNESS
    if do_normalize:
        print("  Target loudness (LUFS):")
        print("    -14 = YouTube/Spotify 標準 (おすすめ)")
        print("    -16 = やや控えめ")
        print("    -23 = EBU R128 推奨値 (最も控えめ)")
        loudness = prompt_int(
            "  Value",
            default=DEFAULT_LOUDNESS,
            min_val=-30,
            max_val=-5,
        )

    output_base = prompt(
        "Output directory",
        default=DEFAULT_OUTPUT,
    )

    playlist_dir = os.path.join(output_base, info["title"])
    print(f"\nOutput: {playlist_dir}")
    print()

    if not prompt_bool("Start download?", default=True):
        print("Cancelled.")
        sys.exit(0)

    print(f"\nDownloading: {info['title']} ({info['total']} tracks)")
    try:
        files = download_playlist(
            url, playlist_dir,
            fmt=fmt, template=template,
            progress_callback=download_progress,
        )
    except Exception as e:
        print(f"\nDownload failed: {e}")
        sys.exit(1)

    print(f"\n\nDownloaded {len(files)}/{info['total']} tracks")

    if do_normalize and files:
        print(f"\nNormalising volume (album mode, target: {loudness} LUFS)...")
        files = normalize_album(
            files, target_loudness=loudness,
            progress_callback=normalise_progress,
        )
        print()

    print(f"\nDone! Saved to: {playlist_dir}")
    print(f"Total: {len(files)} tracks")


def main():
    try:
        interactive()
    except KeyboardInterrupt:
        print("\nInterrupted.")
        sys.exit(1)


if __name__ == "__main__":
    main()
