package binmgr

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func toolFile(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func writeTool(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, toolFile(name))
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func isolate(t *testing.T, exeDir, cwd string) {
	t.Helper()
	oldExe, oldGetwd := executablePath, getwd
	executablePath = func() (string, error) {
		return filepath.Join(exeDir, "ytm-playlist-dl"), nil
	}
	getwd = func() (string, error) { return cwd, nil }
	t.Cleanup(func() {
		executablePath, getwd = oldExe, oldGetwd
	})
	for _, key := range []string{
		"YTDLP_PATH", "YT_DLP_PATH", "FFMPEG_PATH", "FFPROBE_PATH",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("PATH", t.TempDir())
}

func TestResolveNextToExecutable(t *testing.T) {
	dir := t.TempDir()
	expected := writeTool(t, dir, "yt-dlp")
	isolate(t, dir, t.TempDir())
	got, err := Resolve(YtDlp, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
}

func TestResolveInBinDirectory(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	expected := writeTool(t, bin, "yt-dlp")
	isolate(t, dir, t.TempDir())
	got, err := Resolve(YtDlp, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
}

func TestResolveExplicitAndEnv(t *testing.T) {
	dir := t.TempDir()
	explicit := writeTool(t, dir, "yt-dlp")
	isolate(t, t.TempDir(), t.TempDir())
	got, err := Resolve(YtDlp, explicit)
	if err != nil || got != explicit {
		t.Fatalf("explicit: got %q err %v", got, err)
	}

	envDir := t.TempDir()
	fromEnv := writeTool(t, envDir, "yt-dlp")
	t.Setenv("YTDLP_PATH", fromEnv)
	got, err = Resolve(YtDlp, "")
	if err != nil || got != fromEnv {
		t.Fatalf("env: got %q err %v", got, err)
	}
}

func TestResolveCurrentDirectoryBin(t *testing.T) {
	cwd := t.TempDir()
	bin := filepath.Join(cwd, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	expected := writeTool(t, bin, "ffmpeg")
	isolate(t, t.TempDir(), cwd)
	got, err := Resolve(FFmpeg, "")
	if err != nil || got != expected {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestResolveFromPath(t *testing.T) {
	pathDir := t.TempDir()
	expected := writeTool(t, pathDir, "ffprobe")
	isolate(t, t.TempDir(), t.TempDir())
	t.Setenv("PATH", pathDir)
	got, err := Resolve(FFprobe, "")
	if err != nil || got != expected {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestSibling(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := writeTool(t, dir, "ffmpeg")
	ffprobe := writeTool(t, dir, "ffprobe")
	got, ok := Sibling(ffmpeg, FFprobe)
	if !ok || got != ffprobe {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestResolveNotFound(t *testing.T) {
	isolate(t, t.TempDir(), t.TempDir())
	if _, err := Resolve(YtDlp, ""); err == nil {
		t.Fatal("expected an error")
	}
}
