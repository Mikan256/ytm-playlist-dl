package model

// Entry is one track in a playlist.
type Entry struct {
	Index    int     `json:"index"`
	ID       string  `json:"id,omitempty"`
	Title    string  `json:"title"`
	URL      string  `json:"url,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

// PlaylistInfo is the result of extracting a playlist.
type PlaylistInfo struct {
	Title      string  `json:"title"`
	Total      int     `json:"total"`
	URL        string  `json:"url"`
	IsPlaylist bool    `json:"is_playlist"`
	Entries    []Entry `json:"entries"`
}

// DownloadResult describes files produced by a download run.
type DownloadResult struct {
	Files   []string `json:"files"`
	Failed  []string `json:"failed"`
	Total   int      `json:"total"`
	Skipped []string `json:"skipped"`
}

// WithSlices makes JSON encode empty collections as [] rather than null.
func (r DownloadResult) WithSlices() DownloadResult {
	if r.Files == nil {
		r.Files = []string{}
	}
	if r.Failed == nil {
		r.Failed = []string{}
	}
	if r.Skipped == nil {
		r.Skipped = []string{}
	}
	return r
}

// NormalizationResult describes loudness processing.
type NormalizationResult struct {
	Files         []string `json:"files"`
	Normalized    []string `json:"normalized"`
	Skipped       []string `json:"skipped"`
	Failed        []string `json:"failed"`
	AlbumLoudness *float64 `json:"album_loudness"`
	GainDB        *float64 `json:"gain_db"`
	Mode          string   `json:"mode"`
}

// WithSlices makes JSON encode empty collections as [] rather than null.
func (r NormalizationResult) WithSlices() NormalizationResult {
	if r.Files == nil {
		r.Files = []string{}
	}
	if r.Normalized == nil {
		r.Normalized = []string{}
	}
	if r.Skipped == nil {
		r.Skipped = []string{}
	}
	if r.Failed == nil {
		r.Failed = []string{}
	}
	return r
}

// Measurement is one track's loudness analysis.
type Measurement struct {
	InputI      float64 `json:"input_i"`
	InputLRA    float64 `json:"input_lra"`
	InputTP     float64 `json:"input_tp"`
	InputThresh float64 `json:"input_thresh"`
	Duration    float64 `json:"duration"`
}

// Event is used for CLI progress callbacks and web job events.
type Event struct {
	Type     string  `json:"type"`
	At       float64 `json:"at,omitempty"`
	Message  string  `json:"message,omitempty"`
	Index    int     `json:"index,omitempty"`
	Total    int     `json:"total,omitempty"`
	Title    string  `json:"title,omitempty"`
	Current  int     `json:"current,omitempty"`
	Label    string  `json:"label,omitempty"`
	Phase    string  `json:"phase,omitempty"`
	Percent  float64 `json:"percent,omitempty"`
	ExitCode *int    `json:"exit_code,omitempty"`
}

// RunResult is returned by the shared service.
type RunResult struct {
	Options       Options              `json:"options"`
	Info          PlaylistInfo         `json:"info"`
	OutputDir     string               `json:"output_dir"`
	Files         []string             `json:"files"`
	Download      *DownloadResult      `json:"download"`
	Normalization *NormalizationResult `json:"normalization"`
	Report        map[string]any       `json:"report"`
	ReportPath    string               `json:"report_path,omitempty"`
	ArchivePath   string               `json:"archive_path,omitempty"`
	Planned       []string             `json:"planned,omitempty"`
	ExitCode      int                  `json:"exit_code"`
}
