package playlistexport

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
)

func Write(format, outputDir string, files []string, entries []model.Entry) (string, error) {
	switch format {
	case "", "none":
		return "", nil
	case "m3u":
		return writeM3U(outputDir, files, entries)
	case "csv":
		return writeCSV(outputDir, files, entries)
	case "json":
		return writeJSON(outputDir, files, entries)
	default:
		return "", fmt.Errorf("unknown export format: %s", format)
	}
}

func writeM3U(outputDir string, files []string, entries []model.Entry) (string, error) {
	path := filepath.Join(outputDir, "playlist.m3u")
	var builder []byte
	builder = append(builder, "#EXTM3U\n"...)
	for index, file := range files {
		entry := entryAt(entries, index)
		duration := -1
		if entry.Duration > 0 {
			duration = int(entry.Duration)
		}
		name := entry.Title
		if name == "" {
			name = filepath.Base(file)
		}
		builder = append(builder, fmt.Sprintf("#EXTINF:%d,%s\n", duration, name)...)
		builder = append(builder, relativePath(outputDir, file)+"\n"...)
	}
	if err := os.WriteFile(path, builder, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func writeCSV(outputDir string, files []string, entries []model.Entry) (string, error) {
	path := filepath.Join(outputDir, "playlist.csv")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"index", "title", "duration", "file"}); err != nil {
		return "", err
	}
	for index, file := range files {
		entry := entryAt(entries, index)
		if err := writer.Write([]string{
			strconv.Itoa(entry.Index),
			entry.Title,
			strconv.Itoa(int(entry.Duration)),
			relativePath(outputDir, file),
		}); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return path, nil
}

func writeJSON(outputDir string, files []string, entries []model.Entry) (string, error) {
	path := filepath.Join(outputDir, "playlist.json")
	type item struct {
		Index    int     `json:"index"`
		Title    string  `json:"title"`
		Duration float64 `json:"duration"`
		File     string  `json:"file"`
	}
	items := make([]item, 0, len(files))
	for index, file := range files {
		entry := entryAt(entries, index)
		items = append(items, item{
			Index: entry.Index, Title: entry.Title, Duration: entry.Duration,
			File: relativePath(outputDir, file),
		})
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func entryAt(entries []model.Entry, index int) model.Entry {
	if index < len(entries) {
		return entries[index]
	}
	return model.Entry{Index: index + 1}
}

func relativePath(outputDir, file string) string {
	rel, err := filepath.Rel(outputDir, file)
	if err != nil {
		return filepath.Base(file)
	}
	return filepath.ToSlash(rel)
}
