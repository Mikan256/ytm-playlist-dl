// Package binmgr locates external programs used by ytm-playlist-dl.
//
// The lookup order is designed for portable distributions that ship yt-dlp
// and ffmpeg next to the application:
//
//  1. explicit --yt-dlp / --ffmpeg path
//  2. YTDLP_PATH / FFMPEG_PATH / FFPROBE_PATH
//  3. next to the executable, or in an adjacent bin/ directory
//  4. in the current directory, or its bin/ directory
//  5. PATH
package binmgr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type Tool struct {
	Name string
	Env  []string
}

var (
	YtDlp   = Tool{Name: "yt-dlp", Env: []string{"YTDLP_PATH", "YT_DLP_PATH"}}
	FFmpeg  = Tool{Name: "ffmpeg", Env: []string{"FFMPEG_PATH"}}
	FFprobe = Tool{Name: "ffprobe", Env: []string{"FFPROBE_PATH"}}
)

func names(tool Tool) []string {
	if runtime.GOOS == "windows" {
		return []string{tool.Name + ".exe", tool.Name}
	}
	return []string{tool.Name}
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return false
	}
	return true
}

func inDir(dir string, tool Tool) (string, bool) {
	for _, name := range names(tool) {
		candidate := filepath.Join(dir, name)
		if executableFile(candidate) {
			return absolute(candidate), true
		}
	}
	return "", false
}

func candidatePath(value string, tool Tool) (string, bool) {
	if value == "" {
		return "", false
	}
	expanded := value
	if len(expanded) >= 2 && expanded[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = filepath.Join(home, expanded[2:])
		}
	}
	if info, err := os.Stat(expanded); err == nil {
		if info.IsDir() {
			return inDir(expanded, tool)
		}
		if executableFile(expanded) {
			return absolute(expanded), true
		}
	}
	if path, err := exec.LookPath(expanded); err == nil {
		return absolute(path), true
	}
	return "", false
}

func absolute(path string) string {
	if result, err := filepath.Abs(path); err == nil {
		return result
	}
	return path
}

func executableDir() string {
	path, err := executablePath()
	if err != nil {
		return ""
	}
	return filepath.Dir(path)
}

var (
	executablePath = os.Executable
	getwd          = os.Getwd
)

func Resolve(tool Tool, explicit string) (string, error) {
	if path, ok := candidatePath(explicit, tool); ok {
		return path, nil
	}
	for _, key := range tool.Env {
		if path, ok := candidatePath(os.Getenv(key), tool); ok {
			return path, nil
		}
	}
	if dir := executableDir(); dir != "" {
		if path, ok := inDir(dir, tool); ok {
			return path, nil
		}
		if path, ok := inDir(filepath.Join(dir, "bin"), tool); ok {
			return path, nil
		}
		if runtime.GOOS == "darwin" {
			// A macOS .app places the executable in Contents/MacOS and
			// resources in Contents/Resources.
			contents := filepath.Dir(dir)
			for _, candidate := range []string{
				filepath.Join(contents, "Resources", "bin"),
				filepath.Join(contents, "Resources"),
			} {
				if path, ok := inDir(candidate, tool); ok {
					return path, nil
				}
			}
		}
	}
	if cwd, err := getwd(); err == nil {
		if path, ok := inDir(cwd, tool); ok {
			return path, nil
		}
		if path, ok := inDir(filepath.Join(cwd, "bin"), tool); ok {
			return path, nil
		}
	}
	for _, name := range names(tool) {
		if path, err := exec.LookPath(name); err == nil {
			return absolute(path), nil
		}
	}
	return "", fmt.Errorf(
		"%s was not found. Put it next to the executable or in a bin/ directory, "+
			"set %s, or install it on PATH",
		tool.Name, envHint(tool),
	)
}

func envHint(tool Tool) string {
	if len(tool.Env) == 0 {
		return "the corresponding environment variable"
	}
	return tool.Env[0]
}

// Sibling returns a tool located in the same directory as another binary.
func Sibling(binary string, tool Tool) (string, bool) {
	if binary == "" {
		return "", false
	}
	return inDir(filepath.Dir(binary), tool)
}
