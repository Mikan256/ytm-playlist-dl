//go:build purego

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/cli"
	"github.com/Mikan256/ytm-playlist-dl/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

type desktopHandler struct {
	app       *application.App
	base      http.Handler
	updaterOK bool
	window    *application.WebviewWindow
}

func (h *desktopHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/desktop/info":
		if r.Method != http.MethodGet {
			writeDesktopError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeDesktopJSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"desktop":       true,
			"version":       cli.Version,
			"platform":      runtime.GOOS,
			"arch":          runtime.GOARCH,
			"updater_ready": h.updaterOK,
		})
		return
	case "/api/desktop/choose-directory":
		if r.Method != http.MethodPost {
			writeDesktopError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.chooseDirectory(w, r)
		return
	case "/api/desktop/open":
		if r.Method != http.MethodPost {
			writeDesktopError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.openPath(w, r)
		return
	case "/api/desktop/update":
		if r.Method != http.MethodPost {
			writeDesktopError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !h.updaterOK {
			writeDesktopError(w, http.StatusServiceUnavailable, "updater is not configured")
			return
		}
		go func() {
			if err := h.app.Updater.CheckAndInstall(context.Background()); err != nil {
				h.app.Logger.Error("updater failed", "error", err)
			}
		}()
		writeDesktopJSON(w, http.StatusAccepted, map[string]any{"ok": true})
		return
	case "/api/desktop/quit":
		if r.Method != http.MethodPost {
			writeDesktopError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeDesktopJSON(w, http.StatusOK, map[string]any{"ok": true})
		h.app.Quit()
		return
	}
	if h.base == nil {
		writeDesktopError(w, http.StatusServiceUnavailable, "web UI is not ready")
		return
	}
	h.base.ServeHTTP(w, r)
}

func (h *desktopHandler) chooseDirectory(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Current string `json:"current"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	dialog := h.app.Dialog.OpenFile().
		SetTitle("出力先フォルダを選択").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true)
	if directory := existingDirectory(payload.Current); directory != "" {
		dialog.SetDirectory(directory)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		writeDesktopError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeDesktopJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}

func (h *desktopHandler) openPath(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path   string `json:"path"`
		Reveal bool   `json:"reveal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeDesktopError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(payload.Path) == "" {
		writeDesktopError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := openPath(payload.Path, payload.Reveal); err != nil {
		writeDesktopError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeDesktopJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeDesktopJSON(w http.ResponseWriter, status int, value any) {
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

func writeDesktopError(w http.ResponseWriter, status int, message string) {
	writeDesktopJSON(w, status, map[string]any{"ok": false, "error": message})
}

func existingDirectory(path string) string {
	path = expandHome(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return ""
	}
	return absolute
}

func openPath(path string, reveal bool) error {
	path = expandHome(strings.TrimSpace(path))
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absolute); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "windows":
		if reveal {
			return exec.Command("explorer", "/select,"+absolute).Start()
		}
		return exec.Command("explorer", absolute).Start()
	case "darwin":
		if reveal {
			return exec.Command("open", "-R", absolute).Start()
		}
		return exec.Command("open", absolute).Start()
	default:
		target := absolute
		if reveal {
			info, err := os.Stat(absolute)
			if err == nil && !info.IsDir() {
				target = filepath.Dir(absolute)
			}
		}
		return exec.Command("xdg-open", target).Start()
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func defaultOutputDir() string {
	cfg, _ := config.Load("")
	return expandHome(cfg.Output)
}

func openOutputFolder() {
	dir := defaultOutputDir()
	_ = os.MkdirAll(dir, 0o755)
	if err := openPath(dir, false); err != nil {
		application.Get().Logger.Error("could not open output folder", "error", err)
	}
}

func configureUpdater(app *application.App) bool {
	provider, err := github.New(github.Config{
		Repository:    "Mikan256/ytm-playlist-dl",
		ChecksumAsset: "SHA256SUMS",
		AssetMatcher: func(req updater.CheckRequest, assets []github.ReleaseAsset) int {
			for index, asset := range assets {
				name := strings.ToLower(asset.Name)
				if !strings.HasPrefix(name, "ytm-playlist-dl-desktop-") {
					continue
				}
				if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") {
					continue
				}
				if strings.Contains(name, strings.ToLower(req.Platform)) &&
					strings.Contains(name, strings.ToLower(req.Arch)) {
					return index
				}
			}
			return -1
		},
	})
	if err != nil {
		app.Logger.Error("could not create GitHub updater", "error", err)
		return false
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: cli.Version,
		Providers:      []updater.Provider{provider},
	}); err != nil {
		app.Logger.Error("could not initialise updater", "error", err)
		return false
	}
	return true
}

func startBackgroundUpdateCheck(app *application.App) {
	go func() {
		time.Sleep(15 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		release, err := app.Updater.Check(ctx)
		if err != nil || release == nil {
			return
		}
		if err := app.Updater.CheckAndInstall(ctx); err != nil {
			app.Logger.Error("automatic update failed", "error", err)
		}
	}()
}

func configureMenu(app *application.App, updaterOK bool) {
	menu := app.NewMenu()

	fileMenu := menu.AddSubmenu("ファイル")
	fileMenu.Add("出力フォルダを開く").
		SetAccelerator("CmdOrCtrl+O").
		OnClick(func(*application.Context) {
			openOutputFolder()
		})
	fileMenu.AddSeparator()
	fileMenu.Add("終了").
		SetAccelerator("CmdOrCtrl+Q").
		OnClick(func(*application.Context) {
			app.Quit()
		})

	helpMenu := menu.AddSubmenu("ヘルプ")
	updateItem := helpMenu.Add("更新を確認...")
	updateItem.SetEnabled(updaterOK)
	updateItem.OnClick(func(*application.Context) {
		go func() {
			if err := app.Updater.CheckAndInstall(context.Background()); err != nil {
				app.Logger.Error("update check failed", "error", err)
			}
		}()
	})
	helpMenu.Add("バージョン情報").
		OnClick(func(*application.Context) {
			app.Dialog.Info().
				SetTitle("ytm-playlist-dl").
				SetMessage(fmt.Sprintf(
					"ytm-playlist-dl %s\n%s/%s\n\nYouTube Music playlist downloader",
					cli.Version, runtime.GOOS, runtime.GOARCH,
				)).
				Show()
		})

	app.Menu.Set(menu)
}

func configureTray(app *application.App, window *application.WebviewWindow,
	icon []byte, updaterOK bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			app.Logger.Error("system tray is unavailable", "error", recovered)
		}
	}()
	tray := app.SystemTray.New()
	tray.SetIcon(icon)
	tray.SetLabel("ytm-playlist-dl")
	tray.SetTooltip("ytm-playlist-dl")

	trayMenu := app.NewMenu()
	trayMenu.Add("ウィンドウを表示").OnClick(func(*application.Context) {
		window.Show()
		window.Focus()
	})
	trayMenu.Add("出力フォルダを開く").OnClick(func(*application.Context) {
		openOutputFolder()
	})
	updateItem := trayMenu.Add("更新を確認...")
	updateItem.SetEnabled(updaterOK)
	updateItem.OnClick(func(*application.Context) {
		go func() {
			if err := app.Updater.CheckAndInstall(context.Background()); err != nil {
				app.Logger.Error("update check failed", "error", err)
			}
		}()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("終了").OnClick(func(*application.Context) {
		app.Quit()
	})
	tray.SetMenu(trayMenu)
	tray.OnClick(func() {
		if window.IsVisible() {
			window.Hide()
			return
		}
		window.Show()
		window.Focus()
	})
}
