import json
import math
import os
import shutil
import subprocess
import tempfile


def _get_duration(filepath):
    result = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries",
         "format=duration", "-of", "csv=p=0", filepath],
        capture_output=True, text=True,
    )
    try:
        return float(result.stdout.strip())
    except (ValueError, TypeError):
        return 0.0


def _parse_loudnorm_json(stderr_text):
    lines = stderr_text.splitlines()
    json_lines = []
    in_json = False
    for line in lines:
        if line.strip().startswith("{"):
            in_json = True
        if in_json:
            json_lines.append(line)
        if line.strip().endswith("}"):
            break
    if not json_lines:
        return None
    try:
        return json.loads("".join(json_lines))
    except json.JSONDecodeError:
        return None


def _measure_loudness(filepath):
    target_i = -14
    cmd = [
        "ffmpeg", "-i", filepath,
        "-af", f"loudnorm=I={target_i}:LRA=11:TP=-1:print_format=json",
        "-f", "null", "-",
    ]
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        return None

    data = _parse_loudnorm_json(result.stderr)
    if not data:
        return None

    duration = _get_duration(filepath)
    try:
        return {
            "input_i": float(data.get("input_i", -14)),
            "input_lra": float(data.get("input_lra", 11)),
            "input_tp": float(data.get("input_tp", -1)),
            "input_thresh": float(data.get("input_thresh", -24)),
            "duration": duration,
        }
    except (ValueError, TypeError, KeyError):
        return None


def _apply_gain(input_path, output_path, gain_db):
    ext = os.path.splitext(output_path)[1].lstrip(".")
    codec = "flac" if ext == "flac" else ("libmp3lame" if ext == "mp3" else "copy")

    if gain_db > 0:
        filter_str = f"volume={gain_db}dB,alimiter=limit=-1dB"
    else:
        filter_str = f"volume={gain_db}dB"

    cmd = [
        "ffmpeg", "-y", "-i", input_path,
        "-af", filter_str,
        "-c:a", codec,
        output_path,
    ]
    result = subprocess.run(cmd, capture_output=True, text=True)
    return result.returncode == 0


def normalize_album(files, target_loudness=-14, progress_callback=None):
    total = len(files)
    if total == 0:
        return files

    print(f"  Analysing loudness across {total} tracks...")
    measurements = []
    for i, f in enumerate(files):
        if progress_callback:
            progress_callback(i + 1, total, os.path.basename(f).rsplit(".", 1)[0], "analysing")
        m = _measure_loudness(f)
        if m:
            measurements.append(m)

    if not measurements:
        print("  Warning: could not measure loudness, skipping normalisation")
        return files

    total_duration = sum(m["duration"] for m in measurements)
    if total_duration > 0:
        weighted_sum = sum(
            m["duration"] * (10 ** (m["input_i"] / 10))
            for m in measurements
        )
        album_i = 10 * math.log10(weighted_sum / total_duration)
    else:
        album_i = target_loudness

    gain = target_loudness - album_i
    print(f"  Album integrated loudness: {album_i:+.1f} LUFS")
    print(f"  Applying uniform gain: {gain:+.1f} dB")

    output_files = []
    for i, f in enumerate(files):
        if progress_callback:
            progress_callback(i + 1, total, os.path.basename(f).rsplit(".", 1)[0], "normalising")
        with tempfile.NamedTemporaryFile(
                suffix=os.path.splitext(f)[1], delete=False
        ) as tmp:
            tmp_path = tmp.name
        ok = _apply_gain(f, tmp_path, gain)
        if ok:
            shutil.move(tmp_path, f)
            output_files.append(f)
        else:
            print(f"  Warning: failed to normalise {os.path.basename(f)}, skipping")
            if os.path.exists(tmp_path):
                os.unlink(tmp_path)
            output_files.append(f)

    return output_files
