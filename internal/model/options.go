package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Mikan256/ytm-playlist-dl/internal/config"
)

// Options controls one playlist run.
type Options struct {
	URL                 string  `json:"url"`
	Format              string  `json:"fmt"`
	Template            string  `json:"template"`
	TemplateString      *string `json:"template_string"`
	Loudness            float64 `json:"loudness"`
	Output              string  `json:"output"`
	NormalizeMode       string  `json:"normalize_mode"`
	ForceNormalize      bool    `json:"force_normalize"`
	Start               *int    `json:"start"`
	End                 *int    `json:"end"`
	KeepGoing           bool    `json:"keep_going"`
	AudioQuality        *string `json:"audio_quality"`
	EmbedMetadata       bool    `json:"embed_metadata"`
	EmbedThumbnail      bool    `json:"embed_thumbnail"`
	Jobs                int     `json:"jobs"`
	NormalizeJobs       int     `json:"normalize_jobs"`
	ConcurrentFragments int     `json:"concurrent_fragments"`
	Zip                 bool    `json:"zip"`
	ExportFormat        string  `json:"export_format"`
	DryRun              bool    `json:"dry_run"`
	ListTracks          bool    `json:"list_tracks"`
	LogFile             string  `json:"log_file,omitempty"`
	ReportFile          string  `json:"report_file,omitempty"`
	YtDlpPath           string  `json:"yt_dlp_path,omitempty"`
	FFmpegPath          string  `json:"ffmpeg_path,omitempty"`
}

func DefaultOptions(url string) Options {
	cfg := config.Default()
	return OptionsFromConfig(cfg, url)
}

func OptionsFromConfig(cfg config.Config, url string) Options {
	return Options{
		URL:                 url,
		Format:              cfg.Format,
		Template:            cfg.Template,
		TemplateString:      cfg.TemplateString,
		Loudness:            cfg.Loudness,
		Output:              cfg.Output,
		NormalizeMode:       cfg.NormalizeMode,
		ForceNormalize:      cfg.ForceNormalize,
		KeepGoing:           cfg.KeepGoing,
		AudioQuality:        cfg.AudioQuality,
		EmbedMetadata:       cfg.EmbedMetadata,
		EmbedThumbnail:      cfg.EmbedThumbnail,
		Jobs:                cfg.Jobs,
		NormalizeJobs:       cfg.NormalizeJobs,
		ConcurrentFragments: cfg.ConcurrentFragments,
		Zip:                 cfg.Zip,
		ExportFormat:        cfg.ExportFormat,
	}
}

func (o Options) Config() config.Config {
	return config.Config{
		Format:              o.Format,
		Template:            o.Template,
		TemplateString:      o.TemplateString,
		Loudness:            o.Loudness,
		Output:              o.Output,
		NormalizeMode:       o.NormalizeMode,
		ForceNormalize:      o.ForceNormalize,
		KeepGoing:           o.KeepGoing,
		AudioQuality:        o.AudioQuality,
		EmbedMetadata:       o.EmbedMetadata,
		EmbedThumbnail:      o.EmbedThumbnail,
		Jobs:                o.Jobs,
		NormalizeJobs:       o.NormalizeJobs,
		ConcurrentFragments: o.ConcurrentFragments,
		Zip:                 o.Zip,
		ExportFormat:        o.ExportFormat,
	}
}

func (o Options) Validate() error {
	if strings.TrimSpace(o.URL) == "" {
		return errors.New("playlist URL is required")
	}
	if !contains(config.FormatChoices, o.Format) {
		return fmt.Errorf("unknown output format: %s", o.Format)
	}
	if !contains(config.TemplateChoices, o.Template) {
		return fmt.Errorf("unknown template: %s", o.Template)
	}
	if !contains(config.NormalizeModes, o.NormalizeMode) {
		return fmt.Errorf("unknown normalize mode: %s", o.NormalizeMode)
	}
	if !contains(config.ExportFormats, o.ExportFormat) {
		return fmt.Errorf("unknown export format: %s", o.ExportFormat)
	}
	if o.Loudness < config.LoudnessMin || o.Loudness > config.LoudnessMax {
		return fmt.Errorf(
			"loudness must be between %g and %g LUFS",
			config.LoudnessMin, config.LoudnessMax,
		)
	}
	if o.Start != nil && *o.Start < 1 {
		return errors.New("start must be 1 or greater")
	}
	if o.End != nil && *o.End < 1 {
		return errors.New("end must be 1 or greater")
	}
	if o.Start != nil && o.End != nil && *o.End < *o.Start {
		return errors.New("end must be greater than or equal to start")
	}
	if o.Jobs < 1 {
		return errors.New("jobs must be 1 or greater")
	}
	if o.NormalizeJobs < 1 {
		return errors.New("normalize-jobs must be 1 or greater")
	}
	if o.ConcurrentFragments < 1 {
		return errors.New("concurrent-fragments must be 1 or greater")
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// FromMap decodes a JSON/API/config mapping. The format key accepts both
// "fmt" and "format" for compatibility with the original Python API.
func FromMap(data map[string]any, url string) (Options, error) {
	cfg := config.Default()
	options := OptionsFromConfig(cfg, url)

	if value, ok := data["format"]; ok {
		if _, hasFmt := data["fmt"]; !hasFmt {
			data["fmt"] = value
		}
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return options, err
	}
	if err := json.Unmarshal(encoded, &options); err != nil {
		return options, err
	}
	if options.URL == "" {
		options.URL = url
	}
	if options.Format == "" {
		options.Format = config.DefaultFormat
	}
	if options.Template == "" {
		options.Template = config.DefaultTemplate
	}
	if options.Output == "" {
		options.Output = config.DefaultOutput
	}
	if options.NormalizeMode == "" {
		options.NormalizeMode = config.DefaultNormalizeMode
	}
	if options.Jobs == 0 {
		options.Jobs = config.DefaultJobs
	}
	if options.NormalizeJobs == 0 {
		options.NormalizeJobs = config.DefaultNormalizeJobs
	}
	if options.ConcurrentFragments == 0 {
		options.ConcurrentFragments = config.DefaultConcurrentFragments
	}
	if options.ExportFormat == "" {
		options.ExportFormat = "none"
	}
	options.Format = strings.ToLower(options.Format)
	options.NormalizeMode = strings.ToLower(options.NormalizeMode)
	options.ExportFormat = strings.ToLower(options.ExportFormat)
	options.URL = strings.TrimSpace(options.URL)
	options.TemplateString = trimOptional(options.TemplateString)
	options.AudioQuality = trimOptional(options.AudioQuality)
	options.LogFile = strings.TrimSpace(options.LogFile)
	options.ReportFile = strings.TrimSpace(options.ReportFile)
	return options, nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// ToMap returns the same shape as the original Python RunOptions.to_dict().
func (o Options) ToMap() map[string]any {
	data, _ := json.Marshal(o)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result
}

// ParseOptionalInt is shared by the CLI and API payload handling.
func ParseOptionalInt(value any) (*int, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case float64:
		number := int(typed)
		return &number, nil
	case int:
		number := typed
		return &number, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return nil, err
		}
		return &number, nil
	default:
		return nil, fmt.Errorf("invalid integer value: %v", value)
	}
}
