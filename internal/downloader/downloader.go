package downloader

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/binmgr"
	"github.com/Mikan256/ytm-playlist-dl/internal/config"
	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/urlutil"
	"github.com/Mikan256/ytm-playlist-dl/internal/ytdlp"
)

var templateMap = map[string]string{
	"num_title":        "%(playlist_index)02d_%(title)s.%(ext)s",
	"num_artist_title": "%(playlist_index)02d_%(artist,uploader)s_%(title)s.%(ext)s",
}

var retryDelay = func(attempt int) time.Duration {
	return time.Duration(1<<(attempt-1)) * time.Second
}

type Downloader struct {
	client *ytdlp.Client
}

func New(ytdlpPath string) (*Downloader, error) {
	client, err := ytdlp.New(ytdlpPath)
	if err != nil {
		return nil, err
	}
	return &Downloader{client: client}, nil
}

func (d *Downloader) Info(ctx context.Context, rawURL string) (model.PlaylistInfo, error) {
	return d.client.Info(ctx, rawURL)
}

func BaseTemplate(template string, custom *string) string {
	if custom != nil && strings.TrimSpace(*custom) != "" {
		return *custom
	}
	if value, ok := templateMap[template]; ok {
		return value
	}
	return templateMap["num_title"]
}

func BuildTemplate(outputDir, template string, custom *string, index int) string {
	value := BaseTemplate(template, custom)
	if index > 0 {
		value = ApplyIndex(value, index)
	}
	return filepath.Join(outputDir, value)
}

func ApplyIndex(template string, index int) string {
	replacements := []struct {
		old string
		new string
	}{
		{"%(playlist_index)02d", fmt.Sprintf("%02d", index)},
		{"%(playlist_index)03d", fmt.Sprintf("%03d", index)},
		{"%(playlist_index)s", strconv.Itoa(index)},
	}
	result := template
	for _, replacement := range replacements {
		result = strings.ReplaceAll(result, replacement.old, replacement.new)
	}
	if result == template && index > 0 {
		// Parallel per-track downloads cannot use playlist_index. Prefix the
		// index so custom templates still produce unique names.
		result = fmt.Sprintf("%02d_%s", index, template)
	}
	return result
}

func PlaylistItems(start, end *int) string {
	if start == nil && end == nil {
		return ""
	}
	first := 1
	if start != nil {
		first = *start
	}
	if end == nil {
		return fmt.Sprintf("%d:", first)
	}
	if first == *end {
		return strconv.Itoa(first)
	}
	return fmt.Sprintf("%d-%d", first, *end)
}

func SelectedEntries(info model.PlaylistInfo, start, end *int) []model.Entry {
	if len(info.Entries) == 0 {
		return nil
	}
	if !info.IsPlaylist {
		return append([]model.Entry{}, info.Entries...)
	}
	first := 1
	if start != nil {
		first = *start
	}
	last := info.Total
	if end != nil {
		last = *end
	}
	selected := make([]model.Entry, 0, len(info.Entries))
	for _, entry := range info.Entries {
		if entry.Index >= first && entry.Index <= last {
			selected = append(selected, entry)
		}
	}
	return selected
}

func IsPartial(name string) bool {
	if strings.HasSuffix(name, ".ytdl") {
		return true
	}
	if strings.HasSuffix(name, ".part") {
		return true
	}
	if index := strings.LastIndex(name, ".part-Frag"); index >= 0 {
		suffix := name[index+len(".part-Frag"):]
		if suffix != "" {
			if _, err := strconv.Atoi(suffix); err == nil {
				return true
			}
		}
	}
	return false
}

func CleanupPartials(outputDir string) error {
	if _, err := os.Stat(outputDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return filepath.WalkDir(outputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if IsPartial(entry.Name()) {
			return os.Remove(path)
		}
		return nil
	})
}

func CollectFiles(outputDir, format string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(outputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != outputDir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || IsPartial(name) {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(name))
		switch format {
		case "flac":
			if extension != ".flac" {
				return nil
			}
		case "mp3":
			if extension != ".mp3" {
				return nil
			}
		case "original":
			if !config.AudioExtensions[extension] {
				return nil
			}
		default:
			return fmt.Errorf("unknown output format: %s", format)
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(files, func(i, j int) bool {
		return naturalLess(filepath.Base(files[i]), filepath.Base(files[j]))
	})
	return files, nil
}

func naturalLess(left, right string) bool {
	left = strings.ToLower(left)
	right = strings.ToLower(right)
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		if isDigit(left[i]) && isDigit(right[j]) {
			leftStart, rightStart := i, j
			for i < len(left) && isDigit(left[i]) {
				i++
			}
			for j < len(right) && isDigit(right[j]) {
				j++
			}
			leftNumber := strings.TrimLeft(left[leftStart:i], "0")
			rightNumber := strings.TrimLeft(right[rightStart:j], "0")
			if len(leftNumber) != len(rightNumber) {
				return len(leftNumber) < len(rightNumber)
			}
			if leftNumber != rightNumber {
				return leftNumber < rightNumber
			}
			continue
		}
		if left[i] != right[j] {
			return left[i] < right[j]
		}
		i++
		j++
	}
	return len(left) < len(right)
}

func isDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func (d *Downloader) buildArgs(options model.Options, outputTemplate,
	playlistItems, url string) []string {
	args := []string{
		"--newline",
		"--no-warnings",
		"--no-color",
		"--progress-template", ytdlp.ProgressTemplate,
		"-f", "bestaudio/best",
		"-o", outputTemplate,
		"--retries", "3",
		"--fragment-retries", "3",
		"--concurrent-fragments", strconv.Itoa(options.ConcurrentFragments),
	}
	if options.KeepGoing {
		args = append(args, "--ignore-errors")
	}
	if playlistItems != "" {
		args = append(args, "--playlist-items", playlistItems)
	}
	switch options.Format {
	case "flac":
		args = append(args, "--extract-audio", "--audio-format", "flac")
	case "mp3":
		args = append(args, "--extract-audio", "--audio-format", "mp3")
		if options.AudioQuality != nil && *options.AudioQuality != "" {
			args = append(args, "--audio-quality", *options.AudioQuality)
		}
	}
	if options.EmbedMetadata {
		args = append(args, "--embed-metadata")
	}
	if options.EmbedThumbnail {
		args = append(args, "--write-thumbnail", "--embed-thumbnail")
	}
	if options.FFmpegPath != "" {
		args = append(args, "--ffmpeg-location", options.FFmpegPath)
	}
	args = append(args, url)
	return args
}

func (d *Downloader) runWithRetries(ctx context.Context, args []string,
	onEvent func(model.Event), keepGoing bool) ytdlp.RunResult {
	var last ytdlp.RunResult
	for attempt := 1; attempt <= 3; attempt++ {
		last = d.client.Run(ctx, args, onEvent)
		if keepGoing {
			return last
		}
		if last.Err == nil && len(last.Errors) == 0 {
			return last
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return last
			case <-time.After(retryDelay(attempt)):
			}
		}
	}
	return last
}

func (d *Downloader) Download(ctx context.Context, options model.Options,
	info model.PlaylistInfo, progress func(model.Event)) (model.DownloadResult, error) {
	options = resolveFFmpeg(options)
	normalized, err := normalizeInfoURL(info.URL)
	if err != nil {
		return model.DownloadResult{}, err
	}
	outputDir := filepath.Join(outputDir(options.Output), info.Title)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return model.DownloadResult{}, err
	}
	if err := CleanupPartials(outputDir); err != nil {
		return model.DownloadResult{}, err
	}
	existingFiles, err := CollectFiles(outputDir, options.Format)
	if err != nil {
		return model.DownloadResult{}, err
	}
	existing := map[string]bool{}
	for _, file := range existingFiles {
		existing[file] = true
	}

	var progressMu sync.Mutex
	safeProgress := func(event model.Event) {
		if progress == nil {
			return
		}
		progressMu.Lock()
		defer progressMu.Unlock()
		progress(event)
	}

	var failures []string
	if options.Jobs > 1 {
		selected := SelectedEntries(info, options.Start, options.End)
		if len(selected) == 0 {
			return model.DownloadResult{}, errors.New("no tracks selected")
		}
		failures = d.downloadParallel(
			ctx, options, info, selected, safeProgress,
		)
	} else {
		templateIndex := 0
		playlistItems := ""
		if !info.IsPlaylist {
			templateIndex = 1
		} else {
			playlistItems = PlaylistItems(options.Start, options.End)
		}
		base := BuildTemplate(
			outputDir, options.Template, options.TemplateString, templateIndex,
		)
		args := d.buildArgs(options, base, playlistItems, normalized)
		counter := 0
		result := d.runWithRetries(ctx, args, func(event model.Event) {
			if event.Index == 0 {
				if info.IsPlaylist {
					counter++
					event.Index = counter
				} else {
					event.Index = 1
				}
			}
			event.Total = info.Total
			safeProgress(event)
		}, options.KeepGoing)
		failures = append(failures, result.Errors...)
		if result.Err != nil {
			message := result.ErrorText()
			if message == "" {
				message = result.Err.Error()
			}
			failures = append(failures, message)
			if !options.KeepGoing {
				files, _ := CollectFiles(outputDir, options.Format)
				return model.DownloadResult{
					Files:  files,
					Failed: unique(failures),
					Total:  info.Total,
				}, errors.New(message)
			}
		}
	}

	files, err := CollectFiles(outputDir, options.Format)
	if err != nil {
		return model.DownloadResult{}, err
	}
	failures = unique(failures)
	if len(files) == 0 {
		if options.KeepGoing && len(failures) > 0 {
			return model.DownloadResult{
				Files:   files,
				Failed:  failures,
				Total:   info.Total,
				Skipped: []string{},
			}, nil
		}
		return model.DownloadResult{}, errors.New("download finished but no tracks were saved")
	}
	if len(failures) > 0 && !options.KeepGoing {
		return model.DownloadResult{
			Files:   files,
			Failed:  failures,
			Total:   info.Total,
			Skipped: []string{},
		}, fmt.Errorf("download failed: %s", failures[0])
	}

	skipped := []string{}
	for _, file := range files {
		if existing[file] {
			skipped = append(skipped, file)
		}
	}
	return model.DownloadResult{
		Files:   files,
		Failed:  failures,
		Total:   info.Total,
		Skipped: skipped,
	}, nil
}

func (d *Downloader) downloadParallel(ctx context.Context, options model.Options,
	info model.PlaylistInfo, entries []model.Entry,
	progress func(model.Event)) []string {
	workers := options.Jobs
	if workers > len(entries) {
		workers = len(entries)
	}
	type job struct {
		entry model.Entry
	}
	jobs := make(chan job)
	var failuresMu sync.Mutex
	failureList := []string{}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				template := BuildTemplate(
					filepath.Join(outputDir(options.Output), info.Title),
					options.Template, options.TemplateString, item.entry.Index,
				)
				targetURL := item.entry.URL
				if targetURL == "" {
					targetURL = info.URL
				}
				args := d.buildArgs(options, template, "", targetURL)
				result := d.runWithRetries(ctx, args, func(event model.Event) {
					if event.Index == 0 {
						event.Index = item.entry.Index
					}
					if event.Title == "" {
						event.Title = item.entry.Title
					}
					event.Total = info.Total
					progress(event)
				}, options.KeepGoing)
				if result.Err != nil {
					message := result.ErrorText()
					if message == "" {
						message = result.Err.Error()
					}
					failuresMu.Lock()
					failureList = append(
						failureList,
						fmt.Sprintf("%s: %s", item.entry.Title, message),
					)
					failuresMu.Unlock()
				}
				for _, message := range result.Errors {
					failuresMu.Lock()
					failureList = append(
						failureList,
						fmt.Sprintf("%s: %s", item.entry.Title, message),
					)
					failuresMu.Unlock()
				}
			}
		}()
	}
producer:
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			break producer
		case jobs <- job{entry: entry}:
		}
	}
	close(jobs)
	wg.Wait()
	return failureList
}

func outputDir(value string) string {
	if strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, value[2:])
		}
	}
	return value
}

func normalizeInfoURL(value string) (string, error) {
	if value == "" {
		return "", errors.New("playlist URL is required")
	}
	return urlutil.Normalize(value)
}

func resolveFFmpeg(options model.Options) model.Options {
	if options.FFmpegPath != "" {
		return options
	}
	if path, err := binmgr.Resolve(binmgr.FFmpeg, ""); err == nil {
		options.FFmpegPath = path
	}
	return options
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
