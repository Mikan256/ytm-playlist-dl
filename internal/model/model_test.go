package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWithSlicesUsesJSONArrays(t *testing.T) {
	download := DownloadResult{}.WithSlices()
	normalization := NormalizationResult{}.WithSlices()
	raw, err := json.Marshal(map[string]any{
		"download": download, "normalization": normalization,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, `"files":null`) ||
		strings.Contains(text, `"normalized":null`) ||
		strings.Contains(text, `"skipped":null`) ||
		strings.Contains(text, `"failed":null`) {
		t.Fatalf("unexpected null array in %s", text)
	}
	if !strings.Contains(text, `"files":[]`) ||
		!strings.Contains(text, `"normalized":[]`) ||
		!strings.Contains(text, `"skipped":[]`) ||
		!strings.Contains(text, `"failed":[]`) {
		t.Fatalf("unexpected JSON: %s", text)
	}
}
