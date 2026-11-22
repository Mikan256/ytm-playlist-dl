package depupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/binmgr"
)

const ReleaseBaseURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download"

func AssetName() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "yt-dlp_linux", nil
	case "linux/arm64":
		return "yt-dlp_linux_aarch64", nil
	case "darwin/amd64", "darwin/arm64":
		return "yt-dlp_macos", nil
	case "windows/amd64", "windows/arm64":
		return "yt-dlp.exe", nil
	default:
		return "", fmt.Errorf(
			"no yt-dlp standalone build for %s/%s",
			runtime.GOOS, runtime.GOARCH,
		)
	}
}

func TargetDir() string {
	if path, err := binmgr.Resolve(binmgr.YtDlp, ""); err == nil {
		return filepath.Dir(path)
	}
	if executable, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(executable), "bin")
	}
	return "bin"
}

// UpdateYtDlp downloads the latest official standalone binary, verifies its
// SHA-256 checksum and atomically replaces the local copy.
func UpdateYtDlp(ctx context.Context, targetDir string) (string, error) {
	return updateFromBase(ctx, ReleaseBaseURL, targetDir)
}

func updateFromBase(ctx context.Context, baseURL, targetDir string) (string, error) {
	asset, err := AssetName()
	if err != nil {
		return "", err
	}
	if targetDir == "" {
		targetDir = TargetDir()
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	sums, err := downloadText(ctx, client, baseURL+"/SHA2-256SUMS")
	if err != nil {
		return "", fmt.Errorf("could not download checksums: %w", err)
	}
	expected, err := checksumFor(sums, asset)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(targetDir, "yt-dlp-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	response, err := request(ctx, client, baseURL+"/"+asset)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", response.StatusCode)
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), response.Body); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return "", fmt.Errorf(
			"checksum mismatch for %s: got %s, want %s",
			asset, actual, expected,
		)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(targetDir, "yt-dlp")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func request(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ytm-playlist-dl-updater")
	return client.Do(req)
}

func downloadText(ctx context.Context, client *http.Client, url string) (string, error) {
	response, err := request(ctx, client, url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func checksumFor(sums, asset string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == asset {
			if len(fields[0]) != 64 {
				return "", errors.New("invalid checksum length")
			}
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s was not found", asset)
}
