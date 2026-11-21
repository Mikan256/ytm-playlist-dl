package main

import (
	"os"

	"github.com/Mikan256/ytm-playlist-dl/internal/cli"
)

func main() {
	args := append([]string{"--web"}, os.Args[1:]...)
	os.Exit(cli.Main(args))
}
