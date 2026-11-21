//go:build purego && linux && gtk3

package main

import (
	"fmt"

	"github.com/ebitengine/purego"
)

func checkRuntime() error {
	if hasLibrary("libgtk-3.so.0") && hasLibrary("libwebkit2gtk-4.1.so.0") {
		return nil
	}
	return fmt.Errorf(
		"GTK3 and WebKit2GTK 4.1 runtime libraries are required for this build. " +
			"Install them, or use the default GTK4 build",
	)
}

func hasLibrary(name string) bool {
	handle, err := purego.Dlopen(name, purego.RTLD_LAZY)
	if err != nil {
		return false
	}
	_ = purego.Dlclose(handle)
	return true
}
