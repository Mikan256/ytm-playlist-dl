package downloader

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/testutil"
)

const fakeDownloader = `#!/bin/sh
out=""
prev=""
for argument in "$@"; do
  if [ "$prev" = "-o" ]; then out="$argument"; fi
  prev="$argument"
done
dir=$(dirname "$out")
mkdir -p "$dir"
case "$out" in
  *01_*)
    : > "$dir/01_First Song.flac"
    printf 'YTM_PROGRESS\t100%%\t1\tFirst Song\n'
    ;;
  *02_*)
    : > "$dir/02_Second Song.flac"
    printf 'YTM_PROGRESS\t100%%\t2\tSecond Song\n'
    ;;
  *)
    : > "$dir/01_First Song.flac"
    : > "$dir/02_Second Song.flac"
    printf 'YTM_PROGRESS\t100%%\t1\tFirst Song\n'
    printf 'YTM_PROGRESS\t100%%\t2\tSecond Song\n'
    ;;
esac
exit 0
`

const failingDownloader = `#!/bin/sh
echo "ERROR: track unavailable" >&2
exit 1
`

func fakeDownloaderClient(t *testing.T, script string) *Downloader {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture")
	}
	dir := t.TempDir()
	path := testutil.WriteExecutable(t, dir, "yt-dlp", script)
	client, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func testInfo() model.PlaylistInfo {
	return model.PlaylistInfo{
		Title: "Test Playlist",
		Total: 2,
		URL:   "https://youtube.com/playlist?list=PL1",
		Entries: []model.Entry{
			{Index: 1, Title: "First Song", URL: "https://youtube.com/watch?v=a"},
			{Index: 2, Title: "Second Song", URL: "https://youtube.com/watch?v=b"},
		},
	}
}

func testOptions(output string) model.Options {
	options := model.DefaultOptions("https://youtube.com/playlist?list=PL1")
	options.Output = output
	options.Format = "original"
	options.NormalizeMode = "off"
	return options
}

func TestTemplates(t *testing.T) {
	if got := BaseTemplate("num_title", nil); got != "%(playlist_index)02d_%(title)s.%(ext)s" {
		t.Fatalf("base template = %q", got)
	}
	custom := "%(title)s.%(ext)s"
	if got := BaseTemplate("num_title", &custom); got != custom {
		t.Fatalf("custom template = %q", got)
	}
	if got := ApplyIndex("%(playlist_index)02d_%(title)s.%(ext)s", 7); got != "07_%(title)s.%(ext)s" {
		t.Fatalf("indexed template = %q", got)
	}
	if got := ApplyIndex("%(title)s.%(ext)s", 7); got != "07_%(title)s.%(ext)s" {
		t.Fatalf("prefixed template = %q", got)
	}
}

func TestPlaylistItems(t *testing.T) {
	start, end := 2, 5
	if got := PlaylistItems(nil, nil); got != "" {
		t.Fatalf("items = %q", got)
	}
	if got := PlaylistItems(&start, &end); got != "2-5" {
		t.Fatalf("items = %q", got)
	}
	if got := PlaylistItems(&start, nil); got != "2:" {
		t.Fatalf("items = %q", got)
	}
}

func TestSelectedEntries(t *testing.T) {
	start, end := 2, 2
	entries := SelectedEntries(testInfo(), &start, &end)
	if len(entries) != 1 || entries[0].Index != 2 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestPartialNames(t *testing.T) {
	for _, name := range []string{"a.part", "a.part-Frag1", "a.ytdl"} {
		if !IsPartial(name) {
			t.Errorf("%q should be partial", name)
		}
	}
	if IsPartial("a.flac") {
		t.Fatal("audio file should not be partial")
	}
}

func TestCollectFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"01_a.flac", "02_b.flac", "02_b.mp3", "03_c.flac.part",
		"100_d.flac", "11_e.flac", "cover.jpg",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := CollectFiles(dir, "flac")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"01_a.flac", "02_b.flac", "11_e.flac", "100_d.flac"}
	if len(files) != len(want) {
		t.Fatalf("files = %v", files)
	}
	for index, name := range want {
		if filepath.Base(files[index]) != name {
			t.Fatalf("file[%d] = %q, want %q", index, files[index], name)
		}
	}
}

func TestCleanupPartials(t *testing.T) {
	dir := t.TempDir()
	partial := filepath.Join(dir, "a.flac.part")
	audio := filepath.Join(dir, "a.flac")
	if err := os.WriteFile(partial, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(audio, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CleanupPartials(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatal("partial file was not removed")
	}
	if _, err := os.Stat(audio); err != nil {
		t.Fatal("audio file should remain")
	}
}

func TestDownloadSerial(t *testing.T) {
	client := fakeDownloaderClient(t, fakeDownloader)
	dir := t.TempDir()
	result, err := client.Download(context.Background(), testOptions(dir), testInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files = %v", result.Files)
	}
}

func TestDownloadParallel(t *testing.T) {
	client := fakeDownloaderClient(t, fakeDownloader)
	dir := t.TempDir()
	options := testOptions(dir)
	options.Jobs = 2
	result, err := client.Download(context.Background(), options, testInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files = %v", result.Files)
	}
}

func TestKeepGoingReturnsFailures(t *testing.T) {
	client := fakeDownloaderClient(t, failingDownloader)
	dir := t.TempDir()
	options := testOptions(dir)
	options.KeepGoing = true
	result, err := client.Download(context.Background(), options, testInfo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) == 0 {
		t.Fatal("expected failures")
	}
}

func TestNoFilesWithoutKeepGoingIsError(t *testing.T) {
	old := retryDelay
	retryDelay = func(int) time.Duration { return 0 }
	defer func() { retryDelay = old }()
	client := fakeDownloaderClient(t, failingDownloader)
	dir := t.TempDir()
	if _, err := client.Download(
		context.Background(), testOptions(dir), testInfo(), nil,
	); err == nil {
		t.Fatal("expected an error")
	}
}
