package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateZip(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"01_song.flac":     "audio",
		"sub/02_song.flac": "audio2",
		".manifest.json":   "{}",
	}
	for name, content := range files {
		if err := os.WriteFile(
			filepath.Join(source, name), []byte(content), 0o644,
		); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "out", "playlist.zip")
	created, err := CreateZip(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if created != target {
		t.Fatalf("created = %q, want %q", created, target)
	}
	reader, err := zip.OpenReader(created)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
		if file.Name == "01_song.flac" {
			opened, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			buffer := make([]byte, 5)
			_, _ = opened.Read(buffer)
			opened.Close()
			if string(buffer) != "audio" {
				t.Fatalf("content = %q", buffer)
			}
		}
	}
	for name := range files {
		if !names[filepath.ToSlash(name)] {
			t.Fatalf("missing %q in %v", name, names)
		}
	}
}

func TestCreateZipMissingSource(t *testing.T) {
	if _, err := CreateZip(
		filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "a.zip"),
	); err == nil {
		t.Fatal("expected an error")
	}
}
