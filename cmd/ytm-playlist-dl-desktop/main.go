//go:build purego

package main

import (
	_ "embed"
	"log"

	"github.com/Mikan256/ytm-playlist-dl/internal/webui"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed assets/icon.png
var icon []byte

func main() {
	if err := checkRuntime(); err != nil {
		log.Fatal(err)
	}

	desktop := &desktopHandler{}
	app := application.New(application.Options{
		Name:        "ytm-playlist-dl",
		Description: "YouTube Music playlist and video downloader",
		Assets: application.AssetOptions{
			Handler: desktop,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	desktop.app = app
	desktop.base = webui.NewHandler(webui.NewJobManager(nil))

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:              "ytm-playlist-dl",
		Width:              1240,
		Height:             900,
		MinWidth:           900,
		MinHeight:          620,
		BackgroundColour:   application.NewRGB(242, 230, 210),
		URL:                "http://wails.localhost/",
		UseApplicationMenu: true,
	})
	desktop.window = window
	window.Center()
	window.Show()

	desktop.updaterOK = configureUpdater(app)
	configureMenu(app, desktop.updaterOK)
	configureTray(app, window, icon, desktop.updaterOK)
	if desktop.updaterOK {
		startBackgroundUpdateCheck(app)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
