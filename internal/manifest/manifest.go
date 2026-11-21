package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const Name = ".ytm-playlist-dl.manifest.json"

type Record struct {
	Size         int64    `json:"size"`
	MtimeNS      int64    `json:"mtime_ns"`
	Mode         string   `json:"mode"`
	Target       float64  `json:"target"`
	GainDB       *float64 `json:"gain_db"`
	NormalizedAt string   `json:"normalized_at"`
}

type data struct {
	Version    int               `json:"version"`
	Normalized map[string]Record `json:"normalized"`
}

type Manifest struct {
	path string
	mu   sync.Mutex
	data data
}

func PathForDir(dir string) string {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		absolute = dir
	}
	return filepath.Join(absolute, Name)
}

func PathForFiles(files []string) string {
	if len(files) == 0 {
		return ""
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		absolute, err := filepath.Abs(file)
		if err != nil {
			absolute = file
		}
		paths = append(paths, absolute)
	}
	base := paths[0]
	if len(paths) > 1 {
		base = commonDir(paths)
	} else {
		base = filepath.Dir(base)
	}
	return PathForDir(base)
}

func commonDir(paths []string) string {
	base := filepath.Dir(paths[0])
	for _, path := range paths[1:] {
		dir := filepath.Dir(path)
		for {
			rel, err := filepath.Rel(base, dir)
			if err == nil && rel != ".." &&
				!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				break
			}
			parent := filepath.Dir(base)
			if parent == base {
				return base
			}
			base = parent
		}
	}
	return base
}

func New(path string) *Manifest {
	m := &Manifest{
		path: path,
		data: data{Version: 1, Normalized: map[string]Record{}},
	}
	m.load()
	return m
}

func (m *Manifest) load() {
	if m.path == "" {
		return
	}
	raw, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	var parsed data
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return
	}
	if parsed.Normalized == nil {
		parsed.Normalized = map[string]Record{}
	}
	if parsed.Version == 0 {
		parsed.Version = 1
	}
	m.data = parsed
}

func (m *Manifest) key(file string) string {
	base := filepath.Dir(m.path)
	absolute, err := filepath.Abs(file)
	if err != nil {
		absolute = file
	}
	rel, err := filepath.Rel(base, absolute)
	if err != nil {
		return filepath.Base(file)
	}
	return rel
}

func fingerprint(file string) (Record, bool) {
	info, err := os.Stat(file)
	if err != nil {
		return Record{}, false
	}
	return Record{Size: info.Size(), MtimeNS: info.ModTime().UnixNano()}, true
}

func (m *Manifest) isCurrentLocked(file, mode string, target float64) bool {
	if m.path == "" {
		return false
	}
	record, ok := m.data.Normalized[m.key(file)]
	if !ok {
		return false
	}
	current, ok := fingerprint(file)
	if !ok {
		return false
	}
	delta := record.Target - target
	if delta < 0 {
		delta = -delta
	}
	return record.Size == current.Size &&
		record.MtimeNS == current.MtimeNS &&
		record.Mode == mode &&
		delta < 1e-6
}

func (m *Manifest) IsCurrent(file, mode string, target float64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isCurrentLocked(file, mode, target)
}

func (m *Manifest) GainFor(file, mode string, target float64) *float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isCurrentLocked(file, mode, target) {
		return nil
	}
	record := m.data.Normalized[m.key(file)]
	if record.GainDB == nil {
		return nil
	}
	gain := *record.GainDB
	return &gain
}

func (m *Manifest) Mark(file, mode string, target float64, gain *float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.path == "" {
		return
	}
	current, ok := fingerprint(file)
	if !ok {
		return
	}
	record := Record{
		Size:         current.Size,
		MtimeNS:      current.MtimeNS,
		Mode:         mode,
		Target:       target,
		GainDB:       gain,
		NormalizedAt: time.Now().UTC().Format(time.RFC3339),
	}
	m.data.Normalized[m.key(file)] = record
}

func (m *Manifest) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}
