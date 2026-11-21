import json
import math
import os
import shutil
import subprocess
import tempfile

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


def normalize_album(files, target_loudness=-14, progress_callback=None):
    total = len(files)
    if total == 0:
        return files

    ensure_ffmpeg()

    print(f"  Analysing loudness across {total} tracks...")
    measurements = []
    for i, filepath in enumerate(files):
        if progress_callback:
            progress_callback(
                i + 1, total, os.path.basename(filepath).rsplit(".", 1)[0],
                "analysing",
            )
        measurement = _measure_loudness(filepath)
        if measurement:
            measurements.append(measurement)

    album_i = compute_album_loudness(measurements)
    if album_i is None:
        print("  Warning: could not measure loudness, skipping normalisation")
        return files

    gain = target_loudness - album_i
    print(f"  Album integrated loudness: {album_i:+.1f} LUFS")
    print(f"  Applying uniform gain: {gain:+.1f} dB")

    output_files = []
    for i, filepath in enumerate(files):
        if progress_callback:
            progress_callback(
                i + 1, total, os.path.basename(filepath).rsplit(".", 1)[0],
                "normalising",
            )

        suffix = os.path.splitext(filepath)[1]
        fd, tmp_path = tempfile.mkstemp(
            suffix=suffix, dir=os.path.dirname(os.path.abspath(filepath))
        )
        os.close(fd)
        try:
            ok = _apply_gain(filepath, tmp_path, gain)
            if ok:
                os.replace(tmp_path, filepath)
                output_files.append(filepath)
            else:
                print(
                    "  Warning: failed to normalise "
                    f"{os.path.basename(filepath)}, skipping"
                )
                output_files.append(filepath)
        finally:
            if os.path.exists(tmp_path):
                os.unlink(tmp_path)

    return output_files
