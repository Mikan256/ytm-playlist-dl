package main

import (
	"os"

	"github.com/Mikan256/ytm-playlist-dl/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
