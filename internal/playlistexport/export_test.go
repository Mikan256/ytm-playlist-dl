package playlistexport

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
)

func testInput(t *testing.T) (string, []string, []model.Entry) {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "01_First.flac"),
		filepath.Join(dir, "02_Second.flac"),
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries := []model.Entry{
		{Index: 1, Title: "First", Duration: 60},
		{Index: 2, Title: "Second", Duration: 90},
	}
	return dir, files, entries
}

func TestM3U(t *testing.T) {
	dir, files, entries := testInput(t)
	path, err := Write("m3u", dir, files, entries)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.HasPrefix(text, "#EXTM3U") ||
		!strings.Contains(text, "#EXTINF:60,First") ||
		!strings.Contains(text, "01_First.flac") {
		t.Fatalf("unexpected M3U:\n%s", text)
	}
}

func TestCSV(t *testing.T) {
	dir, files, entries := testInput(t)
	path, err := Write("csv", dir, files, entries)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][1] != "First" || rows[2][1] != "Second" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestJSON(t *testing.T) {
	dir, files, entries := testInput(t)
	path, err := Write("json", dir, files, entries)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0]["title"] != "First" {
		t.Fatalf("items = %v", items)
	}
}

func TestNoneAndUnknown(t *testing.T) {
	dir, files, entries := testInput(t)
	path, err := Write("none", dir, files, entries)
	if err != nil || path != "" {
		t.Fatalf("none: %q %v", path, err)
	}
	if _, err := Write("xml", dir, files, entries); err == nil {
		t.Fatal("expected unknown format error")
	}
}
