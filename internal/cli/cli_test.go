package cli

import (
	"testing"

	"github.com/Mikan256/ytm-playlist-dl/internal/config"
)

func TestParseFlagsAroundURL(t *testing.T) {
	cfg := config.Default()
	parsed, err := parse([]string{
		"https://youtube.com/playlist?list=PL1",
		"--format", "mp3",
		"--template", "num_artist_title",
		"--loudness", "-16",
		"--start", "2",
		"--end", "5",
		"--keep-going",
		"--jobs", "3",
		"--zip",
		"--no-normalize",
	}, cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.url != "https://youtube.com/playlist?list=PL1" {
		t.Fatalf("url = %q", parsed.url)
	}
	if parsed.format != "mp3" || parsed.template != "num_artist_title" {
		t.Fatalf("parsed = %+v", parsed)
	}
	if parsed.loudness != -16 || parsed.start != 2 || parsed.end != 5 {
		t.Fatalf("parsed = %+v", parsed)
	}
	if !parsed.keepGoing || parsed.jobs != 3 {
		t.Fatalf("parsed = %+v", parsed)
	}
	if !parsed.zip {
		t.Fatalf("zip not set: %+v", parsed)
	}
	if parsed.normalizeMode != "off" {
		t.Fatalf("normalize mode = %q", parsed.normalizeMode)
	}
}

func TestReplayGainShortcut(t *testing.T) {
	parsed, err := parse([]string{"url", "--replaygain"}, config.Default(), "")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.normalizeMode != "replaygain" {
		t.Fatalf("normalize mode = %q", parsed.normalizeMode)
	}
}

func TestConflictingNormalizeFlags(t *testing.T) {
	if _, err := parse(
		[]string{"url", "--replaygain", "--no-normalize"},
		config.Default(), "",
	); err == nil {
		t.Fatal("expected an error")
	}
}

func TestInvalidRanges(t *testing.T) {
	for _, args := range [][]string{
		{"url", "--start", "0"},
		{"url", "--end", "0"},
		{"url", "--start", "5", "--end", "2"},
	} {
		if _, err := parse(args, config.Default(), ""); err == nil {
			t.Errorf("%v should be invalid", args)
		}
	}
}

func TestOptionsFromArgs(t *testing.T) {
	cfg := config.Default()
	parsed, err := parse([]string{
		"url", "--format", "mp3", "--start", "2", "--end", "3",
	}, cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	options := optionsFromArgs(parsed, cfg)
	if options.Format != "mp3" || options.Start == nil || *options.Start != 2 {
		t.Fatalf("options = %+v", options)
	}
	if options.NormalizeJobs != cfg.NormalizeJobs {
		t.Fatalf("normalize jobs = %d", options.NormalizeJobs)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := config.Default()
	cfg.Format = "mp3"
	cfg.Jobs = 4
	cfg.NormalizeMode = "replaygain"
	parsed, err := parse([]string{"url"}, cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.format != "mp3" || parsed.jobs != 4 || parsed.normalizeMode != "replaygain" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestSaveConfigFlag(t *testing.T) {
	parsed, err := parse([]string{"url", "--save-config"}, config.Default(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.saveConfig {
		t.Fatal("save-config not set")
	}
}
