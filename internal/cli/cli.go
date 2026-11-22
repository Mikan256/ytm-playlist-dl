package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/pflag"

	"github.com/Mikan256/ytm-playlist-dl/internal/binmgr"
	"github.com/Mikan256/ytm-playlist-dl/internal/config"
	"github.com/Mikan256/ytm-playlist-dl/internal/depupdate"
	"github.com/Mikan256/ytm-playlist-dl/internal/downloader"
	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/service"
	"github.com/Mikan256/ytm-playlist-dl/internal/webui"
)

var Version = "0.6.0"

type parsedArgs struct {
	url                 string
	format              string
	template            string
	templateString      string
	loudness            float64
	output              string
	normalizeMode       string
	noNormalize         bool
	replaygain          bool
	forceNormalize      bool
	start               int
	end                 int
	keepGoing           bool
	audioQuality        string
	embedMetadata       bool
	embedThumbnail      bool
	zip                 bool
	exportFormat        string
	jobs                int
	normalizeJobs       int
	concurrentFragments int
	dryRun              bool
	listTracks          bool
	reportFile          string
	logFile             string
	jsonOutput          bool
	configPath          string
	saveConfig          bool
	web                 bool
	host                string
	port                int
	openBrowser         bool
	ytdlpPath           string
	ffmpegPath          string
	updateYtdlp         bool
	doctor              bool
	version             bool
}

func scanConfigPath(args []string) string {
	for index, arg := range args {
		if arg == "--config" && index+1 < len(args) {
			return args[index+1]
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.SplitN(arg, "=", 2)[1]
		}
	}
	return ""
}

func Main(args []string) int {
	configPath := scanConfigPath(args)
	cfg, cfgErr := config.Load(configPath)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", cfgErr)
	}
	progressMu.Lock()
	printedDownload = map[int]bool{}
	progressMu.Unlock()
	parsed, err := parse(args, cfg, configPath)
	if err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}
	if parsed.version {
		fmt.Printf("ytm-playlist-dl %s\n", Version)
		return 0
	}
	if parsed.updateYtdlp {
		return updateYtDlp()
	}
	if parsed.doctor {
		return doctor()
	}
	if parsed.web {
		if err := webui.RunServer(parsed.host, parsed.port, parsed.openBrowser); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	options := optionsFromArgs(parsed, cfg)
	if parsed.saveConfig {
		saved, err := config.Save(options.Config(), parsed.configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not save config: %v\n", err)
			return 1
		}
		fmt.Printf("Saved config to: %s\n", saved)
	}
	if parsed.url == "" {
		return interactive(cfg)
	}
	if err := options.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer stop()
	printer := progressPrinter(parsed.jsonOutput)
	result, runErr := (service.Service{}).Execute(ctx, options, nil, printer)
	if runErr != nil {
		if !parsed.jsonOutput {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", runErr)
		}
	}
	if parsed.jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return result.ExitCode
	}
	printResult(result, parsed)
	return result.ExitCode
}

func parse(args []string, cfg config.Config, configPath string) (parsedArgs, error) {
	var parsed parsedArgs
	flags := pflag.NewFlagSet("ytm-playlist-dl", pflag.ContinueOnError)
	flags.SetInterspersed(true)

	flags.StringVar(&parsed.format, "format", cfg.Format, "output format: flac, mp3 or original")
	flags.StringVar(&parsed.template, "template", cfg.Template, "file name template")
	flags.StringVar(&parsed.templateString, "template-string", optional(cfg.TemplateString), "custom yt-dlp output template")
	flags.Float64Var(&parsed.loudness, "loudness", cfg.Loudness, "target loudness in LUFS")
	flags.StringVar(&parsed.output, "output", cfg.Output, "output directory")
	flags.StringVar(&parsed.normalizeMode, "normalize-mode", cfg.NormalizeMode, "album, replaygain or off")
	flags.BoolVar(&parsed.noNormalize, "no-normalize", false, "skip loudness normalisation")
	flags.BoolVar(&parsed.replaygain, "replaygain", false, "write ReplayGain tags without re-encoding")
	flags.BoolVar(&parsed.forceNormalize, "force-normalize", cfg.ForceNormalize, "normalise even when already recorded")
	flags.IntVar(&parsed.start, "start", 0, "first track number (1-based)")
	flags.IntVar(&parsed.end, "end", 0, "last track number (inclusive)")
	flags.BoolVar(&parsed.keepGoing, "keep-going", cfg.KeepGoing, "continue after individual track failures")
	flags.StringVar(&parsed.audioQuality, "audio-quality", optional(cfg.AudioQuality), "MP3 bitrate")
	flags.BoolVar(&parsed.embedMetadata, "embed-metadata", cfg.EmbedMetadata, "embed metadata")
	flags.BoolVar(&parsed.embedThumbnail, "embed-thumbnail", cfg.EmbedThumbnail, "embed cover art")
	flags.BoolVar(&parsed.zip, "zip", cfg.Zip, "create a zip archive of the output")
	flags.StringVar(&parsed.exportFormat, "export", cfg.ExportFormat, "playlist export: none, m3u, csv or json")
	flags.IntVar(&parsed.jobs, "jobs", cfg.Jobs, "parallel track downloads")
	flags.IntVar(&parsed.normalizeJobs, "normalize-jobs", cfg.NormalizeJobs, "parallel normalisation workers")
	flags.IntVar(&parsed.concurrentFragments, "concurrent-fragments", cfg.ConcurrentFragments, "parallel fragments per track")
	flags.BoolVar(&parsed.dryRun, "dry-run", false, "show planned output without downloading")
	flags.BoolVar(&parsed.listTracks, "list-tracks", false, "list playlist tracks and exit")
	flags.StringVar(&parsed.reportFile, "report-file", "", "write a JSON report")
	flags.StringVar(&parsed.logFile, "log-file", "", "append a text log")
	flags.BoolVar(&parsed.jsonOutput, "json", false, "print the result as JSON")
	flags.StringVar(&parsed.configPath, "config", configPath, "path to a JSON config file")
	flags.BoolVar(&parsed.saveConfig, "save-config", false, "save settings to the config file")
	flags.BoolVar(&parsed.web, "web", false, "start the browser UI")
	flags.StringVar(&parsed.host, "host", config.DefaultHost, "web UI bind address")
	flags.IntVar(&parsed.port, "port", config.DefaultPort, "web UI port")
	flags.BoolVar(&parsed.openBrowser, "open-browser", false, "open the web UI in a browser")
	flags.StringVar(&parsed.ytdlpPath, "yt-dlp", "", "path to the yt-dlp binary")
	flags.StringVar(&parsed.ffmpegPath, "ffmpeg", "", "path to the ffmpeg binary")
	flags.BoolVar(&parsed.updateYtdlp, "update-ytdlp", false, "download the latest yt-dlp binary")
	flags.BoolVar(&parsed.doctor, "doctor", false, "show resolved binary paths and versions")
	flags.BoolVar(&parsed.version, "version", false, "print version and exit")

	if err := flags.Parse(args); err != nil {
		return parsed, err
	}
	if parsed.noNormalize && parsed.replaygain {
		return parsed, errors.New("--no-normalize and --replaygain cannot be used together")
	}
	if parsed.noNormalize {
		parsed.normalizeMode = "off"
	}
	if parsed.replaygain {
		parsed.normalizeMode = "replaygain"
	}
	positionals := flags.Args()
	if len(positionals) > 1 {
		return parsed, fmt.Errorf("unexpected arguments: %s", strings.Join(positionals[1:], " "))
	}
	if len(positionals) == 1 {
		parsed.url = positionals[0]
	}
	if flags.Changed("start") && parsed.start < 1 {
		return parsed, errors.New("--start must be 1 or greater")
	}
	if flags.Changed("end") && parsed.end < 1 {
		return parsed, errors.New("--end must be 1 or greater")
	}
	if parsed.start > 0 && parsed.end > 0 && parsed.end < parsed.start {
		return parsed, errors.New("--end must be greater than or equal to --start")
	}
	return parsed, nil
}

func optionsFromArgs(parsed parsedArgs, cfg config.Config) model.Options {
	options := model.OptionsFromConfig(cfg, parsed.url)
	options.Format = parsed.format
	options.Template = parsed.template
	options.TemplateString = optionalPointer(parsed.templateString)
	options.Loudness = parsed.loudness
	options.Output = parsed.output
	options.NormalizeMode = parsed.normalizeMode
	options.ForceNormalize = parsed.forceNormalize
	options.KeepGoing = parsed.keepGoing
	options.AudioQuality = optionalPointer(parsed.audioQuality)
	options.EmbedMetadata = parsed.embedMetadata
	options.EmbedThumbnail = parsed.embedThumbnail
	options.Zip = parsed.zip
	options.ExportFormat = parsed.exportFormat
	options.Jobs = parsed.jobs
	options.NormalizeJobs = parsed.normalizeJobs
	options.ConcurrentFragments = parsed.concurrentFragments
	options.DryRun = parsed.dryRun
	options.ListTracks = parsed.listTracks
	options.LogFile = parsed.logFile
	options.ReportFile = parsed.reportFile
	options.YtDlpPath = parsed.ytdlpPath
	options.FFmpegPath = parsed.ffmpegPath
	if parsed.start > 0 {
		value := parsed.start
		options.Start = &value
	}
	if parsed.end > 0 {
		value := parsed.end
		options.End = &value
	}
	return options
}

func optionalPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func optional(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var progressMu sync.Mutex
var printedDownload = map[int]bool{}

func progressPrinter(suppressStatus bool) func(model.Event) {
	return func(event model.Event) {
		progressMu.Lock()
		defer progressMu.Unlock()
		switch event.Type {
		case "download":
			if event.Percent < 99.5 {
				return
			}
			index := event.Index
			if index == 0 || printedDownload[index] {
				return
			}
			printedDownload[index] = true
			total := event.Total
			if total == 0 {
				total = index
			}
			fmt.Fprintf(os.Stderr, "\r  [%d/%d] Downloaded: %s\n", index, total, event.Title)
		case "normalize":
			phase := "Normalising"
			if event.Phase == "analysing" {
				phase = "Analysing"
			}
			fmt.Fprintf(os.Stderr, "\r  [%d/%d] %s: %s\n",
				event.Current, event.Total, phase, event.Label)
		case "status":
			if !suppressStatus && event.Message != "" {
				fmt.Fprintf(os.Stderr, "  %s\n", event.Message)
			}
		case "error":
			fmt.Fprintf(os.Stderr, "  Error: %s\n", event.Message)
		}
	}
}

func printResult(result model.RunResult, parsed parsedArgs) {
	if parsed.listTracks {
		kind := "Playlist"
		if !result.Info.IsPlaylist {
			kind = "Video"
		}
		fmt.Printf("\n%s: %s (%d tracks)\n", kind, result.Info.Title, result.Info.Total)
		for _, entry := range result.Info.Entries {
			duration := ""
			if entry.Duration > 0 {
				duration = fmt.Sprintf(" (%ds)", int(entry.Duration))
			}
			fmt.Printf("  %02d. %s%s\n", entry.Index, entry.Title, duration)
		}
		fmt.Println()
		return
	}
	if parsed.dryRun {
		fmt.Printf("\nDry run: %d track(s)\n", len(result.Planned))
		for _, path := range result.Planned {
			fmt.Printf("  %s\n", path)
		}
		fmt.Println()
		return
	}
	fmt.Printf("\nDownloaded %d/%d tracks\n", len(result.Files), result.Info.Total)
	if result.Download != nil && len(result.Download.Failed) > 0 {
		fmt.Fprintln(os.Stderr, "Failed tracks:")
		for index, message := range result.Download.Failed {
			if index >= 10 {
				break
			}
			fmt.Fprintf(os.Stderr, "  - %s\n", message)
		}
	}
	if result.Normalization != nil && len(result.Normalization.Normalized) > 0 {
		fmt.Printf("Normalised %d tracks (%s)\n",
			len(result.Normalization.Normalized), result.Normalization.Mode)
	}
	fmt.Printf("\nDone! Saved to: %s\n", result.OutputDir)
	fmt.Printf("Total: %d tracks\n", len(result.Files))
	if result.ArchivePath != "" {
		fmt.Printf("Archive: %s\n", result.ArchivePath)
	}
	if result.ExportPath != "" {
		fmt.Printf("Playlist: %s\n", result.ExportPath)
	}
	if result.ReportPath != "" {
		fmt.Printf("Report: %s\n", result.ReportPath)
	}
	if result.ExitCode == 2 {
		fmt.Fprintln(os.Stderr, "Some tracks failed; see the report for details.")
	}
}

func updateYtDlp() int {
	fmt.Println("Downloading the latest yt-dlp...")
	path, err := depupdate.UpdateYtDlp(context.Background(), depupdate.TargetDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Printf("yt-dlp updated: %s\n", path)
	return 0
}

func doctor() int {
	checks := []struct {
		label string
		tool  binmgr.Tool
		args  []string
	}{
		{"yt-dlp", binmgr.YtDlp, []string{"--version"}},
		{"ffmpeg", binmgr.FFmpeg, []string{"-version"}},
		{"ffprobe", binmgr.FFprobe, []string{"-version"}},
	}
	failed := false
	for _, check := range checks {
		path, err := binmgr.Resolve(check.tool, "")
		if err != nil {
			fmt.Printf("%-8s : NOT FOUND\n", check.label)
			failed = true
			continue
		}
		output, err := exec.Command(path, check.args...).CombinedOutput()
		version := strings.TrimSpace(string(output))
		if index := strings.IndexByte(version, '\n'); index >= 0 {
			version = version[:index]
		}
		if err != nil {
			version = "found, but failed to run"
			failed = true
		}
		fmt.Printf("%-8s : %s\n           %s\n", check.label, path, version)
	}
	if failed {
		return 1
	}
	return 0
}

func interactive(cfg config.Config) int {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("  ytm-playlist-dl")
	fmt.Println("  YouTube Music Playlist Downloader")
	fmt.Println(strings.Repeat("=", 50))
	rawURL := prompt(reader, "Enter playlist URL", "")
	if strings.TrimSpace(rawURL) == "" {
		fmt.Println("URL is required")
		return 1
	}
	client, err := downloader.New("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Println("\nFetching playlist info...")
	info, err := client.Info(context.Background(), rawURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	kind := "Playlist"
	if !info.IsPlaylist {
		kind = "Video"
	}
	fmt.Printf("\n%s: %s\nTracks:   %d\n\n", kind, info.Title, info.Total)

	fmt.Println("Output format:")
	for _, key := range config.FormatChoices {
		fmt.Printf("  %s: %s\n", key, config.FormatDescriptions[key])
	}
	format := promptChoice(reader, "Output format", cfg.Format, config.FormatChoices)
	template := promptTemplate(reader, cfg.Template)
	normalizeMode := promptChoice(reader, "Normalise mode", cfg.NormalizeMode, config.NormalizeModes)
	loudness := cfg.Loudness
	if normalizeMode != "off" {
		fmt.Println("  Target loudness (LUFS):")
		fmt.Println("    -14 = YouTube/Spotify 標準 (おすすめ)")
		fmt.Println("    -16 = やや控えめ")
		fmt.Println("    -23 = EBU R128 推奨値 (最も控えめ)")
		loudness = promptIntFloat(reader, "  Value", cfg.Loudness,
			config.LoudnessMin, config.LoudnessMax)
	}
	output := prompt(reader, "Output directory", cfg.Output)
	if !promptBool(reader, "Start download?", true) {
		fmt.Println("Cancelled.")
		return 0
	}
	options := model.OptionsFromConfig(cfg, rawURL)
	options.Format = format
	options.Template = template
	options.NormalizeMode = normalizeMode
	options.Loudness = loudness
	options.Output = output
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer stop()
	result, runErr := (service.Service{}).Execute(ctx, options, &info, progressPrinter(false))
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", runErr)
	}
	printResult(result, parsedArgs{})
	return result.ExitCode
}

func prompt(reader *bufio.Reader, message, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", message, defaultValue)
	} else {
		fmt.Printf("%s: ", message)
	}
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}

func promptChoice(reader *bufio.Reader, message, defaultValue string,
	choices []string) string {
	for {
		value := strings.ToLower(prompt(reader, message, defaultValue))
		for _, choice := range choices {
			if value == choice {
				return value
			}
		}
		fmt.Printf("  Please choose from: %s\n", strings.Join(choices, ", "))
	}
}

func promptTemplate(reader *bufio.Reader, defaultValue string) string {
	fmt.Println("\nFile naming:")
	keys := config.TemplateChoices
	defaultIndex := 1
	for index, key := range keys {
		if key == defaultValue {
			defaultIndex = index + 1
		}
		fmt.Printf("  %d. %s\n", index+1, config.TemplateLabels[key])
	}
	for {
		value := prompt(reader, "  Select", strconv.Itoa(defaultIndex))
		number, err := strconv.Atoi(value)
		if err == nil && number >= 1 && number <= len(keys) {
			return keys[number-1]
		}
		for _, key := range keys {
			if value == key {
				return key
			}
		}
		fmt.Printf("  Please enter a number 1-%d\n", len(keys))
	}
}

func promptIntFloat(reader *bufio.Reader, message string, defaultValue,
	minValue, maxValue float64) float64 {
	for {
		value := prompt(reader, message, strconv.FormatFloat(defaultValue, 'f', -1, 64))
		number, err := strconv.ParseFloat(value, 64)
		if err == nil && number >= minValue && number <= maxValue {
			return number
		}
		fmt.Printf("  Please enter a number between %g and %g\n", minValue, maxValue)
	}
}

func promptBool(reader *bufio.Reader, message string, defaultValue bool) bool {
	hint := "Y/n"
	if !defaultValue {
		hint = "y/N"
	}
	fmt.Printf("%s [%s]: ", message, hint)
	value, _ := reader.ReadString('\n')
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return defaultValue
	}
	return strings.HasPrefix(value, "y")
}
