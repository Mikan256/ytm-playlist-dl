package web

import "embed"

// Static contains the browser UI assets.
//
//go:embed static/*
var Static embed.FS
