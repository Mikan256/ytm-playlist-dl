import json
import math
import os
import shutil
import subprocess
import tempfile

from manifest import Manifest, manifest_path_for_files
from models import NormalizationResult

# Extensions that can be re-encoded while applying the album gain. Using
# "-c:a copy" together with an audio filter is rejected by ffmpeg, so common
# "original" formats must be mapped to an encoder instead.
CODEC_BY_EXTENSION = {
    ".aac": "aac",
    ".flac": "flac",
    ".m4a": "aac",
    ".m4b": "aac",
    ".mka": "libopus",
    ".mp3": "libmp3lame",
    ".mp4": "aac",
    ".oga": "libvorbis",
    ".ogg": "libvorbis",
    ".opus": "libopus",
    ".wav": "pcm_s16le",
    ".webm": "libopus",
}


def codec_for_path(path):
    return CODEC_BY_EXTENSION.get(os.path.splitext(path)[1].lower())


def ensure_ffmpeg(which=None):
    """Raise a helpful error when ffmpeg/ffprobe are unavailable."""
    which = which or shutil.which
    missing = [name for name in ("ffmpeg", "ffprobe") if which(name) is None]
    if missing:
        raise RuntimeError(
            "ffmpeg/ffprobe not found. Install ffmpeg and make sure it is on PATH."
        )


def _finite_float(value, default=None):
    try:
        number = float(value)
    except (TypeError, ValueError):
        return default
    return number if math.isfinite(number) else default


def _get_duration(filepath):
    try:
        result = subprocess.run(
            ["ffprobe", "-v", "error", "-show_entries",
             "format=duration", "-of", "csv=p=0", filepath],
            capture_output=True, text=True,
        )
    except OSError:
        return 0.0
    duration = _finite_float(result.stdout.strip())
    return duration if duration and duration > 0 else 0.0


def parse_loudnorm_json(stderr_text):
    """Extract the final JSON object printed by ffmpeg's loudnorm filter."""
    if not stderr_text:
        return None
    start = stderr_text.rfind("{")
    end = stderr_text.rfind("}")
    if start == -1 or end <= start:
        return None
    try:
        data = json.loads(stderr_text[start:end + 1])
    except json.JSONDecodeError:
        return None
    return data if isinstance(data, dict) else None


def _measure_loudness(filepath, target_i=-14):
    cmd = [
        "ffmpeg", "-hide_banner", "-i", filepath,
        "-af", f"loudnorm=I={target_i}:LRA=11:TP=-1:print_format=json",
        "-f", "null", "-",
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
    except OSError:
        return None
    if result.returncode != 0:
        return None

    data = parse_loudnorm_json(result.stderr)
    if not data:
        return None

    input_i = _finite_float(data.get("input_i"))
    if input_i is None:
        return None

    return {
        "input_i": input_i,
        "input_lra": _finite_float(data.get("input_lra"), 11.0),
        "input_tp": _finite_float(data.get("input_tp"), -1.0),
        "input_thresh": _finite_float(data.get("input_thresh"), -24.0),
        "duration": _get_duration(filepath),
    }


def compute_album_loudness(measurements):
    """Compute duration-weighted integrated loudness for the whole album."""
    valid = []
    for measurement in measurements:
        if not measurement:
            continue
        duration = _finite_float(measurement.get("duration"), 0.0) or 0.0
        loudness = _finite_float(measurement.get("input_i"))
        if duration > 0 and loudness is not None:
            valid.append((duration, loudness))

    if not valid:
        return None

    total_duration = sum(duration for duration, _ in valid)
    if total_duration <= 0:
        return None

    # Integrated loudness is an energy (power) average, not an arithmetic one.
    weighted_power = sum(
        duration * (10 ** (loudness / 10))
        for duration, loudness in valid
    )
    if weighted_power <= 0:
        return None
    return 10 * math.log10(weighted_power / total_duration)


def _apply_gain(input_path, output_path, gain_db, codec=None):
    codec = codec or codec_for_path(output_path)
    if codec is None:
        return False

    filter_str = f"volume={gain_db:.6f}dB"
    if gain_db > 0:
        # Prevent clipping after boosting. level=false keeps the applied gain
        # instead of auto-levelling the whole track to the limiter threshold.
        filter_str += ",alimiter=limit=0.891251:level=false"

    cmd = [
        "ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
        "-i", input_path,
        "-vn",
        "-af", filter_str,
        "-c:a", codec,
        output_path,
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
    except OSError:
        return False
    return result.returncode == 0


def _write_replaygain_tags(input_path, output_path, tags):
    cmd = [
        "ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
        "-i", input_path, "-map_metadata", "0", "-c", "copy",
    ]
    for key, value in tags.items():
        cmd.extend(["-metadata", f"{key}={value}"])
    cmd.append(output_path)
    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
    except OSError:
        return False
    return result.returncode == 0


def _replace_with_output(filepath, writer):
    suffix = os.path.splitext(filepath)[1]
    fd, tmp_path = tempfile.mkstemp(
        suffix=suffix, dir=os.path.dirname(os.path.abspath(filepath))
    )
    os.close(fd)
    try:
        if writer(tmp_path):
            os.replace(tmp_path, filepath)
            return True
        return False
    finally:
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)


def _manifest_for(files, manifest_path):
    return Manifest(manifest_path or manifest_path_for_files(files))


def _split_by_manifest(files, manifest, force, mode, target_loudness):
    if force:
        return list(files), []
    skipped = []
    pending = []
    for filepath in files:
        if manifest.is_current(filepath, mode, target_loudness):
            skipped.append(filepath)
        else:
            pending.append(filepath)
    return pending, skipped


def _save_manifest(manifest):
    try:
        manifest.save()
    except OSError as exc:
        print(f"  Warning: could not save normalisation manifest: {exc}")


def _progress(callback, current, total, filepath, phase):
    if callback:
        callback(
            current, total, os.path.basename(filepath).rsplit(".", 1)[0], phase
        )


def normalize_album(files, target_loudness=-14, progress_callback=None,
                    force=False, manifest_path=None, mode="album"):
    if mode == "off":
        return NormalizationResult(files=list(files), mode="off")
    if mode == "replaygain":
        return normalize_replaygain(
            files, target_loudness=target_loudness,
            progress_callback=progress_callback, force=force,
            manifest_path=manifest_path,
        )
    if mode != "album":
        raise ValueError(f"Unknown normalisation mode: {mode}")

    files = list(files)
    total = len(files)
    if total == 0:
        return NormalizationResult(files=[], mode="album")

    manifest = _manifest_for(files, manifest_path)
    pending, skipped = _split_by_manifest(
        files, manifest, force, "album", target_loudness
    )
    if not pending:
        print(f"  Normalisation already applied to {total} tracks, skipping")
        return NormalizationResult(
            files=files, skipped=skipped, mode="album"
        )

    ensure_ffmpeg()

    previous_gain = None
    for filepath in skipped:
        previous_gain = manifest.gain_for(filepath, "album", target_loudness)
        if previous_gain is not None:
            break

    measure_files = pending if previous_gain is not None else files
    print(f"  Analysing loudness across {len(measure_files)} tracks...")
    measurements = []
    for i, filepath in enumerate(measure_files):
        _progress(progress_callback, i + 1, len(measure_files), filepath,
                  "analysing")
        measurement = _measure_loudness(filepath)
        if measurement:
            measurements.append(measurement)

    album_i = compute_album_loudness(measurements)
    if previous_gain is not None:
        gain = previous_gain
    elif album_i is None:
        print("  Warning: could not measure loudness, skipping normalisation")
        return NormalizationResult(files=files, skipped=skipped, mode="album")
    else:
        gain = target_loudness - album_i
        print(f"  Album integrated loudness: {album_i:+.1f} LUFS")
    print(f"  Applying uniform gain: {gain:+.1f} dB")

    normalized = []
    failed = []
    for i, filepath in enumerate(pending):
        _progress(progress_callback, i + 1, len(pending), filepath,
                  "normalising")
        if _replace_with_output(
            filepath, lambda output, src=filepath: _apply_gain(src, output, gain)
        ):
            normalized.append(filepath)
            manifest.mark(filepath, "album", target_loudness, gain)
        else:
            print(
                "  Warning: failed to normalise "
                f"{os.path.basename(filepath)}, skipping"
            )
            failed.append(filepath)

    _save_manifest(manifest)
    return NormalizationResult(
        files=files,
        normalized=normalized,
        skipped=skipped,
        failed=failed,
        album_loudness=album_i,
        gain_db=gain,
        mode="album",
    )


def normalize_replaygain(files, target_loudness=-14, progress_callback=None,
                         force=False, manifest_path=None):
    files = list(files)
    total = len(files)
    if total == 0:
        return NormalizationResult(files=[], mode="replaygain")

    manifest = _manifest_for(files, manifest_path)
    pending, skipped = _split_by_manifest(
        files, manifest, force, "replaygain", target_loudness
    )
    if not pending:
        print(f"  ReplayGain tags already applied to {total} tracks, skipping")
        return NormalizationResult(
            files=files, skipped=skipped, mode="replaygain"
        )

    ensure_ffmpeg()

    print(f"  Analysing loudness for ReplayGain across {len(pending)} tracks...")
    measurements = {}
    for i, filepath in enumerate(pending):
        _progress(progress_callback, i + 1, len(pending), filepath,
                  "analysing")
        measurement = _measure_loudness(filepath)
        if measurement:
            measurements[filepath] = measurement

    album_i = compute_album_loudness(list(measurements.values()))
    album_gain = target_loudness - album_i if album_i is not None else 0.0
    peaks = [
        10 ** (m["input_tp"] / 20)
        for m in measurements.values()
    ]
    album_peak = max(peaks) if peaks else 1.0

    normalized = []
    failed = []
    for i, filepath in enumerate(pending):
        _progress(progress_callback, i + 1, len(pending), filepath,
                  "normalising")
        measurement = measurements.get(filepath)
        if measurement is None:
            failed.append(filepath)
            continue
        track_gain = target_loudness - measurement["input_i"]
        track_peak = 10 ** (measurement["input_tp"] / 20)
        tags = {
            "REPLAYGAIN_TRACK_GAIN": f"{track_gain:+.2f} dB",
            "REPLAYGAIN_TRACK_PEAK": f"{track_peak:.6f}",
            "REPLAYGAIN_ALBUM_GAIN": f"{album_gain:+.2f} dB",
            "REPLAYGAIN_ALBUM_PEAK": f"{album_peak:.6f}",
        }
        ok = _replace_with_output(
            filepath,
            lambda output, src=filepath, values=tags:
                _write_replaygain_tags(src, output, values),
        )
        if ok:
            normalized.append(filepath)
            manifest.mark(
                filepath, "replaygain", target_loudness, track_gain
            )
        else:
            print(
                "  Warning: failed to write ReplayGain tags for "
                f"{os.path.basename(filepath)}, skipping"
            )
            failed.append(filepath)

    _save_manifest(manifest)
    return NormalizationResult(
        files=files,
        normalized=normalized,
        skipped=skipped,
        failed=failed,
        album_loudness=album_i,
        gain_db=album_gain,
        mode="replaygain",
    )
