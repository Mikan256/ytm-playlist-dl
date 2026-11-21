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

// Normalize converts playlist, video, Shorts and youtu.be URLs to a canonical
// youtube.com URL. Extra tracking parameters are discarded.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("YouTube URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "music.youtube.com", "m.youtube.com", "www.youtube.com":
		host = "youtube.com"
	case "youtu.be":
		host = "youtube.com"
	case "youtube.com":
		// Keep as is.
	default:
		return "", fmt.Errorf("unsupported YouTube URL host: %s", parsed.Hostname())
	}

	query := parsed.Query()
	list := strings.TrimSpace(query.Get("list"))
	videoID := ""
	if strings.EqualFold(parsed.Hostname(), "youtu.be") {
		videoID = strings.Trim(parsed.Path, "/")
	} else if strings.TrimRight(parsed.Path, "/") == "/watch" {
		videoID = strings.TrimSpace(query.Get("v"))
	} else {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) == 2 {
			switch parts[0] {
			case "shorts", "live", "embed":
				videoID = parts[1]
			}
		}
	}

	if list == "" && videoID == "" {
		return "", errors.New("URL must contain a video id or playlist id")
	}

	values := url.Values{}
	if list != "" {
		values.Set("list", list)
	}
	if videoID != "" {
		values.Set("v", videoID)
	}

	path := parsed.Path
	if videoID != "" {
		path = "/watch"
	} else {
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
