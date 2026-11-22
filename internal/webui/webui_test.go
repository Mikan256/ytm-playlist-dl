package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/testutil"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ytm-webui-test-*")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("YTM_PLAYLIST_DL_STATE_DIR", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func testHandler(t *testing.T, runner Runner) *httptest.Server {
	t.Helper()
	return httptest.NewServer(NewHandler(NewJobManager(runner)))
}

func requestJSON(t *testing.T, client *http.Client, method, url string,
	payload any) (int, map[string]any) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	var decoded map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return response.StatusCode, decoded
}

func TestStaticAndConfig(t *testing.T) {
	server := testHandler(t, nil)
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(string(raw), "ytm-playlist-dl") {
		t.Fatalf("index status=%d body=%s", response.StatusCode, raw)
	}

	response, err = server.Client().Get(server.URL + "/static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("static status = %d", response.StatusCode)
	}

	status, data := requestJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/config", nil)
	if status != 200 || data["ok"] != true {
		t.Fatalf("config = %d %+v", status, data)
	}
	if _, ok := data["formats"]; !ok {
		t.Fatalf("config payload = %+v", data)
	}
}

func TestInfoEndpoint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture")
	}
	dir := t.TempDir()
	testutil.WriteExecutable(t, dir, "yt-dlp", `#!/bin/sh
echo '{"title":"Info Playlist","entries":[{"id":"a","title":"A","url":"https://youtube.com/watch?v=a"}]}'
`)
	testutil.PrependPath(t, dir)

	server := testHandler(t, nil)
	defer server.Close()
	status, data := requestJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/info", map[string]any{
			"url": "https://youtube.com/playlist?list=PL1",
		})
	if status != 200 || data["ok"] != true {
		t.Fatalf("info = %d %+v", status, data)
	}
	info := data["info"].(map[string]any)
	if info["title"] != "Info Playlist" {
		t.Fatalf("info = %+v", info)
	}
}

func TestJobsLifecycle(t *testing.T) {
	runner := func(ctx context.Context, options model.Options,
		info *model.PlaylistInfo, progress func(model.Event)) (model.RunResult, error) {
		if progress != nil {
			progress(model.Event{Type: "status", Message: "ok"})
			progress(model.Event{Type: "download", Index: 1, Total: 1, Title: "A", Percent: 100})
		}
		return model.RunResult{
			Options: options,
			Info: model.PlaylistInfo{
				Title: "P", Total: 1,
				Entries: []model.Entry{{Index: 1, Title: "A"}},
			},
			OutputDir: "/tmp/out",
			Files:     []string{"/tmp/out/01_A.flac"},
			Download:  &model.DownloadResult{Files: []string{"/tmp/out/01_A.flac"}, Total: 1},
			Report:    map[string]any{"exit_code": 0},
			ExitCode:  0,
		}, nil
	}
	server := testHandler(t, runner)
	defer server.Close()

	status, data := requestJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/jobs", map[string]any{
			"url":            "https://youtube.com/playlist?list=PL1",
			"fmt":            "original",
			"normalize_mode": "off",
			"output":         "/tmp/out",
		})
	if status != http.StatusAccepted {
		t.Fatalf("start job = %d %+v", status, data)
	}
	jobID := data["job"].(map[string]any)["id"].(string)

	deadline := time.Now().Add(3 * time.Second)
	var job map[string]any
	for time.Now().Before(deadline) {
		status, data = requestJSON(t, server.Client(), http.MethodGet,
			server.URL+"/api/jobs/"+jobID, nil)
		if status != 200 {
			t.Fatalf("get job = %d %+v", status, data)
		}
		job = data["job"].(map[string]any)
		if job["status"] == "done" || job["status"] == "error" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "done" {
		t.Fatalf("job = %+v", job)
	}
	result := job["result"].(map[string]any)
	if len(result["files"].([]any)) != 1 {
		t.Fatalf("result = %+v", result)
	}

	status, data = requestJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/jobs/"+jobID+"/report", nil)
	if status != 200 || data["ok"] != true {
		t.Fatalf("report = %d %+v", status, data)
	}
}

func TestInvalidJobOptions(t *testing.T) {
	server := testHandler(t, nil)
	defer server.Close()
	status, data := requestJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/jobs", map[string]any{
			"url": "https://youtube.com/playlist?list=PL1",
			"fmt": "ogg",
		})
	if status != http.StatusBadRequest || data["ok"] != false {
		t.Fatalf("status=%d data=%+v", status, data)
	}
}

func TestUnknownJob(t *testing.T) {
	server := testHandler(t, nil)
	defer server.Close()
	status, data := requestJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/jobs/nope", nil)
	if status != http.StatusNotFound || data["ok"] != false {
		t.Fatalf("status=%d data=%+v", status, data)
	}
}

func TestArchiveEndpoint(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "output.zip")
	if err := os.WriteFile(archivePath, []byte("PK\x03\x04test"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := func(ctx context.Context, options model.Options,
		info *model.PlaylistInfo, progress func(model.Event)) (model.RunResult, error) {
		return model.RunResult{
			Options:     options,
			Info:        model.PlaylistInfo{Title: "P", Total: 1},
			OutputDir:   "/tmp/out",
			Files:       []string{"/tmp/out/01_A.flac"},
			ArchivePath: archivePath,
			Report:      map[string]any{"exit_code": 0},
		}, nil
	}
	server := testHandler(t, runner)
	defer server.Close()

	status, data := requestJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/jobs", map[string]any{
			"url":            "https://youtube.com/playlist?list=PL1",
			"fmt":            "original",
			"normalize_mode": "off",
			"output":         "/tmp/out",
		})
	if status != http.StatusAccepted {
		t.Fatalf("start job = %d %+v", status, data)
	}
	jobID := data["job"].(map[string]any)["id"].(string)
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, current := requestJSON(t, server.Client(), http.MethodGet,
			server.URL+"/api/jobs/"+jobID, nil)
		job := current["job"].(map[string]any)
		if job["status"] == "done" || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	response, err := server.Client().Get(
		server.URL + "/api/jobs/" + jobID + "/archive",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		t.Fatalf("archive status = %d", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("content type = %q", response.Header.Get("Content-Type"))
	}
	if len(raw) == 0 {
		t.Fatal("archive is empty")
	}
}

func TestJobCancel(t *testing.T) {
	started := make(chan struct{})
	runner := func(ctx context.Context, options model.Options,
		info *model.PlaylistInfo, progress func(model.Event)) (model.RunResult, error) {
		close(started)
		<-ctx.Done()
		return model.RunResult{}, ctx.Err()
	}
	manager := NewJobManagerWithHistory(runner, "")
	job := manager.Start(model.DefaultOptions("https://youtube.com/playlist?list=PL1"))
	<-started
	if !manager.Cancel(job.ID) {
		t.Fatal("cancel returned false")
	}
	deadline := time.Now().Add(2 * time.Second)
	var snapshot map[string]any
	for time.Now().Before(deadline) {
		snapshot = job.snapshot()
		if snapshot["status"] == "cancelled" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if snapshot["status"] != "cancelled" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestJobHistoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	runner := func(ctx context.Context, options model.Options,
		info *model.PlaylistInfo, progress func(model.Event)) (model.RunResult, error) {
		return model.RunResult{
			Options: options,
			Info:    model.PlaylistInfo{Title: "History Playlist", Total: 1},
			Report:  map[string]any{"exit_code": 0},
		}, nil
	}
	manager := NewJobManagerWithHistory(runner, path)
	job := manager.Start(model.DefaultOptions("https://youtube.com/playlist?list=PL1"))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if job.snapshot()["status"] == "done" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	reloaded := NewJobManagerWithHistory(runner, path)
	jobs := reloaded.List()
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d", len(jobs))
	}
	if jobs[0].summary()["playlist"] != "History Playlist" {
		t.Fatalf("summary = %+v", jobs[0].summary())
	}
}

func TestJobManagerError(t *testing.T) {
	manager := NewJobManager(func(context.Context, model.Options,
		*model.PlaylistInfo, func(model.Event)) (model.RunResult, error) {
		return model.RunResult{}, context.DeadlineExceeded
	})
	job := manager.Start(model.DefaultOptions("url"))
	deadline := time.Now().Add(2 * time.Second)
	var snapshot map[string]any
	for time.Now().Before(deadline) {
		snapshot = job.snapshot()
		if snapshot["status"] == "error" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if snapshot["status"] != "error" || snapshot["error"] == nil {
		t.Fatalf("job = %+v", snapshot)
	}
}
