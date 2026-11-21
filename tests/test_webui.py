import http.client
import json
import threading
import time
import unittest
from unittest import mock

import service
import webui
from models import DownloadResult
from options import RunOptions


INFO = {
    "title": "My Playlist",
    "total": 1,
    "entries": [{"index": 1, "title": "One", "duration": 10}],
}


class WebUITests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = webui.create_server("127.0.0.1", 0)
        cls.port = cls.server.server_address[1]
        cls.thread = threading.Thread(
            target=cls.server.serve_forever, daemon=True
        )
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(timeout=2)

    def request(self, method, path, payload=None):
        connection = http.client.HTTPConnection(
            "127.0.0.1", self.port, timeout=5
        )
        body = None
        headers = {}
        if payload is not None:
            body = json.dumps(payload)
            headers["Content-Type"] = "application/json"
        connection.request(method, path, body=body, headers=headers)
        response = connection.getresponse()
        raw = response.read()
        connection.close()
        text = raw.decode("utf-8", errors="replace")
        try:
            data = json.loads(text) if text else {}
        except json.JSONDecodeError:
            data = {"raw": text}
        return response.status, response.getheader("Content-Type"), data

    def wait_for_job(self, job_id, timeout=3):
        deadline = time.time() + timeout
        while time.time() < deadline:
            status, _, data = self.request("GET", f"/api/jobs/{job_id}")
            self.assertEqual(status, 200)
            if data["job"]["status"] in {"done", "partial", "error"}:
                return data["job"]
            time.sleep(0.01)
        self.fail("job did not finish")

    def test_index_and_static_files_are_served(self):
        status, content_type, _ = self.request("GET", "/")
        self.assertEqual(status, 200)
        self.assertIn("text/html", content_type)

        status, content_type, _ = self.request("GET", "/static/app.js")
        self.assertEqual(status, 200)
        self.assertIn("javascript", content_type)

    def test_static_path_traversal_is_blocked(self):
        status, _, data = self.request("GET", "/static/../config.py")
        self.assertEqual(status, 404)
        self.assertFalse(data["ok"])

    def test_config_endpoint(self):
        status, _, data = self.request("GET", "/api/config")
        self.assertEqual(status, 200)
        self.assertIn("formats", data)
        self.assertIn("normalize_modes", data)
        self.assertIn("config", data)

    def test_info_endpoint(self):
        with mock.patch.object(
            webui.downloader, "get_playlist_info", return_value=INFO
        ):
            status, _, data = self.request(
                "POST", "/api/info", {"url": "https://youtube.com/playlist?list=PL1"}
            )
        self.assertEqual(status, 200)
        self.assertEqual(data["info"]["title"], "My Playlist")

    def test_info_endpoint_rejects_missing_url(self):
        status, _, data = self.request("POST", "/api/info", {})
        self.assertEqual(status, 400)
        self.assertFalse(data["ok"])

    def test_start_job_and_fetch_result(self):
        def fake_execute(options, progress=None):
            if progress:
                progress({"type": "status", "message": "ok"})
                progress({"type": "download", "index": 1, "total": 1,
                          "title": "One"})
            return service.RunResult(
                options=options,
                info=INFO,
                output_dir="/tmp/out",
                download=DownloadResult(files=["/tmp/out/01_One.flac"], total=1),
                report={"exit_code": 0, "files": 1},
            )

        payload = {
            "url": "https://youtube.com/playlist?list=PL1",
            "output": "/tmp/out",
            "normalize_mode": "off",
        }
        with mock.patch.object(webui.service, "execute", side_effect=fake_execute):
            status, _, data = self.request("POST", "/api/jobs", payload)
            self.assertEqual(status, 202)
            job = self.wait_for_job(data["job"]["id"])

        self.assertEqual(job["status"], "done")
        self.assertEqual(job["result"]["files"], ["/tmp/out/01_One.flac"])
        self.assertTrue(any(
            event["type"] == "download" for event in job["events"]
        ))

        status, _, report = self.request(
            "GET", f"/api/jobs/{data['job']['id']}/report"
        )
        self.assertEqual(status, 200)
        self.assertEqual(report["report"]["exit_code"], 0)

    def test_start_job_rejects_invalid_options(self):
        status, _, data = self.request("POST", "/api/jobs", {
            "url": "https://youtube.com/playlist?list=PL1",
            "fmt": "ogg",
        })
        self.assertEqual(status, 400)
        self.assertFalse(data["ok"])

    def test_save_config_endpoint(self):
        with mock.patch.object(
            webui.config, "save_config", return_value="/tmp/config.json"
        ) as save:
            status, _, data = self.request("POST", "/api/config", {
                "format": "mp3", "jobs": 2,
            })
        self.assertEqual(status, 200)
        self.assertEqual(data["path"], "/tmp/config.json")
        self.assertEqual(save.call_args.args[0]["format"], "mp3")

    def test_unknown_job_returns_404(self):
        status, _, data = self.request("GET", "/api/jobs/nope")
        self.assertEqual(status, 404)
        self.assertFalse(data["ok"])

    def test_health_endpoint(self):
        status, _, data = self.request("GET", "/health")
        self.assertEqual(status, 200)
        self.assertEqual(data["status"], "healthy")


class JobManagerTests(unittest.TestCase):
    def test_error_status_is_recorded(self):
        options = RunOptions(url="https://youtube.com/playlist?list=PL1")

        def broken(*args, **kwargs):
            raise RuntimeError("boom")

        manager = webui.JobManager(runner=broken)
        job = manager.start(options)
        deadline = time.time() + 2
        while time.time() < deadline and job.status not in {"done", "error"}:
            time.sleep(0.01)
        self.assertEqual(job.status, "error")
        self.assertEqual(job.error, "boom")


if __name__ == "__main__":
    unittest.main()
