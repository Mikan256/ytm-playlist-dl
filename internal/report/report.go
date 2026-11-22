package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
)

type Report struct {
	App           string                     `json:"app"`
	GeneratedAt   string                     `json:"generated_at"`
	Options       map[string]any             `json:"options"`
	Playlist      model.PlaylistInfo         `json:"playlist"`
	OutputDir     string                     `json:"output_dir"`
	ArchivePath   string                     `json:"archive_path,omitempty"`
	ExportPath    string                     `json:"export_path,omitempty"`
	Download      *model.DownloadResult      `json:"download"`
	Normalization *model.NormalizationResult `json:"normalization"`
	Failed        []string                   `json:"failed"`
	Error         *string                    `json:"error"`
	ExitCode      int                        `json:"exit_code"`
}

func Build(options model.Options, info model.PlaylistInfo, outputDir string,
	download *model.DownloadResult, normalization *model.NormalizationResult,
	runErr error) Report {
	failed := []string{}
	if download != nil {
		failed = append(failed, download.Failed...)
	}
	if normalization != nil {
		failed = append(failed, normalization.Failed...)
	}
	exitCode := 0
	var errorText *string
	if runErr != nil {
		message := runErr.Error()
		errorText = &message
		exitCode = 1
	} else if len(failed) > 0 {
		exitCode = 2
	}
	return Report{
		App:           "ytm-playlist-dl",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Options:       options.ToMap(),
		Playlist:      info,
		OutputDir:     outputDir,
		Download:      download,
		Normalization: normalization,
		Failed:        failed,
		Error:         errorText,
		ExitCode:      exitCode,
	}
}

func Write(path string, value Report) (string, error) {
	target := path
	if absolute, err := filepath.Abs(expandHome(target)); err == nil {
		target = absolute
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	return target, nil
}

func AppendLog(path, message string) error {
	if path == "" {
		return nil
	}
	target := path
	if absolute, err := filepath.Abs(expandHome(target)); err == nil {
		target = absolute
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err = file.WriteString("[" + timestamp + "] " + message + "\n")
	return err
}

func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
