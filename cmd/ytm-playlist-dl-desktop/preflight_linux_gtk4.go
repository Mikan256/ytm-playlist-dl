//go:build purego && linux && !gtk3

package main

import (
	"fmt"

	"github.com/ebitengine/purego"
)

func checkRuntime() error {
	if hasLibrary("libgtk-4.so.1") && hasLibrary("libwebkitgtk-6.0.so.4") {
		return nil
	}
	return fmt.Errorf(
		"GTK4 and WebKitGTK 6.0 runtime libraries are required on Linux. " +
			"Install them, or build with -tags purego,gtk3,production for " +
			"GTK3 / WebKit2GTK 4.1",
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
