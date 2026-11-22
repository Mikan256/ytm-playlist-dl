package depupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestChecksumFor(t *testing.T) {
	hash := strings.Repeat("a", 64)
	sums := hash + "  yt-dlp_linux\n" + hash + " *yt-dlp.exe\n"
	value, err := checksumFor(sums, "yt-dlp.exe")
	if err != nil {
		t.Fatal(err)
	}
	if value != hash {
		t.Fatalf("checksum = %q", value)
	}
	if _, err := checksumFor(sums, "missing"); err == nil {
		t.Fatal("expected missing checksum error")
	}
}

func TestUpdateFromBase(t *testing.T) {
	asset, err := AssetName()
	if err != nil {
		t.Skip(err)
	}
	content := []byte("fake yt-dlp binary")
	sum := sha256.Sum256(content)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA2-256SUMS":
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
		case "/" + asset:
			_, _ = w.Write(content)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	targetDir := t.TempDir()
	path, err := updateFromBase(context.Background(), server.URL, targetDir)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(targetDir, "yt-dlp")
	if runtime.GOOS == "windows" {
		expected += ".exe"
	}
	if path != expected {
		t.Fatalf("path = %q, want %q", path, expected)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("content = %q", got)
	}
}

func TestUpdateChecksumMismatch(t *testing.T) {
	asset, err := AssetName()
	if err != nil {
		t.Skip(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA2-256SUMS":
			fmt.Fprintf(w, "%064d  %s\n", 0, asset)
		case "/" + asset:
			_, _ = w.Write([]byte("tampered"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if _, err := updateFromBase(
		context.Background(), server.URL, t.TempDir(),
	); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}
