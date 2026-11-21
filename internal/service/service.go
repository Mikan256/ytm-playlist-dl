package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mikan256/ytm-playlist-dl/internal/downloader"
	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/normalizer"
	"github.com/Mikan256/ytm-playlist-dl/internal/report"
)

type Service struct{}

func (Service) Execute(ctx context.Context, options model.Options,
	info *model.PlaylistInfo, progress func(model.Event)) (model.RunResult, error) {
	if err := options.Validate(); err != nil {
		return model.RunResult{Options: options}, err
	}
	emit(progress, model.Event{Type: "status", Message: "プレイリスト情報を取得中..."})

	downloaderClient, err := downloader.New(options.YtDlpPath)
	if err != nil {
		return model.RunResult{Options: options}, err
	}
	if info == nil {
		fetched, err := downloaderClient.Info(ctx, options.URL)
		if err != nil {
			return model.RunResult{Options: options}, err
		}
		info = &fetched
	}
	result := model.RunResult{Options: options, Info: *info}
	outputDir := filepath.Join(expandHome(options.Output), info.Title)
	result.OutputDir = outputDir

	if options.ListTracks {
		runReport := report.Build(options, *info, outputDir, nil, nil, nil)
		result.Report = reportMap(runReport)
		if options.ReportFile != "" {
			path, err := report.Write(options.ReportFile, runReport)
			if err != nil {
				return result, err
			}
			result.ReportPath = path
		}
		return result, nil
	}

	if options.DryRun {
		result.Planned = plannedFiles(*info, outputDir, options)
		emit(progress, model.Event{
			Type: "status",
			Message: fmt.Sprintf(
				"ドライラン: %d 曲を %s に保存予定",
				len(result.Planned), outputDir,
			),
		})
		runReport := report.Build(options, *info, outputDir, nil, nil, nil)
		result.Report = reportMap(runReport)
		if options.ReportFile != "" {
			path, err := report.Write(options.ReportFile, runReport)
			if err != nil {
				return result, err
			}
			result.ReportPath = path
		}
		return result, nil
	}

	appendLog(options.LogFile, "download start: "+options.URL)
	emit(progress, model.Event{Type: "status", Message: "ダウンロードを開始しました"})
	downloadResult, downloadErr := downloaderClient.Download(
		ctx, options, *info, progress,
	)
	if downloadErr != nil {
		return finishWithError(options, *info, outputDir, &downloadResult,
			nil, downloadErr, progress, &result)
	}
	result.Download = &downloadResult
	result.Files = append([]string{}, downloadResult.Files...)
	appendLog(options.LogFile, fmt.Sprintf(
		"download finished: %d files, %d failed",
		len(downloadResult.Files), len(downloadResult.Failed),
	))

	var normalization *model.NormalizationResult
	if options.NormalizeMode != "off" && len(downloadResult.Files) > 0 {
		emit(progress, model.Event{
			Type: "status",
			Message: fmt.Sprintf(
				"音量処理を開始しました（%s）", options.NormalizeMode,
			),
		})
		processor, err := normalizer.New(options.FFmpegPath)
		if err != nil {
			return finishWithError(options, *info, outputDir, &downloadResult,
				nil, err, progress, &result)
		}
		processed, err := processor.Normalize(
			ctx, downloadResult.Files, options.Loudness, options.NormalizeMode,
			options.ForceNormalize, "", options.NormalizeJobs, progress,
		)
		if err != nil {
			return finishWithError(options, *info, outputDir, &downloadResult,
				nil, err, progress, &result)
		}
		normalization = &processed
		result.Normalization = normalization
		appendLog(options.LogFile, fmt.Sprintf(
			"normalisation finished: %d processed, %d skipped, %d failed",
			len(processed.Normalized), len(processed.Skipped), len(processed.Failed),
		))
	}

	runReport := report.Build(options, *info, outputDir, &downloadResult,
		normalization, nil)
	result.Report = reportMap(runReport)
	result.ExitCode = runReport.ExitCode
	if options.ReportFile != "" {
		path, err := report.Write(options.ReportFile, runReport)
		if err != nil {
			return result, err
		}
		result.ReportPath = path
	}
	exitCode := result.ExitCode
	emit(progress, model.Event{
		Type: "status",
		Message: map[bool]string{
			true:  "完了しました",
			false: "一部の曲でエラーが発生しました",
		}[exitCode == 0],
		ExitCode: &exitCode,
	})
	return result, nil
}

func finishWithError(options model.Options, info model.PlaylistInfo,
	outputDir string, download *model.DownloadResult,
	normalization *model.NormalizationResult, runErr error,
	progress func(model.Event), result *model.RunResult) (model.RunResult, error) {
	result.Download = download
	result.Normalization = normalization
	if download != nil {
		result.Files = append([]string{}, download.Files...)
	}
	runReport := report.Build(options, info, outputDir, download, normalization, runErr)
	result.Report = reportMap(runReport)
	result.ExitCode = runReport.ExitCode
	if options.ReportFile != "" {
		path, err := report.Write(options.ReportFile, runReport)
		if err == nil {
			result.ReportPath = path
		}
	}
	emit(progress, model.Event{Type: "error", Message: runErr.Error()})
	return *result, runErr
}

func plannedFiles(info model.PlaylistInfo, outputDir string,
	options model.Options) []string {
	extension := map[string]string{
		"flac": "flac", "mp3": "mp3", "original": "audio",
	}[options.Format]
	if extension == "" {
		extension = "audio"
	}
	ordered := downloader.SelectedEntries(info, options.Start, options.End)
	planned := make([]string, 0, len(ordered))
	for _, entry := range ordered {
		planned = append(planned, filepath.Join(
			outputDir,
			fmt.Sprintf("%02d_%s.%s", entry.Index, entry.Title, extension),
		))
	}
	return planned
}

func reportMap(value report.Report) map[string]any {
	data, _ := json.Marshal(value)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result
}

func emit(progress func(model.Event), event model.Event) {
	if progress == nil {
		return
	}
	if event.At == 0 {
		event.At = float64(time.Now().UnixNano()) / 1e9
	}
	progress(event)
}

func appendLog(path, message string) {
	if path == "" {
		return
	}
	_ = report.AppendLog(path, message)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
