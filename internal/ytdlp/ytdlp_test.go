package ytdlp

import (
	"context"
	"runtime"
	"testing"

	"github.com/Mikan256/ytm-playlist-dl/internal/model"
	"github.com/Mikan256/ytm-playlist-dl/internal/testutil"
)

const fakeScript = `#!/bin/sh
case "$*" in
  *--dump-single-json*)
    cat <<'JSON'
{"title":"My: Playlist","entries":[
 {"id":"a","title":"First","url":"abc","duration":60},
 null,
 {"id":"b","title":"Second","webpage_url":"https://youtube.com/watch?v=b","playlist_index":5}
]}
JSON
    ;;
  *fail*)
    echo "ERROR: boom" >&2
    exit 1
    ;;
  *)
    printf 'YTM_PROGRESS\t50%%\t2\tSong Two\n'
    echo "some log"
    ;;
esac
`

func fakeClientWithScript(t *testing.T, script string) *Client {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture")
	}
	dir := t.TempDir()
	path := testutil.WriteExecutable(t, dir, "yt-dlp", script)
	return &Client{Binary: path}
}

func fakeClient(t *testing.T) *Client {
	return fakeClientWithScript(t, fakeScript)
}

func TestInfo(t *testing.T) {
	client := fakeClient(t)
	info, err := client.Info(context.Background(),
		"https://music.youtube.com/playlist?list=PL1&si=x")
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "My Playlist" || info.Total != 2 {
		t.Fatalf("info = %+v", info)
	}
	if info.URL != "https://youtube.com/playlist?list=PL1" {
		t.Fatalf("url = %q", info.URL)
	}
	if !info.IsPlaylist {
		t.Fatal("expected a playlist")
	}
	if info.Entries[0].Index != 1 || info.Entries[0].URL != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("entry 0 = %+v", info.Entries[0])
	}
	if info.Entries[1].Index != 5 {
		t.Fatalf("entry 1 = %+v", info.Entries[1])
	}
}

func TestInfoSingleVideo(t *testing.T) {
	client := fakeClientWithScript(t, `#!/bin/sh
cat <<'JSON'
{"_type":"video","id":"abc","title":"Single: Video","duration":42,"webpage_url":"https://youtube.com/watch?v=abc"}
JSON
`)
	info, err := client.Info(context.Background(),
		"https://www.youtube.com/watch?v=abc")
	if err != nil {
		t.Fatal(err)
	}
	if info.IsPlaylist || info.Total != 1 {
		t.Fatalf("info = %+v", info)
	}
	if info.Title != "Single Video" || info.Entries[0].ID != "abc" {
		t.Fatalf("info = %+v", info)
	}
	if info.Entries[0].Index != 1 || info.Entries[0].Duration != 42 {
		t.Fatalf("entry = %+v", info.Entries[0])
	}
}

func TestParseProgress(t *testing.T) {
	event, ok := ParseProgress("YTM_PROGRESS\t 42.5%\t3\tSong")
	if !ok {
		t.Fatal("expected progress event")
	}
	if event.Percent != 42.5 || event.Index != 3 || event.Title != "Song" {
		t.Fatalf("event = %+v", event)
	}
	if _, ok := ParseProgress("not progress"); ok {
		t.Fatal("unexpected progress event")
	}
}

func TestRunParsesEventsAndErrors(t *testing.T) {
	client := fakeClient(t)
	var events []string
	result := client.Run(context.Background(), []string{"progress"}, func(event model.Event) {
		events = append(events, event.Title)
	})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if len(events) != 1 || events[0] != "Song Two" {
		t.Fatalf("events = %v", events)
	}
	result = client.Run(context.Background(), []string{"fail"}, nil)
	if result.Err == nil {
		t.Fatal("expected failure")
	}
	if len(result.Errors) != 1 {
		t.Fatalf("errors = %v", result.Errors)
	}
}

func TestFindMissingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixture")
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("YTDLP_PATH", "")
	if _, err := Find(""); err == nil {
		t.Fatal("expected missing binary error")
	}
}
