#!/usr/bin/env python3
import argparse
import json
import sys

import config
import service
from config import (
    DEFAULT_TEMPLATE,
    FORMAT_CHOICES,
    FORMAT_DESCRIPTIONS,
    LOUDNESS_MAX,
    LOUDNESS_MIN,
    NORMALIZE_MODES,
    TEMPLATE_CHOICES,
    TEMPLATE_LABELS,
)
from downloader import get_playlist_info
from options import RunOptions


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


def positive_int(value):
    try:
        number = int(value)
    except (TypeError, ValueError):
        raise argparse.ArgumentTypeError("must be an integer")
    if number < 1:
        raise argparse.ArgumentTypeError("must be 1 or greater")
    return number


class PlaylistArgumentParser(argparse.ArgumentParser):
    def parse_args(self, args=None, namespace=None):
        parsed = super().parse_args(args, namespace)
        if (parsed.start is not None and parsed.end is not None
                and parsed.end < parsed.start):
            self.error("--end must be greater than or equal to --start")
        return parsed


def build_parser(config_values=None):
    cfg = config.default_config()
    cfg.update(config_values or {})

    parser = PlaylistArgumentParser(
        prog="python main.py",
        description=(
            "Download a public YouTube Music playlist with sequential "
            "filenames, optional EBU R128 normalisation and a browser UI."
        ),
    )
    parser.add_argument(
        "url",
        nargs="?",
        help="YouTube Music playlist URL (omit for interactive/web mode)",
    )
    parser.add_argument(
        "--format", dest="fmt", choices=FORMAT_CHOICES,
        default=cfg["format"],
        help="output format (default: %(default)s)",
    )
    parser.add_argument(
        "--template", choices=TEMPLATE_CHOICES, default=cfg["template"],
        help="file name template (default: %(default)s)",
    )
    parser.add_argument(
        "--template-string", default=cfg["template_string"],
        help="custom yt-dlp output template (overrides --template)",
    )
    parser.add_argument(
        "--loudness", type=loudness_type, default=cfg["loudness"],
        metavar="LUFS",
        help="target loudness in LUFS (default: %(default)s)",
    )
    parser.add_argument(
        "--output", default=cfg["output"],
        help="output directory (default: %(default)s)",
    )
    normalize = parser.add_mutually_exclusive_group()
    normalize.add_argument(
        "--normalize-mode", choices=NORMALIZE_MODES,
        default=cfg["normalize_mode"],
        help="normalisation mode (default: %(default)s)",
    )
    normalize.add_argument(
        "--no-normalize", dest="normalize_mode", action="store_const",
        const="off", help="skip loudness normalisation",
    )
    normalize.add_argument(
        "--replaygain", dest="normalize_mode", action="store_const",
        const="replaygain",
        help="write ReplayGain tags without re-encoding",
    )
    parser.add_argument(
        "--force-normalize", action="store_true",
        default=cfg["force_normalize"],
        help="normalise even when the manifest says it is already done",
    )
    parser.add_argument(
        "--start", type=positive_int, default=None,
        help="first track number to download (1-based)",
    )
    parser.add_argument(
        "--end", type=positive_int, default=None,
        help="last track number to download (inclusive)",
    )
    parser.add_argument(
        "--keep-going", action="store_true", default=cfg["keep_going"],
        help="continue after individual track failures and report them",
    )
    parser.add_argument(
        "--audio-quality", default=cfg["audio_quality"],
        help="MP3 bitrate, e.g. 192 or 320",
    )
    parser.add_argument(
        "--embed-metadata", action="store_true",
        default=cfg["embed_metadata"],
        help="embed title/artist metadata",
    )
    parser.add_argument(
        "--embed-thumbnail", action="store_true",
        default=cfg["embed_thumbnail"],
        help="download and embed cover art",
    )
    parser.add_argument(
        "--jobs", type=positive_int, default=cfg["jobs"],
        help="number of tracks to download in parallel (default: %(default)s)",
    )
    parser.add_argument(
        "--concurrent-fragments", type=positive_int,
        default=cfg["concurrent_fragments"],
        help="parallel fragments per track (default: %(default)s)",
    )
    parser.add_argument(
        "--dry-run", action="store_true",
        help="show the planned output without downloading",
    )
    parser.add_argument(
        "--list-tracks", action="store_true",
        help="only list playlist tracks and exit",
    )
    parser.add_argument(
        "--report-file", default=None, help="write a JSON run report"
    )
    parser.add_argument(
        "--log-file", default=None, help="append a text log"
    )
    parser.add_argument(
        "--json", action="store_true", help="print the JSON report to stdout"
    )
    parser.add_argument(
        "--config", default=None, help="path to a JSON config file"
    )
    parser.add_argument(
        "--save-config", action="store_true",
        help="save the resolved settings to the config file",
    )
    parser.add_argument(
        "--web", action="store_true", help="start the browser UI"
    )
    parser.add_argument(
        "--host", default=config.DEFAULT_HOST, help="web UI bind address"
    )
    parser.add_argument(
        "--port", type=int, default=config.DEFAULT_PORT, help="web UI port"
    )
    parser.add_argument(
        "--open-browser", action="store_true",
        help="open the web UI in the default browser",
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


def _print_tracks(info):
    print(f"\nPlaylist: {info['title']} ({info['total']} tracks)")
    for entry in info.get("entries") or []:
        duration = entry.get("duration")
        suffix = f" ({int(duration)}s)" if duration else ""
        print(f"  {int(entry.get('index', 0)):02d}. {entry.get('title')}{suffix}")
    print()


def _print_planned(planned):
    print(f"\nDry run: {len(planned)} track(s)")
    for path in planned:
        print(f"  {path}")
    print()


def _args_to_options(args):
    return RunOptions(
        url=args.url or "",
        fmt=args.fmt,
        template=args.template,
        template_string=args.template_string,
        loudness=args.loudness,
        output=args.output,
        normalize_mode=args.normalize_mode,
        force_normalize=args.force_normalize,
        start=args.start,
        end=args.end,
        keep_going=args.keep_going,
        audio_quality=args.audio_quality,
        embed_metadata=args.embed_metadata,
        embed_thumbnail=args.embed_thumbnail,
        concurrent_fragments=args.concurrent_fragments,
        jobs=args.jobs,
        dry_run=args.dry_run,
        list_tracks=args.list_tracks,
        log_file=args.log_file,
        report_file=args.report_file,
    )


def _progress_printer(event):
    if event.get("type") == "download":
        download_progress(event["index"], event["total"], event["title"])
    elif event.get("type") == "normalize":
        normalise_progress(
            event["current"], event["total"], event["label"], event["phase"]
        )
    elif event.get("type") == "status" and event.get("message"):
        sys.stderr.write(f"  {event['message']}\n")
        sys.stderr.flush()


def run(options, info=None, confirm=False, show_info=True, json_output=False):
    """Execute options through the shared service. Returns an exit code."""
    options.validate()
    if info is None:
        if not json_output:
            print("Fetching playlist info...")
        try:
            info = get_playlist_info(options.url)
        except Exception as exc:
            print(f"Error: {exc}", file=sys.stderr)
            return 1

    if show_info and not json_output:
        _print_playlist_info(info)

    if confirm and not prompt_bool("Start download?", default=True):
        print("Cancelled.")
        return 0

    try:
        result = service.execute(options, progress=_progress_printer, info=info)
    except Exception as exc:
        print(f"\nError: {exc}", file=sys.stderr)
        return 1

    if json_output:
        print(json.dumps(result.as_dict(), ensure_ascii=False, indent=2))
        return result.exit_code

    if options.list_tracks:
        _print_tracks(result.info)
    elif options.dry_run:
        _print_planned(result.planned)
    else:
        print(f"\nDownloaded {len(result.files)}/{result.info['total']} tracks")
        if result.download and result.download.failed:
            print("Failed tracks:", file=sys.stderr)
            for message in result.download.failed[:10]:
                print(f"  - {message}", file=sys.stderr)
        if result.normalization and result.normalization.normalized:
            print(
                "Normalised "
                f"{len(result.normalization.normalized)} tracks "
                f"({result.normalization.mode})"
            )
        print(f"\nDone! Saved to: {result.output_dir}")
        print(f"Total: {len(result.files)} tracks")
        if result.report_path:
            print(f"Report: {result.report_path}")
        if result.exit_code == 2:
            print("Some tracks failed; see the report for details.",
                  file=sys.stderr)
    return result.exit_code


def interactive(defaults=None):
    cfg = config.default_config()
    cfg.update(defaults or {})
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
    fmt = prompt("Output format", default=cfg["format"], choices=FORMAT_CHOICES)

    template = select_template(cfg["template"])

    normalize_mode = prompt(
        "Normalise mode", default=cfg["normalize_mode"],
        choices=NORMALIZE_MODES,
    )

    loudness = cfg["loudness"]
    if normalize_mode != "off":
        print("  Target loudness (LUFS):")
        print("    -14 = YouTube/Spotify 標準 (おすすめ)")
        print("    -16 = やや控えめ")
        print("    -23 = EBU R128 推奨値 (最も控えめ)")
        loudness = prompt_int(
            "  Value", default=int(cfg["loudness"]),
            min_val=LOUDNESS_MIN, max_val=LOUDNESS_MAX,
        )

    output_base = prompt("Output directory", default=cfg["output"])
    if not prompt_bool("Start download?", default=True):
        print("Cancelled.")
        return 0

    options = RunOptions.from_mapping(cfg, url=url)
    options.fmt = fmt
    options.template = template
    options.normalize_mode = normalize_mode
    options.loudness = float(loudness)
    options.output = output_base
    return run(options, info=info, show_info=False)


def _config_path_from_argv(argv):
    for index, arg in enumerate(argv):
        if arg == "--config" and index + 1 < len(argv):
            return argv[index + 1]
        if arg.startswith("--config="):
            return arg.split("=", 1)[1]
    return None


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    cfg = config.default_config()
    cfg.update(config.load_config(_config_path_from_argv(argv)))
    parser = build_parser(cfg)
    args = parser.parse_args(argv)

    if args.web:
        from webui import run_server
        return run_server(args.host, args.port, open_browser=args.open_browser)

    options = _args_to_options(args)

    if args.save_config:
        saved = config.save_config(options.to_config(), args.config)
        print(f"Saved config to: {saved}")

    try:
        if args.url:
            return run(
                options, info=None, confirm=False, show_info=True,
                json_output=args.json,
            )
        return interactive(cfg)
    except KeyboardInterrupt:
        print("\nInterrupted.")
        return 1


if __name__ == "__main__":
    sys.exit(main())
