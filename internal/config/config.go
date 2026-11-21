package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const AppName = "ytm-playlist-dl"

const (
	DefaultFormat              = "flac"
	DefaultLoudness            = -14.0
	DefaultOutput              = "./downloads"
	DefaultTemplate            = "num_title"
	DefaultNormalizeMode       = "album"
	DefaultJobs                = 1
	DefaultNormalizeJobs       = 1
	DefaultConcurrentFragments = 1
	DefaultHost                = "127.0.0.1"
	DefaultPort                = 8765
	LoudnessMin                = -30.0
	LoudnessMax                = -5.0
)

var FormatChoices = []string{"flac", "mp3", "original"}

var FormatDescriptions = map[string]string{
	"flac":     "FLAC (可逆圧縮、高音質)",
	"mp3":      "MP3 (汎用性が高い)",
	"original": "元の形式のまま",
}

var NormalizeModes = []string{"album", "replaygain", "off"}

var NormalizeLabels = map[string]string{
	"album":      "アルバムモード（音声を再エンコードして音量統一）",
	"replaygain": "ReplayGainタグ（再エンコードせずタグのみ付与）",
	"off":        "正規化しない",
}

var TemplateChoices = []string{"num_title", "num_artist_title"}

var TemplateLabels = map[string]string{
	"num_title":        "{番号}_{曲名}",
	"num_artist_title": "{番号}_{アーティスト}_{曲名}",
}

var AudioExtensions = map[string]bool{
	".aac": true, ".flac": true, ".m4a": true, ".m4b": true,
	".mka": true, ".mp3": true, ".mp4": true, ".oga": true,
	".ogg": true, ".opus": true, ".wav": true, ".webm": true,
}

// Config is the persistent JSON settings file.
type Config struct {
	Format              string  `json:"format"`
	Template            string  `json:"template"`
	TemplateString      *string `json:"template_string"`
	Loudness            float64 `json:"loudness"`
	Output              string  `json:"output"`
	NormalizeMode       string  `json:"normalize_mode"`
	ForceNormalize      bool    `json:"force_normalize"`
	KeepGoing           bool    `json:"keep_going"`
	AudioQuality        *string `json:"audio_quality"`
	EmbedMetadata       bool    `json:"embed_metadata"`
	EmbedThumbnail      bool    `json:"embed_thumbnail"`
	Jobs                int     `json:"jobs"`
	NormalizeJobs       int     `json:"normalize_jobs"`
	ConcurrentFragments int     `json:"concurrent_fragments"`
	Zip                 bool    `json:"zip"`
}

func Default() Config {
	return Config{
		Format:              DefaultFormat,
		Template:            DefaultTemplate,
		Loudness:            DefaultLoudness,
		Output:              DefaultOutput,
		NormalizeMode:       DefaultNormalizeMode,
		Jobs:                DefaultJobs,
		NormalizeJobs:       DefaultNormalizeJobs,
		ConcurrentFragments: DefaultConcurrentFragments,
	}
}

func Path(explicit string) string {
	if explicit != "" {
		if expanded, err := filepath.Abs(explicit); err == nil {
			return expanded
		}
		return explicit
	}
	if override := os.Getenv("YTM_PLAYLIST_DL_CONFIG"); override != "" {
		return override
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, AppName, "config.json")
}

// Load returns defaults with values from the JSON file applied.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(Path(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	return cfg, nil
}

func Save(cfg Config, path string) (string, error) {
	target := Path(path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	return target, nil
}
