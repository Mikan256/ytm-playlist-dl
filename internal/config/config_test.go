package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Format != DefaultFormat || cfg.Template != DefaultTemplate {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Loudness != DefaultLoudness || cfg.Jobs != 1 || cfg.NormalizeJobs != 1 {
		t.Fatalf("unexpected numeric defaults: %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.Format = "mp3"
	cfg.Jobs = 4
	written, err := Save(cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	if written != path {
		t.Fatalf("written = %q, want %q", written, path)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Format != "mp3" || loaded.Jobs != 4 {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestLoadMissingAndInvalid(t *testing.T) {
	dir := t.TempDir()
	loaded, err := Load(filepath.Join(dir, "missing.json"))
	if err != nil || loaded.Format != DefaultFormat {
		t.Fatalf("missing config: %+v %v", loaded, err)
	}
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err = Load(path)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if loaded.Format != DefaultFormat {
		t.Fatalf("invalid config should fall back to defaults: %+v", loaded)
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("YTM_PLAYLIST_DL_CONFIG", path)
	if got := Path(""); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
}
