#!/usr/bin/env python3
import argparse
import os
import sys

from config import (
    DEFAULT_FORMAT,
    DEFAULT_LOUDNESS,
    DEFAULT_OUTPUT,
    DEFAULT_TEMPLATE,
    FORMAT_CHOICES,
    FORMAT_DESCRIPTIONS,
    LOUDNESS_MAX,
    LOUDNESS_MIN,
    TEMPLATE_CHOICES,
    TEMPLATE_LABELS,
)
from downloader import download_playlist, get_playlist_info
from normalizer import normalize_album


def loudness_type(value):
    try:
        loudness = float(value)
    except (TypeError, ValueError):
        raise argparse.ArgumentTypeError("loudness must be a number")
    if not LOUDNESS_MIN <= loudness <= LOUDNESS_MAX:
        raise argparse.ArgumentTypeError(
            f"loudness must be between {LOUDNESS_MIN:g} and {LOUDNESS_MAX:g} LUFS"
        )
    return loudness


def build_parser():
    parser = argparse.ArgumentParser(
        prog="python main.py",
        description=(
            "Download a public YouTube Music playlist with sequential "
            "filenames and EBU R128 album loudness normalisation."
        ),
    )
    parser.add_argument(
        "url",
        nargs="?",
        help="YouTube Music playlist URL (omit for interactive mode)",
    )
    parser.add_argument(
        "--format", dest="fmt", choices=FORMAT_CHOICES, default=DEFAULT_FORMAT,
        help="output format (default: %(default)s)",
    )
    parser.add_argument(
        "--template", choices=TEMPLATE_CHOICES, default=DEFAULT_TEMPLATE,
        help="file name template (default: %(default)s)",
    )
    parser.add_argument(
        "--loudness", type=loudness_type, default=DEFAULT_LOUDNESS,
        metavar="LUFS",
        help="target loudness in LUFS (default: %(default)s)",
    )
    parser.add_argument(
        "--output", default=DEFAULT_OUTPUT,
        help="output directory (default: %(default)s)",
    )
    parser.add_argument(
        "--no-normalize", action="store_true",
        help="skip loudness normalisation",
    )
    return parser


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
    hint = f" [{default}]" if default is not None else ""
    val = input(f"{msg}{hint}: ").strip()
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


def select_template(default=DEFAULT_TEMPLATE):
    print("\nFile naming:")
    for i, key in enumerate(TEMPLATE_CHOICES, 1):
        print(f"  {i}. {TEMPLATE_LABELS[key]}")
    default_idx = TEMPLATE_CHOICES.index(default) + 1
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
    return select_template(default)


def download_progress(index, total, title):
    total = total or "?"
    sys.stderr.write(f"\r  [{index}/{total}] Downloaded: {title}\n")
    sys.stderr.flush()


def normalise_progress(current, total, label, phase):
    phase_label = "Analysing" if phase == "analysing" else "Normalising"
    sys.stderr.write(f"\r  [{current}/{total}] {phase_label}: {label}\n")
    sys.stderr.flush()


def _print_playlist_info(info):
    print(f"\nPlaylist: {info['title']}")
    print(f"Tracks:   {info['total']}")
    print()


def run(url, fmt=DEFAULT_FORMAT, template=DEFAULT_TEMPLATE,
        do_normalize=True, loudness=DEFAULT_LOUDNESS,
        output_base=DEFAULT_OUTPUT, info=None, confirm=False,
        show_info=True):
    """Download and normalise one playlist. Returns a process exit code."""
    if info is None:
        print("Fetching playlist info...")
        try:
            info = get_playlist_info(url)
        except Exception as exc:
            print(f"Error: {exc}", file=sys.stderr)
            return 1

    if show_info:
        _print_playlist_info(info)

    output_base = os.path.expanduser(output_base)
    playlist_dir = os.path.join(output_base, info["title"])
    print(f"Output: {playlist_dir}")

    if confirm and not prompt_bool("Start download?", default=True):
        print("Cancelled.")
        return 0

    print(f"\nDownloading: {info['title']} ({info['total']} tracks)")
    try:
        files = download_playlist(
            url, playlist_dir,
            fmt=fmt, template=template,
            progress_callback=download_progress,
            total=info["total"],
        )
    except Exception as exc:
        print(f"\nDownload failed: {exc}", file=sys.stderr)
        return 1

    print(f"\nDownloaded {len(files)}/{info['total']} tracks")

    if do_normalize and files:
        print(f"\nNormalising volume (album mode, target: {loudness} LUFS)...")
        try:
            files = normalize_album(
                files, target_loudness=loudness,
                progress_callback=normalise_progress,
            )
        except Exception as exc:
            print(f"\nNormalisation failed: {exc}", file=sys.stderr)
            return 1
        print()

    print(f"\nDone! Saved to: {playlist_dir}")
    print(f"Total: {len(files)} tracks")
    return 0


def interactive():
    print("=" * 50)
    print("  ytm-playlist-dl")
    print("  YouTube Music Playlist Downloader")
    print("=" * 50)

    url = prompt("Enter playlist URL")
    if not url:
        print("URL is required")
        return 1

    print("\nFetching playlist info...")
    try:
        info = get_playlist_info(url)
    except Exception as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1

    _print_playlist_info(info)

    print("Output format:")
    for key in FORMAT_CHOICES:
        print(f"  {key}: {FORMAT_DESCRIPTIONS[key]}")
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
            min_val=LOUDNESS_MIN,
            max_val=LOUDNESS_MAX,
        )

    output_base = prompt(
        "Output directory",
        default=DEFAULT_OUTPUT,
    )

    if not prompt_bool("Start download?", default=True):
        print("Cancelled.")
        return 0

    return run(
        url, fmt=fmt, template=template,
        do_normalize=do_normalize, loudness=loudness,
        output_base=output_base, info=info, confirm=False,
        show_info=False,
    )


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        if args.url:
            return run(
                args.url, fmt=args.fmt, template=args.template,
                do_normalize=not args.no_normalize,
                loudness=args.loudness, output_base=args.output,
            )
        return interactive()
    except KeyboardInterrupt:
        print("\nInterrupted.")
        return 1


if __name__ == "__main__":
    sys.exit(main())
