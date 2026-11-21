package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
)

func TestBuildAndWrite(t *testing.T) {
	options := model.DefaultOptions("url")
	info := model.PlaylistInfo{Title: "Playlist", Total: 2}
	download := &model.DownloadResult{
		Files:  []string{"a.flac"},
		Failed: []string{"bad track"},
		Total:  2,
	}
	built := Build(options, info, "/out", download, nil, nil)
	if built.ExitCode != 2 {
		t.Fatalf("exit code = %d, want 2", built.ExitCode)
	}
	if len(built.Failed) != 1 || built.Failed[0] != "bad track" {
		t.Fatalf("failed = %v", built.Failed)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	written, err := Write(path, built)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(written)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["app"] != "ytm-playlist-dl" {
		t.Fatalf("app = %v", decoded["app"])
	}
}

func TestAppendLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	if err := AppendLog(path, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(path, "world"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("log is empty")
	}
}
