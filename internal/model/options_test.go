package model

import (
	"testing"

	"github.com/Mikan256/ytm-playlist-dl/internal/config"
)

func TestOptionsFromMap(t *testing.T) {
	options, err := FromMap(map[string]any{
		"url":                  "https://youtube.com/playlist?list=PL1",
		"format":               "mp3",
		"loudness":             -16.0,
		"start":                2.0,
		"end":                  5.0,
		"keep_going":           true,
		"jobs":                 3.0,
		"normalize_jobs":       2.0,
		"template_string":      "  ",
		"embed_metadata":       true,
		"embed_thumbnail":      false,
		"force_normalize":      true,
		"zip":                  true,
		"concurrent_fragments": 4.0,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if options.Format != "mp3" || options.Loudness != -16 {
		t.Fatalf("unexpected options: %+v", options)
	}
	if options.Start == nil || *options.Start != 2 {
		t.Fatalf("start = %v", options.Start)
	}
	if options.End == nil || *options.End != 5 {
		t.Fatalf("end = %v", options.End)
	}
	if !options.KeepGoing || !options.ForceNormalize || !options.EmbedMetadata {
		t.Fatalf("boolean options not set: %+v", options)
	}
	if !options.Zip {
		t.Fatal("zip option not set")
	}
	if options.TemplateString != nil {
		t.Fatalf("blank template_string should be nil")
	}
	if options.Jobs != 3 || options.NormalizeJobs != 2 || options.ConcurrentFragments != 4 {
		t.Fatalf("numeric options: %+v", options)
	}
}

func TestOptionsValidate(t *testing.T) {
	valid := DefaultOptions("https://youtube.com/playlist?list=PL1")
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	start, end := 5, 2
	cases := []Options{
		{URL: ""},
		{URL: "url", Format: "ogg"},
		{URL: "url", Template: "bad"},
		{URL: "url", Loudness: -100},
		{URL: "url", Start: intPtr(0)},
		{URL: "url", Start: &start, End: &end},
		{URL: "url", Jobs: 0},
		{URL: "url", NormalizeJobs: 0},
		{URL: "url", ConcurrentFragments: 0},
	}
	for index, options := range cases {
		if err := options.Validate(); err == nil {
			t.Errorf("case %d should be invalid: %+v", index, options)
		}
	}
}

func TestOptionsConfigRoundTrip(t *testing.T) {
	options := DefaultOptions("url")
	options.Format = "mp3"
	options.Jobs = 2
	cfg := options.Config()
	if cfg.Format != "mp3" || cfg.Jobs != 2 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	back := OptionsFromConfig(cfg, "url")
	if back.Format != "mp3" || back.Jobs != 2 {
		t.Fatalf("unexpected options: %+v", back)
	}
	if !contains(config.FormatChoices, back.Format) {
		t.Fatal("format not from config choices")
	}
}

func intPtr(value int) *int { return &value }
