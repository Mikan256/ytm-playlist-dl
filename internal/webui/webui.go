package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/config"
	"github.com/Mikan256/ytm-playlist-dl/internal/downloader"
	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/service"
	"github.com/Mikan256/ytm-playlist-dl/web"
)

const maxBodySize = 1_000_000

type Runner func(ctx context.Context, options model.Options,
	info *model.PlaylistInfo, progress func(model.Event)) (model.RunResult, error)

type Job struct {
	ID         string
	Options    model.Options
	runner     Runner
	Status     string
	Events     []model.Event
	Result     *model.RunResult
	Error      string
	CreatedAt  float64
	StartedAt  float64
	FinishedAt float64
	mu         sync.Mutex
	cancel     context.CancelFunc
}

func newJob(options model.Options, runner Runner) *Job {
	return &Job{
		ID:        fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix()),
		Options:   options,
		runner:    runner,
		Status:    "queued",
		CreatedAt: now(),
	}
}

func now() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

func (j *Job) addEvent(event model.Event) {
	if event.At == 0 {
		event.At = now()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Events = append(j.Events, event)
	if len(j.Events) > 2000 {
		j.Events = append([]model.Event{}, j.Events[len(j.Events)-1000:]...)
	}
}

func (j *Job) snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	events := append([]model.Event{}, j.Events...)
	var result any
	if j.Result != nil {
		result = j.Result
	}
	return map[string]any{
		"id":          j.ID,
		"status":      j.Status,
		"created_at":  j.CreatedAt,
		"started_at":  j.StartedAt,
		"finished_at": j.FinishedAt,
		"events":      events,
		"error":       emptyNil(j.Error),
		"options":     j.Options.ToMap(),
		"result":      result,
	}
}

func (j *Job) summary() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	var playlist any
	var exitCode any
	if j.Result != nil {
		playlist = j.Result.Info.Title
		exitCode = j.Result.ExitCode
	}
	return map[string]any{
		"id":          j.ID,
		"status":      j.Status,
		"created_at":  j.CreatedAt,
		"finished_at": j.FinishedAt,
		"error":       emptyNil(j.Error),
		"playlist":    playlist,
		"exit_code":   exitCode,
	}
}

func emptyNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type JobManager struct {
	runner Runner
	mu     sync.RWMutex
	jobs   map[string]*Job
}

func NewJobManager(runner Runner) *JobManager {
	if runner == nil {
		runner = func(ctx context.Context, options model.Options,
			info *model.PlaylistInfo, progress func(model.Event)) (model.RunResult, error) {
			return (service.Service{}).Execute(ctx, options, info, progress)
		}
	}
	return &JobManager{runner: runner, jobs: map[string]*Job{}}
}

func (m *JobManager) Start(options model.Options) *Job {
	job := newJob(options, m.runner)
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
	go m.run(job)
	return job
}

func (m *JobManager) run(job *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	job.mu.Lock()
	job.cancel = cancel
	job.Status = "running"
	job.StartedAt = now()
	job.mu.Unlock()
	defer cancel()
	job.addEvent(model.Event{Type: "status", Message: "ジョブを開始しました"})

	result, err := job.runner(ctx, job.Options, nil, job.addEvent)
	job.mu.Lock()
	defer job.mu.Unlock()
	job.FinishedAt = now()
	if err != nil {
		job.Error = err.Error()
		job.Status = "error"
		job.addEventLocked(model.Event{Type: "error", Message: err.Error()})
		return
	}
	job.Result = &result
	if result.ExitCode == 2 {
		job.Status = "partial"
	} else {
		job.Status = "done"
	}
}

func (j *Job) addEventLocked(event model.Event) {
	if event.At == 0 {
		event.At = now()
	}
	j.Events = append(j.Events, event)
}

func (m *JobManager) Get(id string) *Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[id]
}

func (m *JobManager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt > jobs[j].CreatedAt
	})
	return jobs
}

type Handler struct {
	Manager *JobManager
	Static  fs.FS
}

func NewHandler(manager *JobManager) http.Handler {
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(err)
	}
	handler := &Handler{Manager: manager, Static: staticFS}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", handler.config)
	mux.HandleFunc("/api/info", handler.info)
	mux.HandleFunc("/api/jobs", handler.jobs)
	mux.HandleFunc("/api/jobs/", handler.job)
	mux.HandleFunc("/health", handler.health)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", handler.index)
	return mux
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.notFound(w)
		return
	}
	data, err := fs.ReadFile(h.Static, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := config.Load("")
		if err != nil {
			cfg = config.Default()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":               true,
			"config":           cfg,
			"config_path":      config.Path(""),
			"formats":          config.FormatChoices,
			"format_labels":    config.FormatDescriptions,
			"templates":        config.TemplateChoices,
			"template_labels":  config.TemplateLabels,
			"normalize_modes":  config.NormalizeModes,
			"normalize_labels": config.NormalizeLabels,
			"defaults": map[string]any{
				"host": config.DefaultHost,
				"port": config.DefaultPort,
			},
		})
	case http.MethodPost:
		payload, err := readJSON(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		options, err := model.FromMap(payload, "")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		path, err := config.Save(options.Config(), "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	payload, err := readJSON(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rawURL, _ := payload["url"].(string)
	if strings.TrimSpace(rawURL) == "" {
		writeError(w, http.StatusBadRequest, "URL is required")
		return
	}
	client, err := downloader.New("")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := client.Info(r.Context(), rawURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "info": info})
}

func (h *Handler) jobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jobs := h.Manager.List()
		items := make([]map[string]any, 0, len(jobs))
		for _, job := range jobs {
			items = append(items, job.summary())
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "jobs": items})
	case http.MethodPost:
		payload, err := readJSON(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		options, err := model.FromMap(payload, "")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := options.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		job := h.Manager.Start(options)
		writeJSON(w, http.StatusAccepted, map[string]any{
			"ok": true, "job": job.summary(),
		})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) job(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		h.notFound(w)
		return
	}
	job := h.Manager.Get(parts[2])
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if len(parts) >= 4 && parts[3] == "report" {
		job.mu.Lock()
		result := job.Result
		job.mu.Unlock()
		if result == nil {
			writeError(w, http.StatusNotFound, "report not ready")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "report": result.Report,
		})
		return
	}
	if len(parts) >= 4 && parts[3] == "archive" {
		job.mu.Lock()
		result := job.Result
		job.mu.Unlock()
		if result == nil || result.ArchivePath == "" {
			writeError(w, http.StatusNotFound, "archive not ready")
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set(
			"Content-Disposition",
			fmt.Sprintf("attachment; filename=%q", filepath.Base(result.ArchivePath)),
		)
		http.ServeFile(w, r, result.ArchivePath)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": job.snapshot()})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "healthy"})
}

func (h *Handler) notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not found")
}

func readJSON(r *http.Request) (map[string]any, error) {
	limited := http.MaxBytesReader(nil, r.Body, maxBodySize)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return payload, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": message})
}

func RunServer(host string, port int, openBrowser bool) error {
	if host == "" {
		host = config.DefaultHost
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	manager := NewJobManager(nil)
	server := &http.Server{Handler: NewHandler(manager)}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://%s:%d/", host, actualPort)
	fmt.Printf("ytm-playlist-dl web UI: %s\n", url)
	fmt.Println("Ctrl+C to stop.")
	if openBrowser {
		_ = openInBrowser(url)
	}
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func openInBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start()
}
