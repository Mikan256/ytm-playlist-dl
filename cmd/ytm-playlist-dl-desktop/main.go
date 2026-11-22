//go:build purego

package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/webui"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed assets/icon.png
var icon []byte

func main() {
	if err := checkRuntime(); err != nil {
		log.Fatal(err)
	}

	desktop := &desktopHandler{base: webui.NewHandler(webui.NewJobManager(nil))}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: desktop}
	go func() {
		if err := server.Serve(listener); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Printf("local web server: %v", err)
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	app := application.New(application.Options{
		Name:        "ytm-playlist-dl",
		Description: "YouTube Music playlist and video downloader",
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})
	desktop.app = app

	state := loadWindowState()
	width, height := 1240, 900
	if state.Width >= 900 && state.Height >= 620 {
		width, height = state.Width, state.Height
	}
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:              "ytm-playlist-dl",
		Width:              width,
		Height:             height,
		MinWidth:           900,
		MinHeight:          620,
		BackgroundColour:   application.NewRGB(242, 230, 210),
		URL:                fmt.Sprintf("http://%s/", listener.Addr().String()),
		UseApplicationMenu: true,
	})
	desktop.window = window
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		desktop.saveWindowState()
		if !desktop.quitting {
			e.Cancel()
			window.Hide()
		}
	})
	if state.X != 0 || state.Y != 0 {
		window.SetPosition(state.X, state.Y)
	} else {
		window.Center()
	}
	window.Show()

	desktop.updaterOK = configureUpdater(app)
	configureMenu(app, desktop)
	configureTray(app, window, icon, desktop)
	if desktop.updaterOK {
		startBackgroundUpdateCheck(app)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
