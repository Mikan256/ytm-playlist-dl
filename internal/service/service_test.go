package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/testutil"
)

const fakeYtDlp = `#!/bin/sh
case "$*" in
  *--dump-single-json*)
    cat <<'JSON'
{"title":"Service Playlist","entries":[
 {"id":"a","title":"First Song","url":"https://youtube.com/watch?v=a","duration":60},
 {"id":"b","title":"Second Song","url":"https://youtube.com/watch?v=b","duration":90}
]}
JSON
    ;;
  *)
    out=""
    previous=""
    for argument in "$@"; do
      if [ "$previous" = "-o" ]; then out="$argument"; fi
      previous="$argument"
    done
    dir=$(dirname "$out")
    mkdir -p "$dir"
    : > "$dir/01_First Song.flac"
    : > "$dir/02_Second Song.flac"
    ;;
esac
exit 0
`

const fakeFFmpeg = `#!/bin/sh
case "$*" in
  *loudnorm*)
    cat >&2 <<'JSON'
{"input_i":"-20.0","input_tp":"-3.0","input_lra":"5.0","input_thresh":"-30.0"}
JSON
    exit 0
    ;;
esac
input=""; previous=""; last=""
for argument in "$@"; do
  if [ "$previous" = "-i" ]; then input="$argument"; fi
  previous="$argument"; last="$argument"
done
if [ -n "$input" ]; then cp "$input" "$last"; fi
exit 0
`

const fakeFFprobe = `#!/bin/sh
echo 120.0
`

func fakePaths(t *testing.T) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture")
	}
	dir := t.TempDir()
	ytdlp := testutil.WriteExecutable(t, dir, "yt-dlp", fakeYtDlp)
	testutil.WriteExecutable(t, dir, "ffmpeg", fakeFFmpeg)
	testutil.WriteExecutable(t, dir, "ffprobe", fakeFFprobe)
	return ytdlp, filepath.Join(dir, "ffmpeg")
}

func baseOptions(t *testing.T) model.Options {
	t.Helper()
	ytdlp, ffmpeg := fakePaths(t)
	options := model.DefaultOptions("https://youtube.com/playlist?list=PL1")
	options.Output = t.TempDir()
	options.YtDlpPath = ytdlp
	options.FFmpegPath = ffmpeg
	return options
}

func TestListTracks(t *testing.T) {
	options := baseOptions(t)
	options.ListTracks = true
	result, err := (Service{}).Execute(context.Background(), options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Info.Total != 2 || result.Info.Title != "Service Playlist" {
		t.Fatalf("info = %+v", result.Info)
	}
	if len(result.Files) != 0 {
		t.Fatalf("files = %v", result.Files)
	}
}

func TestDryRun(t *testing.T) {
	options := baseOptions(t)
	options.DryRun = true
	options.Format = "original"
	start, end := 2, 2
	options.Start = &start
	options.End = &end
	result, err := (Service{}).Execute(context.Background(), options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Planned) != 1 || !containsSuffix(result.Planned[0], "02_Second Song.audio") {
		t.Fatalf("planned = %v", result.Planned)
	}
}

func TestDownloadAndNormalize(t *testing.T) {
	options := baseOptions(t)
	options.Format = "flac"
	options.NormalizeMode = "album"
	options.ReportFile = filepath.Join(options.Output, "report.json")
	result, err := (Service{}).Execute(context.Background(), options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 || result.Download == nil {
		t.Fatalf("result = %+v", result)
	}
	if result.Normalization == nil || len(result.Normalization.Normalized) != 2 {
		t.Fatalf("normalization = %+v", result.Normalization)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if _, err := os.Stat(result.ReportPath); err != nil {
		t.Fatal(err)
	}
}

func TestSingleVideoWithZip(t *testing.T) {
	_, ffmpeg := fakePaths(t)
	dir := t.TempDir()
	ytdlp := testutil.WriteExecutable(t, dir, "single-yt-dlp", `#!/bin/sh
case "$*" in
  *--dump-single-json*)
    echo '{"_type":"video","id":"a","title":"Single Song","duration":42,"webpage_url":"https://youtube.com/watch?v=a"}'
    ;;
  *)
    out=""
    previous=""
    for argument in "$@"; do
      if [ "$previous" = "-o" ]; then out="$argument"; fi
      previous="$argument"
    done
    dir=$(dirname "$out")
    mkdir -p "$dir"
    : > "$dir/01_Single Song.flac"
    ;;
esac
exit 0
`)
	options := model.DefaultOptions("https://youtube.com/watch?v=a")
	options.Output = t.TempDir()
	options.YtDlpPath = ytdlp
	options.FFmpegPath = ffmpeg
	options.Format = "original"
	options.NormalizeMode = "off"
	options.Zip = true
	result, err := (Service{}).Execute(context.Background(), options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Info.IsPlaylist || result.Info.Total != 1 {
		t.Fatalf("info = %+v", result.Info)
	}
	if len(result.Files) != 1 {
		t.Fatalf("files = %v", result.Files)
	}
	if result.ArchivePath == "" {
		t.Fatal("archive path is empty")
	}
	if _, err := os.Stat(result.ArchivePath); err != nil {
		t.Fatal(err)
	}
	if result.Report["archive_path"] != result.ArchivePath {
		t.Fatalf("report = %+v", result.Report)
	}
}

func TestPlaylistExport(t *testing.T) {
	options := baseOptions(t)
	options.Format = "original"
	options.NormalizeMode = "off"
	options.ExportFormat = "m3u"
	result, err := (Service{}).Execute(context.Background(), options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExportPath == "" {
		t.Fatal("export path is empty")
	}
	if _, err := os.Stat(result.ExportPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(result.ExportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSuffix(result.ExportPath, "playlist.m3u") ||
		!strings.Contains(string(raw), "#EXTM3U") {
		t.Fatalf("unexpected export: %s\n%s", result.ExportPath, raw)
	}
	if result.Report["export_path"] != result.ExportPath {
		t.Fatalf("report = %+v", result.Report)
	}
}

func TestPartialFailureExitCode(t *testing.T) {
	_, ffmpeg := fakePaths(t)
	dir := t.TempDir()
	failing := testutil.WriteExecutable(t, dir, "fail-yt-dlp", `#!/bin/sh
case "$*" in
  *--dump-single-json*)
    echo '{"title":"P","entries":[{"id":"a","title":"A","url":"https://youtube.com/watch?v=a"}]}'
    ;;
  *)
    echo "ERROR: track unavailable" >&2
    exit 1
    ;;
esac
exit 0
`)
	options := model.DefaultOptions("https://youtube.com/playlist?list=PL1")
	options.Output = t.TempDir()
	options.YtDlpPath = failing
	options.FFmpegPath = ffmpeg
	options.Format = "original"
	options.NormalizeMode = "off"
	options.KeepGoing = true
	result, err := (Service{}).Execute(context.Background(), options, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d, report = %+v", result.ExitCode, result.Report)
	}
	if len(result.Report["failed"].([]any)) == 0 {
		t.Fatalf("report = %+v", result.Report)
	}
}

func containsSuffix(value, suffix string) bool {
	return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
}
