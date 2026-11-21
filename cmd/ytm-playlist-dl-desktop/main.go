//go:build purego

package main

import (
	"log"

	"github.com/Mikan256/ytm-playlist-dl/internal/webui"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	if err := checkRuntime(); err != nil {
		log.Fatal(err)
	}
	handler := webui.NewHandler(webui.NewJobManager(nil))

	app := application.New(application.Options{
		Name:        "ytm-playlist-dl",
		Description: "YouTube Music playlist and video downloader",
		Assets: application.AssetOptions{
			Handler: handler,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "ytm-playlist-dl",
		Width:            1240,
		Height:           900,
		MinWidth:         900,
		MinHeight:        620,
		BackgroundColour: application.NewRGB(242, 230, 210),
		URL:              "http://wails.localhost/",
	})
	window.Center()
	window.Show()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
