"""Shared execution pipeline used by both the CLI and the web UI."""

import os
from dataclasses import dataclass, field
from typing import Callable, Optional

import downloader
import normalizer
import report
from models import DownloadResult, NormalizationResult
from options import RunOptions


class ServiceError(RuntimeError):
    pass


@dataclass
class RunResult:
    options: RunOptions
    info: dict
    output_dir: str
    download: Optional[DownloadResult] = None
    normalization: Optional[NormalizationResult] = None
    report: dict = field(default_factory=dict)
    report_path: Optional[str] = None
    planned: list = field(default_factory=list)

    @property
    def files(self):
        return list(self.download.files) if self.download else []

    @property
    def exit_code(self):
        return int(self.report.get("exit_code", 0))

    def as_dict(self):
        return {
            "exit_code": self.exit_code,
            "info": self.info,
            "output_dir": self.output_dir,
            "files": self.files,
            "download": (
                self.download.as_dict() if self.download is not None else None
            ),
            "normalization": (
                self.normalization.as_dict()
                if self.normalization is not None else None
            ),
            "planned": list(self.planned),
            "report_path": self.report_path,
            "report": self.report,
        }


def _emit(progress, event):
    if progress:
        progress(event)


def _log(options, message):
    report.append_log(options.log_file, message)


def _selected_entries(info, start, end):
    entries = info.get("entries") or []
    if not entries:
        return entries
    first = start or 1
    last = end or len(entries)
    return [
        entry for entry in entries
        if first <= int(entry.get("index", 0)) <= last
    ]


def _planned_files(info, output_dir, options):
    extension = {
        "flac": "flac",
        "mp3": "mp3",
        "original": "audio",
    }.get(options.fmt, "audio")
    planned = []
    for entry in _selected_entries(info, options.start, options.end):
        title = downloader.sanitize(entry.get("title") or "Unknown")
        planned.append(
            os.path.join(
                output_dir,
                f"{int(entry.get('index', len(planned) + 1)):02d}_{title}.{extension}",
            )
        )
    return planned


def execute(options: RunOptions, progress: Optional[Callable] = None,
            info=None) -> RunResult:
    """Run one download job. Raises on fatal errors."""
    options.validate()

    _emit(progress, {"type": "status", "message": "プレイリスト情報を取得中..."})
    if info is None:
        info = downloader.get_playlist_info(options.url)
    output_dir = os.path.join(os.path.expanduser(options.output), info["title"])

    if options.list_tracks:
        run_report = report.build_report(options, info, output_dir)
        result = RunResult(
            options=options, info=info, output_dir=output_dir,
            report=run_report, report_path=_write_report(options, run_report),
        )
        return result

    if options.dry_run:
        planned = _planned_files(info, output_dir, options)
        _emit(progress, {
            "type": "status",
            "message": f"ドライラン: {len(planned)} 曲を {output_dir} に保存予定",
        })
        run_report = report.build_report(options, info, output_dir)
        result = RunResult(
            options=options, info=info, output_dir=output_dir,
            report=run_report, planned=planned,
            report_path=_write_report(options, run_report),
        )
        return result

    _log(options, f"download start: {options.url}")
    _emit(progress, {"type": "status", "message": "ダウンロードを開始しました"})
    download = downloader.download_playlist(
        options.url, output_dir,
        fmt=options.fmt,
        template=options.template,
        progress_callback=lambda index, total, title: _emit(progress, {
            "type": "download", "index": index, "total": total, "title": title,
        }),
        total=info["total"],
        start=options.start,
        end=options.end,
        keep_going=options.keep_going,
        template_string=options.template_string,
        audio_quality=options.audio_quality,
        embed_metadata=options.embed_metadata,
        embed_thumbnail=options.embed_thumbnail,
        concurrent_fragments=options.concurrent_fragments,
        jobs=options.jobs,
    )
    _log(
        options,
        f"download finished: {len(download.files)} files, "
        f"{len(download.failed)} failed",
    )

    normalization = None
    if options.normalize_mode != "off" and download.files:
        _emit(progress, {
            "type": "status",
            "message": f"音量処理を開始しました（{options.normalize_mode}）",
        })
        normalization = normalizer.normalize_album(
            download.files,
            target_loudness=options.loudness,
            progress_callback=lambda current, total, label, phase: _emit(
                progress, {
                    "type": "normalize", "current": current, "total": total,
                    "label": label, "phase": phase,
                }
            ),
            force=options.force_normalize,
            mode=options.normalize_mode,
        )
        _log(
            options,
            "normalisation finished: "
            f"{len(normalization.normalized)} processed, "
            f"{len(normalization.skipped)} skipped, "
            f"{len(normalization.failed)} failed",
        )

    run_report = report.build_report(
        options, info, output_dir, download, normalization
    )
    report_path = _write_report(options, run_report)
    result = RunResult(
        options=options,
        info=info,
        output_dir=output_dir,
        download=download,
        normalization=normalization,
        report=run_report,
        report_path=report_path,
    )
    _emit(progress, {
        "type": "status",
        "message": (
            "完了しました"
            if result.exit_code == 0
            else "一部の曲でエラーが発生しました"
        ),
        "exit_code": result.exit_code,
    })
    return result


def _write_report(options, run_report):
    if not options.report_file:
        return None
    return report.write_report(options.report_file, run_report)
