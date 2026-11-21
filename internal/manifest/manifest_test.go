package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkAndIsCurrent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "01_song.flac")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := PathForDir(dir)
	record := New(path)
	if record.IsCurrent(file, "album", -14) {
		t.Fatal("new manifest should not be current")
	}
	gain := 6.0
	record.Mark(file, "album", -14, &gain)
	if err := record.Save(); err != nil {
		t.Fatal(err)
	}
	loaded := New(path)
	if !loaded.IsCurrent(file, "album", -14) {
		t.Fatal("marked file should be current")
	}
	if loaded.IsCurrent(file, "replaygain", -14) {
		t.Fatal("different mode should not be current")
	}
	if loaded.IsCurrent(file, "album", -16) {
		t.Fatal("different target should not be current")
	}
	got := loaded.GainFor(file, "album", -14)
	if got == nil || *got != 6 {
		t.Fatalf("gain = %v", got)
	}
}

func TestChangedFileIsNotCurrent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "01_song.flac")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := New(PathForDir(dir))
	record.Mark(file, "album", -14, nil)
	if err := os.WriteFile(file, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if record.IsCurrent(file, "album", -14) {
		t.Fatal("changed file should not be current")
	}
}

func TestInvalidManifestIsIgnored(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PathForDir(dir), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := New(PathForDir(dir))
	if record.IsCurrent(file, "album", -14) {
		t.Fatal("invalid manifest should be ignored")
	}
}
