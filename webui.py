"""Zero-dependency browser UI and JSON API for ytm-playlist-dl."""

import argparse
import json
import threading
import time
import uuid
import webbrowser
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlparse

import config
import downloader
import service
from options import RunOptions

STATIC_ROOT = Path(__file__).resolve().parent / "web" / "static"
MAX_BODY_SIZE = 1_000_000
CONTENT_TYPES = {
    ".html": "text/html; charset=utf-8",
    ".css": "text/css; charset=utf-8",
    ".js": "application/javascript; charset=utf-8",
    ".json": "application/json; charset=utf-8",
    ".svg": "image/svg+xml",
    ".png": "image/png",
}


class Job:
    def __init__(self, options, runner=None):
        self.id = uuid.uuid4().hex
        self.options = options
        self.runner = runner
        self.status = "queued"
        self.events = []
        self.result = None
        self.error = None
        self.created_at = time.time()
        self.started_at = None
        self.finished_at = None
        self._lock = threading.Lock()

    def add_event(self, event):
        if not isinstance(event, dict):
            return
        with self._lock:
            item = dict(event)
            item["at"] = time.time()
            self.events.append(item)
            if len(self.events) > 2000:
                self.events = self.events[-1000:]

    def snapshot(self):
        with self._lock:
            events = list(self.events)
        return {
            "id": self.id,
            "status": self.status,
            "created_at": self.created_at,
            "started_at": self.started_at,
            "finished_at": self.finished_at,
            "events": events,
            "error": self.error,
            "options": self.options.to_dict(),
            "result": self.result.as_dict() if self.result is not None else None,
        }

    def summary(self):
        return {
            "id": self.id,
            "status": self.status,
            "created_at": self.created_at,
            "finished_at": self.finished_at,
            "error": self.error,
            "playlist": (
                self.result.info.get("title")
                if self.result is not None else None
            ),
            "exit_code": self.result.exit_code if self.result is not None else None,
        }


class JobManager:
    def __init__(self, runner=None):
        self.runner = runner
        self.jobs = {}
        self._lock = threading.Lock()

    def start(self, options):
        job = Job(options, runner=self.runner)
        with self._lock:
            self.jobs[job.id] = job
        thread = threading.Thread(target=self._run, args=(job,), daemon=True)
        thread.start()
        return job

    def _run(self, job):
        job.status = "running"
        job.started_at = time.time()
        job.add_event({"type": "status", "message": "ジョブを開始しました"})
        runner = job.runner or service.execute
        try:
            job.result = runner(job.options, progress=job.add_event)
            job.status = (
                "partial" if job.result.exit_code == 2 else "done"
            )
        except Exception as exc:  # pragma: no cover - exercised via API tests
            job.error = str(exc)
            job.status = "error"
            job.add_event({"type": "error", "message": str(exc)})
        finally:
            job.finished_at = time.time()

    def get(self, job_id):
        with self._lock:
            return self.jobs.get(job_id)

    def list(self):
        with self._lock:
            jobs = list(self.jobs.values())
        return sorted(jobs, key=lambda job: job.created_at, reverse=True)


class RequestHandler(BaseHTTPRequestHandler):
    jobs: JobManager
    static_root: Path = STATIC_ROOT

    def log_message(self, *_args):
        pass

    def _send_json(self, status, payload):
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def _send_error_json(self, status, message):
        self._send_json(status, {"ok": False, "error": message})

    def _read_json(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            raise ValueError("Invalid Content-Length")
        if length > MAX_BODY_SIZE:
            raise ValueError("Request body is too large")
        raw = self.rfile.read(length) if length else b"{}"
        try:
            data = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise ValueError(f"Invalid JSON: {exc}") from exc
        if not isinstance(data, dict):
            raise ValueError("JSON body must be an object")
        return data

    def _serve_static(self, request_path):
        if request_path == "/":
            relative = "index.html"
        elif request_path.startswith("/static/"):
            relative = request_path[len("/static/"):]
        else:
            return None
        relative = unquote(relative)
        target = (self.static_root / relative).resolve()
        try:
            target.relative_to(self.static_root.resolve())
        except ValueError:
            return None
        if not target.is_file():
            return None
        body = target.read_bytes()
        self.send_response(200)
        self.send_header(
            "Content-Type",
            CONTENT_TYPES.get(target.suffix, "application/octet-stream"),
        )
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/api/config":
            defaults = config.default_config()
            defaults.update(config.load_config())
            return self._send_json(200, {
                "ok": True,
                "config": defaults,
                "config_path": str(config.config_path()),
                "formats": config.FORMAT_CHOICES,
                "format_labels": config.FORMAT_DESCRIPTIONS,
                "templates": config.TEMPLATE_CHOICES,
                "template_labels": config.TEMPLATE_LABELS,
                "normalize_modes": config.NORMALIZE_MODES,
                "normalize_labels": config.NORMALIZE_MODE_LABELS,
                "defaults": {
                    "host": config.DEFAULT_HOST,
                    "port": config.DEFAULT_PORT,
                },
            })

        if path == "/api/jobs":
            return self._send_json(200, {
                "ok": True,
                "jobs": [job.summary() for job in self.jobs.list()],
            })

        if path.startswith("/api/jobs/"):
            parts = path.strip("/").split("/")
            if len(parts) >= 3:
                job = self.jobs.get(parts[2])
                if job is None:
                    return self._send_error_json(404, "Job not found")
                if len(parts) == 4 and parts[3] == "report":
                    if job.result is None:
                        return self._send_error_json(404, "Report not ready")
                    return self._send_json(200, {
                        "ok": True, "report": job.result.report,
                    })
                return self._send_json(200, {"ok": True, "job": job.snapshot()})

        if path == "/health":
            return self._send_json(200, {"ok": True, "status": "healthy"})

        if self._serve_static(path) is None:
            self._send_error_json(404, "Not found")

    def do_POST(self):
        path = urlparse(self.path).path
        try:
            payload = self._read_json()
        except ValueError as exc:
            return self._send_error_json(400, str(exc))

        if path == "/api/info":
            url = str(payload.get("url") or "").strip()
            if not url:
                return self._send_error_json(400, "URL is required")
            try:
                info = downloader.get_playlist_info(url)
            except Exception as exc:
                return self._send_error_json(400, str(exc))
            return self._send_json(200, {"ok": True, "info": info})

        if path == "/api/jobs":
            try:
                options = RunOptions.from_mapping(payload)
                options.validate()
            except (TypeError, ValueError) as exc:
                return self._send_error_json(400, str(exc))
            job = self.jobs.start(options)
            return self._send_json(202, {
                "ok": True, "job": job.summary(),
            })

        if path == "/api/config":
            try:
                values = RunOptions.from_mapping(payload)
            except (TypeError, ValueError) as exc:
                return self._send_error_json(400, str(exc))
            try:
                saved = config.save_config(values.to_config())
            except OSError as exc:
                return self._send_error_json(500, str(exc))
            return self._send_json(200, {
                "ok": True, "path": str(saved),
            })

        return self._send_error_json(404, "Not found")


def create_server(host=None, port=None, job_manager=None):
    host = host or config.DEFAULT_HOST
    port = config.DEFAULT_PORT if port is None else int(port)
    manager = job_manager or JobManager()

    class BoundHandler(RequestHandler):
        jobs = manager
        static_root = STATIC_ROOT

    server = ThreadingHTTPServer((host, port), BoundHandler)
    server.daemon_threads = True
    server.jobs = manager
    return server


def run_server(host=None, port=None, open_browser=False):
    server = create_server(host, port)
    url = f"http://{server.server_address[0]}:{server.server_address[1]}/"
    print(f"ytm-playlist-dl web UI: {url}")
    print("Ctrl+C to stop.")
    if open_browser:
        webbrowser.open(url)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping web UI.")
    finally:
        server.server_close()
    return 0


def main(argv=None):
    parser = argparse.ArgumentParser(description="ytm-playlist-dl web UI")
    parser.add_argument("--host", default=config.DEFAULT_HOST)
    parser.add_argument("--port", type=int, default=config.DEFAULT_PORT)
    parser.add_argument("--open-browser", action="store_true")
    args = parser.parse_args(argv)
    return run_server(args.host, args.port, args.open_browser)


if __name__ == "__main__":
    raise SystemExit(main())
