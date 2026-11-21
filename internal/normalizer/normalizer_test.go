package normalizer

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/testutil"
)

const fakeFFmpeg = `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_FFMPEG_LOG"
case "$*" in
  *loudnorm*)
    cat >&2 <<'JSON'
{
  "input_i": "-20.0",
  "input_tp": "-3.0",
  "input_lra": "5.0",
  "input_thresh": "-30.0"
}
JSON
    exit 0
    ;;
esac
input=""
previous=""
last=""
for argument in "$@"; do
  if [ "$previous" = "-i" ]; then input="$argument"; fi
  previous="$argument"
  last="$argument"
done
if [ -n "$input" ]; then cp "$input" "$last"; fi
exit 0
`

const fakeFFprobe = `#!/bin/sh
echo 120.0
`

func TestParseLoudnormJSON(t *testing.T) {
	parsed, ok := parseLoudnormJSON("log line\n{\n\"input_i\":\"-23.4\",\n\"input_tp\":\"-3.1\"\n}\n")
	if !ok || parsed.InputI != "-23.4" || parsed.InputTP != "-3.1" {
		t.Fatalf("parsed = %+v ok=%v", parsed, ok)
	}
	if _, ok := parseLoudnormJSON("no json"); ok {
		t.Fatal("unexpected success")
	}
}

func TestComputeAlbumLoudness(t *testing.T) {
	album := ComputeAlbumLoudness([]model.Measurement{
		{InputI: -20, Duration: 10},
		{InputI: -10, Duration: 10},
	})
	if album == nil {
		t.Fatal("album loudness is nil")
	}
	want := 10 * math.Log10((math.Pow(10, -2)+math.Pow(10, -1))/2)
	if math.Abs(*album-want) > 1e-9 {
		t.Fatalf("album = %f, want %f", *album, want)
	}
	if ComputeAlbumLoudness([]model.Measurement{{InputI: -20, Duration: 0}}) != nil {
		t.Fatal("zero duration should be ignored")
	}
}

func TestCodecForPath(t *testing.T) {
	cases := map[string]string{
		"a.flac": "flac", "a.MP3": "libmp3lame", "a.m4a": "aac",
		"a.opus": "libopus", "a.webm": "libopus", "a.xyz": "",
	}
	for path, want := range cases {
		if got := codecForPath(path); got != want {
			t.Errorf("codecForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func fakeProcessor(t *testing.T) (*Processor, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "ffmpeg.log")
	t.Setenv("FAKE_FFMPEG_LOG", log)
	testutil.WriteExecutable(t, dir, "ffmpeg", fakeFFmpeg)
	testutil.WriteExecutable(t, dir, "ffprobe", fakeFFprobe)
	processor, err := New(filepath.Join(dir, "ffmpeg"))
	if err != nil {
		t.Fatal(err)
	}
	return processor, log
}

func tempAudioFiles(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "01_a.flac"),
		filepath.Join(dir, "02_b.flac"),
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func TestNormalizeAlbumCommandFlow(t *testing.T) {
	processor, log := fakeProcessor(t)
	files := tempAudioFiles(t)
	result, err := processor.Normalize(
		context.Background(), files, -14, "album", false, "", 2, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Normalized) != 2 {
		t.Fatalf("normalized = %v", result.Normalized)
	}
	if result.GainDB == nil || math.Abs(*result.GainDB-6) > 1e-9 {
		t.Fatalf("gain = %v", result.GainDB)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "loudnorm") ||
		!strings.Contains(text, "volume=6.000000dB") ||
		!strings.Contains(text, "-c:a flac") {
		t.Fatalf("unexpected ffmpeg log:\n%s", text)
	}
}

func TestManifestSkipsSecondRun(t *testing.T) {
	processor, _ := fakeProcessor(t)
	files := tempAudioFiles(t)
	first, err := processor.Normalize(
		context.Background(), files, -14, "album", false, "", 1, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Normalized) != 2 {
		t.Fatalf("first normalized = %v", first.Normalized)
	}
	second, err := processor.Normalize(
		context.Background(), files, -14, "album", false, "", 1, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Normalized) != 0 || len(second.Skipped) != 2 {
		t.Fatalf("second result = %+v", second)
	}
}

func TestReplayGainCommandFlow(t *testing.T) {
	processor, log := fakeProcessor(t)
	files := tempAudioFiles(t)
	result, err := processor.Normalize(
		context.Background(), files, -14, "replaygain", false, "", 1, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Normalized) != 2 {
		t.Fatalf("normalized = %v", result.Normalized)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "REPLAYGAIN_TRACK_GAIN=+6.00 dB") ||
		!strings.Contains(text, "REPLAYGAIN_ALBUM_GAIN=+6.00 dB") ||
		!strings.Contains(text, "-c copy") {
		t.Fatalf("unexpected ffmpeg log:\n%s", text)
	}
}

func TestModeOff(t *testing.T) {
	processor, _ := fakeProcessor(t)
	files := tempAudioFiles(t)
	result, err := processor.Normalize(
		context.Background(), files, -14, "off", false, "", 1, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "off" || len(result.Files) != 2 {
		t.Fatalf("result = %+v", result)
	}
}
