package normalizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Mikan256/ytm-playlist-dl/internal/manifest"
	"github.com/Mikan256/ytm-playlist-dl/internal/model"
)

var codecByExtension = map[string]string{
	".aac": "aac", ".flac": "flac", ".m4a": "aac", ".m4b": "aac",
	".mka": "libopus", ".mp3": "libmp3lame", ".mp4": "aac",
	".oga": "libvorbis", ".ogg": "libvorbis", ".opus": "libopus",
	".wav": "pcm_s16le", ".webm": "libopus",
}

type Processor struct {
	FFmpeg  string
	FFprobe string
}

func New(ffmpegPath string) (*Processor, error) {
	ffmpeg := ffmpegPath
	if ffmpeg == "" {
		found, err := exec.LookPath("ffmpeg")
		if err != nil {
			return nil, errors.New(
				"ffmpeg/ffprobe not found. Install ffmpeg and make sure it is on PATH",
			)
		}
		ffmpeg = found
	}
	probe := ""
	if ffmpegPath != "" {
		candidate := filepath.Join(filepath.Dir(ffmpegPath), "ffprobe")
		if _, err := os.Stat(candidate); err == nil {
			probe = candidate
		}
	}
	if probe == "" {
		found, err := exec.LookPath("ffprobe")
		if err != nil {
			return nil, errors.New(
				"ffmpeg/ffprobe not found. Install ffmpeg and make sure it is on PATH",
			)
		}
		probe = found
	}
	return &Processor{FFmpeg: ffmpeg, FFprobe: probe}, nil
}

func (p *Processor) command(ctx context.Context, binary string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, binary, args...)
}

func (p *Processor) duration(ctx context.Context, file string) float64 {
	command := p.command(
		ctx, p.FFprobe, "-v", "error", "-show_entries", "format=duration",
		"-of", "csv=p=0", file,
	)
	output, err := command.Output()
	if err != nil {
		return 0
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0
	}
	return value
}

type loudnormJSON struct {
	InputI      string `json:"input_i"`
	InputTP     string `json:"input_tp"`
	InputLRA    string `json:"input_lra"`
	InputThresh string `json:"input_thresh"`
}

func parseLoudnormJSON(stderr string) (loudnormJSON, bool) {
	start := strings.LastIndex(stderr, "{")
	end := strings.LastIndex(stderr, "}")
	if start < 0 || end <= start {
		return loudnormJSON{}, false
	}
	var parsed loudnormJSON
	if err := json.Unmarshal([]byte(stderr[start:end+1]), &parsed); err != nil {
		return loudnormJSON{}, false
	}
	return parsed, true
}

func finite(value string) (float64, bool) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func (p *Processor) measure(ctx context.Context, file string, threads int) (*model.Measurement, error) {
	args := []string{
		"-hide_banner", "-i", file,
		"-af", "loudnorm=I=-14:LRA=11:TP=-1:print_format=json",
	}
	if threads > 0 {
		args = append(args, "-threads", strconv.Itoa(threads))
	}
	args = append(args, "-f", "null", "-")
	command := p.command(ctx, p.FFmpeg, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, err
	}
	parsed, ok := parseLoudnormJSON(stderr.String())
	if !ok {
		return nil, errors.New("could not parse loudnorm output")
	}
	inputI, ok := finite(parsed.InputI)
	if !ok {
		return nil, errors.New("invalid integrated loudness")
	}
	inputTP, _ := finite(parsed.InputTP)
	inputLRA, _ := finite(parsed.InputLRA)
	inputThresh, _ := finite(parsed.InputThresh)
	return &model.Measurement{
		InputI:      inputI,
		InputTP:     inputTP,
		InputLRA:    inputLRA,
		InputThresh: inputThresh,
		Duration:    p.duration(ctx, file),
	}, nil
}

func ComputeAlbumLoudness(measurements []model.Measurement) *float64 {
	totalDuration := 0.0
	weightedPower := 0.0
	for _, measurement := range measurements {
		if measurement.Duration <= 0 ||
			math.IsNaN(measurement.InputI) ||
			math.IsInf(measurement.InputI, 0) {
			continue
		}
		totalDuration += measurement.Duration
		weightedPower += measurement.Duration * math.Pow(10, measurement.InputI/10)
	}
	if totalDuration <= 0 || weightedPower <= 0 {
		return nil
	}
	value := 10 * math.Log10(weightedPower/totalDuration)
	return &value
}

func codecForPath(path string) string {
	return codecByExtension[strings.ToLower(filepath.Ext(path))]
}

func (p *Processor) applyGain(ctx context.Context, input, output string,
	gain float64, threads int) bool {
	codec := codecForPath(output)
	if codec == "" {
		return false
	}
	filter := fmt.Sprintf("volume=%.6fdB", gain)
	if gain > 0 {
		filter += ",alimiter=limit=0.891251:level=false"
	}
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", input, "-vn", "-af", filter, "-c:a", codec,
	}
	if threads > 0 {
		args = append(args, "-threads", strconv.Itoa(threads))
	}
	args = append(args, output)
	return p.command(ctx, p.FFmpeg, args...).Run() == nil
}

func (p *Processor) writeReplayGainTags(ctx context.Context, input, output string,
	tags map[string]string) bool {
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", input, "-map_metadata", "0", "-c", "copy",
	}
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "-metadata", key+"="+tags[key])
	}
	args = append(args, output)
	return p.command(ctx, p.FFmpeg, args...).Run() == nil
}

func replaceWithOutput(ctx context.Context, file string,
	writer func(string) bool) bool {
	dir := filepath.Dir(file)
	extension := filepath.Ext(file)
	tmp, err := os.CreateTemp(dir, "ytm-normalise-*"+extension)
	if err != nil {
		return false
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)
	if !writer(tmpPath) {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	default:
	}
	return os.Rename(tmpPath, file) == nil
}

func label(file string) string {
	name := filepath.Base(file)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func emit(progress func(model.Event), current, total int, file, phase string) {
	if progress == nil {
		return
	}
	progress(model.Event{
		Type: "normalize", Current: current, Total: total,
		Label: label(file), Phase: phase,
	})
}

func (p *Processor) measureMany(ctx context.Context, files []string, jobs int,
	progress func(model.Event)) map[string]model.Measurement {
	jobs = clampJobs(jobs, len(files))
	results := make([]*model.Measurement, len(files))
	var next atomic.Int64
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	threads := 0
	if jobs > 1 {
		threads = 1
	}
	for worker := 0; worker < jobs; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= len(files) {
					return
				}
				file := files[index]
				progressMu.Lock()
				emit(progress, index+1, len(files), file, "analysing")
				progressMu.Unlock()
				measurement, err := p.measure(ctx, file, threads)
				if err == nil {
					results[index] = measurement
				}
			}
		}()
	}
	wg.Wait()
	collected := map[string]model.Measurement{}
	for index, measurement := range results {
		if measurement != nil {
			collected[files[index]] = *measurement
		}
	}
	return collected
}

func measurementValues(measurements map[string]model.Measurement) []model.Measurement {
	values := make([]model.Measurement, 0, len(measurements))
	for _, measurement := range measurements {
		values = append(values, measurement)
	}
	return values
}

func (p *Processor) applyMany(ctx context.Context, files []string, jobs int,
	gain float64, mode string, target float64, record *manifest.Manifest,
	progress func(model.Event)) ([]string, []string) {
	jobs = clampJobs(jobs, len(files))
	normalized := make([]string, len(files))
	failed := make([]string, len(files))
	var next atomic.Int64
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	threads := 0
	if jobs > 1 {
		threads = 1
	}
	for worker := 0; worker < jobs; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= len(files) {
					return
				}
				file := files[index]
				progressMu.Lock()
				emit(progress, index+1, len(files), file, "normalising")
				progressMu.Unlock()
				ok := replaceWithOutput(ctx, file, func(output string) bool {
					return p.applyGain(ctx, file, output, gain, threads)
				})
				if ok {
					normalized[index] = file
					record.Mark(file, mode, target, &gain)
				} else {
					failed[index] = file
				}
			}
		}()
	}
	wg.Wait()
	return compact(normalized), compact(failed)
}

func (p *Processor) applyReplayGain(ctx context.Context, files []string, jobs int,
	measurements map[string]model.Measurement, albumGain, albumPeak float64,
	target float64, record *manifest.Manifest,
	progress func(model.Event)) ([]string, []string) {
	jobs = clampJobs(jobs, len(files))
	normalized := make([]string, len(files))
	failed := make([]string, len(files))
	var next atomic.Int64
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	for worker := 0; worker < jobs; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= len(files) {
					return
				}
				file := files[index]
				progressMu.Lock()
				emit(progress, index+1, len(files), file, "normalising")
				progressMu.Unlock()
				measurement, ok := measurements[file]
				if !ok {
					failed[index] = file
					continue
				}
				trackGain := target - measurement.InputI
				trackPeak := math.Pow(10, measurement.InputTP/20)
				tags := map[string]string{
					"REPLAYGAIN_TRACK_GAIN": fmt.Sprintf("%+.2f dB", trackGain),
					"REPLAYGAIN_TRACK_PEAK": fmt.Sprintf("%.6f", trackPeak),
					"REPLAYGAIN_ALBUM_GAIN": fmt.Sprintf("%+.2f dB", albumGain),
					"REPLAYGAIN_ALBUM_PEAK": fmt.Sprintf("%.6f", albumPeak),
				}
				success := replaceWithOutput(ctx, file, func(output string) bool {
					return p.writeReplayGainTags(ctx, file, output, tags)
				})
				if success {
					normalized[index] = file
					record.Mark(file, "replaygain", target, &trackGain)
				} else {
					failed[index] = file
				}
			}
		}()
	}
	wg.Wait()
	return compact(normalized), compact(failed)
}

func compact(values []string) []string {
	out := []string{}
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func clampJobs(jobs, total int) int {
	if jobs < 1 {
		jobs = 1
	}
	if total < 1 {
		return 1
	}
	if jobs > total {
		return total
	}
	return jobs
}

func splitByManifest(files []string, record *manifest.Manifest, force bool,
	mode string, target float64) ([]string, []string) {
	if force {
		return append([]string{}, files...), []string{}
	}
	pending := []string{}
	skipped := []string{}
	for _, file := range files {
		if record.IsCurrent(file, mode, target) {
			skipped = append(skipped, file)
		} else {
			pending = append(pending, file)
		}
	}
	return pending, skipped
}

func (p *Processor) Normalize(ctx context.Context, files []string,
	target float64, mode string, force bool, manifestPath string, jobs int,
	progress func(model.Event)) (model.NormalizationResult, error) {
	files = append([]string{}, files...)
	if mode == "off" {
		return model.NormalizationResult{Files: files, Mode: "off"}, nil
	}
	if mode == "replaygain" {
		return p.normalizeReplayGain(
			ctx, files, target, force, manifestPath, jobs, progress,
		)
	}
	if mode != "album" {
		return model.NormalizationResult{}, fmt.Errorf("unknown normalisation mode: %s", mode)
	}
	if len(files) == 0 {
		return model.NormalizationResult{Files: files, Mode: "album"}, nil
	}
	if manifestPath == "" {
		manifestPath = manifest.PathForFiles(files)
	}
	record := manifest.New(manifestPath)
	pending, skipped := splitByManifest(files, record, force, "album", target)
	if len(pending) == 0 {
		return model.NormalizationResult{
			Files: files, Skipped: skipped, Mode: "album",
		}, nil
	}

	var previousGain *float64
	for _, file := range skipped {
		previousGain = record.GainFor(file, "album", target)
		if previousGain != nil {
			break
		}
	}
	measureFiles := files
	if previousGain != nil {
		measureFiles = pending
	}
	measurements := p.measureMany(ctx, measureFiles, jobs, progress)
	albumLoudness := ComputeAlbumLoudness(measurementValues(measurements))
	var gain float64
	if previousGain != nil {
		gain = *previousGain
	} else if albumLoudness == nil {
		return model.NormalizationResult{
			Files: files, Skipped: skipped, Mode: "album",
		}, nil
	} else {
		gain = target - *albumLoudness
	}
	normalized, failed := p.applyMany(
		ctx, pending, jobs, gain, "album", target, record, progress,
	)
	if err := record.Save(); err != nil {
		return model.NormalizationResult{}, err
	}
	return model.NormalizationResult{
		Files: files, Normalized: normalized, Skipped: skipped,
		Failed: failed, AlbumLoudness: albumLoudness, GainDB: &gain,
		Mode: "album",
	}, nil
}

func (p *Processor) normalizeReplayGain(ctx context.Context, files []string,
	target float64, force bool, manifestPath string, jobs int,
	progress func(model.Event)) (model.NormalizationResult, error) {
	if len(files) == 0 {
		return model.NormalizationResult{Files: files, Mode: "replaygain"}, nil
	}
	if manifestPath == "" {
		manifestPath = manifest.PathForFiles(files)
	}
	record := manifest.New(manifestPath)
	pending, skipped := splitByManifest(files, record, force, "replaygain", target)
	if len(pending) == 0 {
		return model.NormalizationResult{
			Files: files, Skipped: skipped, Mode: "replaygain",
		}, nil
	}
	byFile := p.measureMany(ctx, pending, jobs, progress)
	measurements := measurementValues(byFile)
	albumLoudness := ComputeAlbumLoudness(measurements)
	albumGain := 0.0
	if albumLoudness != nil {
		albumGain = target - *albumLoudness
	}
	albumPeak := 0.0
	for _, measurement := range measurements {
		peak := math.Pow(10, measurement.InputTP/20)
		if peak > albumPeak {
			albumPeak = peak
		}
	}
	if albumPeak == 0 {
		albumPeak = 1
	}
	normalized, failed := p.applyReplayGain(
		ctx, pending, jobs, byFile, albumGain, albumPeak, target, record, progress,
	)
	if err := record.Save(); err != nil {
		return model.NormalizationResult{}, err
	}
	return model.NormalizationResult{
		Files: files, Normalized: normalized, Skipped: skipped,
		Failed: failed, AlbumLoudness: albumLoudness, GainDB: &albumGain,
		Mode: "replaygain",
	}, nil
}
