package urlutil

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var invalidFilename = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)
var whitespace = regexp.MustCompile(`\s+`)

// Sanitize makes a string safe as a single file or directory name.
func Sanitize(name string) string {
	cleaned := invalidFilename.ReplaceAllString(name, "")
	cleaned = whitespace.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, ".")
	if cleaned == "" {
		return "Unknown"
	}
	return cleaned
}

// Normalize playlist URLs to a canonical youtube.com URL.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("playlist URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	var videoID string
	switch host {
	case "music.youtube.com", "m.youtube.com", "www.youtube.com":
		host = "youtube.com"
	case "youtu.be":
		videoID = strings.Trim(parsed.Path, "/")
		host = "youtube.com"
	case "youtube.com":
		// Keep as is.
	default:
		return "", fmt.Errorf("unsupported playlist URL host: %s", parsed.Hostname())
	}
	query := parsed.Query()
	list := strings.TrimSpace(query.Get("list"))
	if list == "" {
		return "", errors.New("URL must contain a playlist id ('list' parameter)")
	}
	values := url.Values{}
	values.Set("list", list)
	if videoID == "" && strings.TrimRight(parsed.Path, "/") == "/watch" {
		videoID = strings.TrimSpace(query.Get("v"))
	}
	path := parsed.Path
	if videoID != "" {
		values.Set("v", videoID)
		path = "/watch"
	} else if path == "" || path == "/" {
		path = "/playlist"
	}
	result := url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     path,
		RawQuery: values.Encode(),
	}
	return result.String(), nil
}
