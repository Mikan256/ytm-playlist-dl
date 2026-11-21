package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Mikan256/ytm-playlist-dl/internal/binmgr"
	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/urlutil"
)

const ProgressPrefix = "YTM_PROGRESS\t"

const ProgressTemplate = "download:" + ProgressPrefix +
	"%(progress._percent_str)s\t%(info.playlist_index)s\t%(info.title)s"

type Client struct {
	Binary string
}

func Find(explicit string) (string, error) {
	return binmgr.Resolve(binmgr.YtDlp, explicit)
}

func New(explicit string) (*Client, error) {
	binary, err := Find(explicit)
	if err != nil {
		return nil, err
	}
	return &Client{Binary: binary}, nil
}

type rawEntry struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	WebpageURL    string  `json:"webpage_url"`
	Duration      float64 `json:"duration"`
	PlaylistIndex int     `json:"playlist_index"`
}

type rawPlaylist struct {
	Title   string      `json:"title"`
	Entries []*rawEntry `json:"entries"`
}

func (c *Client) Info(ctx context.Context, rawURL string) (model.PlaylistInfo, error) {
	normalized, err := urlutil.Normalize(rawURL)
	if err != nil {
		return model.PlaylistInfo{}, err
	}
	args := []string{
		"--flat-playlist",
		"--dump-single-json",
		"--no-warnings",
		"--no-color",
		"--quiet",
		normalized,
	}
	command := exec.CommandContext(ctx, c.Binary, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return model.PlaylistInfo{}, fmt.Errorf(
			"failed to get playlist info: %s", message,
		)
	}
	var parsed rawPlaylist
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return model.PlaylistInfo{}, fmt.Errorf(
			"failed to parse playlist info: %w", err,
		)
	}
	info := model.PlaylistInfo{
		Title: urlutil.Sanitize(parsed.Title),
		URL:   normalized,
	}
	for _, entry := range parsed.Entries {
		if entry == nil {
			continue
		}
		index := entry.PlaylistIndex
		if index <= 0 {
			index = len(info.Entries) + 1
		}
		entryURL := entry.URL
		if !strings.HasPrefix(entryURL, "http") {
			videoID := entryURL
			if videoID == "" {
				videoID = entry.ID
			}
			if videoID != "" {
				entryURL = "https://www.youtube.com/watch?v=" + videoID
			}
		}
		if entryURL == "" {
			entryURL = entry.WebpageURL
		}
		info.Entries = append(info.Entries, model.Entry{
			Index:    index,
			ID:       entry.ID,
			Title:    urlutil.Sanitize(entry.Title),
			URL:      entryURL,
			Duration: entry.Duration,
		})
	}
	info.Total = len(info.Entries)
	return info, nil
}

type RunResult struct {
	Errors   []string
	Lines    []string
	ExitCode int
	Err      error
}

func (r RunResult) ErrorText() string {
	if len(r.Errors) > 0 {
		return r.Errors[0]
	}
	if r.Err != nil {
		return r.Err.Error()
	}
	return ""
}

// Run executes yt-dlp and parses progress/error lines from stdout+stderr.
func (c *Client) Run(ctx context.Context, args []string,
	onEvent func(model.Event)) RunResult {
	command := exec.CommandContext(ctx, c.Binary, args...)
	reader, writer := io.Pipe()
	command.Stdout = writer
	command.Stderr = writer

	var lines []string
	var errorsFound []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
			if strings.Contains(line, "ERROR:") {
				errorsFound = append(errorsFound, strings.TrimSpace(line))
			}
			if event, ok := ParseProgress(line); ok && onEvent != nil {
				onEvent(event)
			}
		}
	}()

	err := command.Run()
	_ = writer.Close()
	<-done

	result := RunResult{Lines: lines, Errors: errorsFound, Err: err}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	}
	if ctx.Err() != nil {
		result.Err = ctx.Err()
	}
	return result
}

func ParseProgress(line string) (model.Event, bool) {
	index := strings.Index(line, ProgressPrefix)
	if index < 0 {
		return model.Event{}, false
	}
	payload := line[index+len(ProgressPrefix):]
	parts := strings.SplitN(payload, "\t", 3)
	if len(parts) < 3 {
		return model.Event{}, false
	}
	percentText := strings.TrimSpace(parts[0])
	percentText = strings.TrimSuffix(percentText, "%")
	percent, err := strconv.ParseFloat(percentText, 64)
	if err != nil {
		return model.Event{}, false
	}
	event := model.Event{Type: "download", Percent: percent, Title: parts[2]}
	if value, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
		event.Index = value
	}
	return event, true
}
