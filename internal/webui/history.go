package webui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
)

type historyEntry struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`
	CreatedAt  float64 `json:"created_at"`
	FinishedAt float64 `json:"finished_at"`
	Playlist   string  `json:"playlist,omitempty"`
	ExitCode   int     `json:"exit_code"`
	Error      string  `json:"error,omitempty"`
}

const maxHistory = 50

func historyPath() string {
	dir := os.Getenv("YTM_PLAYLIST_DL_STATE_DIR")
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(cache, "ytm-playlist-dl")
	}
	return filepath.Join(dir, "jobs.json")
}

func loadHistory(path string) []*Job {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []historyEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	jobs := make([]*Job, 0, len(entries))
	for _, entry := range entries {
		result := &model.RunResult{
			Info:     model.PlaylistInfo{Title: entry.Playlist},
			ExitCode: entry.ExitCode,
		}
		jobs = append(jobs, &Job{
			ID:         entry.ID,
			Status:     entry.Status,
			CreatedAt:  entry.CreatedAt,
			FinishedAt: entry.FinishedAt,
			Error:      entry.Error,
			Result:     result,
		})
	}
	return jobs
}

func saveHistory(path string, jobs []*Job) {
	if path == "" {
		return
	}
	entries := make([]historyEntry, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		job.mu.Lock()
		entry := historyEntry{
			ID:         job.ID,
			Status:     job.Status,
			CreatedAt:  job.CreatedAt,
			FinishedAt: job.FinishedAt,
			Error:      job.Error,
		}
		if job.Result != nil {
			entry.Playlist = job.Result.Info.Title
			entry.ExitCode = job.Result.ExitCode
		}
		job.mu.Unlock()
		entries = append(entries, entry)
		if len(entries) >= maxHistory {
			break
		}
	}
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
