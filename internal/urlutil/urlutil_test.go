package urlutil

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			"https://music.youtube.com/playlist?list=PL123&si=tracking",
			"https://youtube.com/playlist?list=PL123",
		},
		{
			"www.youtube.com/playlist?list=PL123&index=4",
			"https://youtube.com/playlist?list=PL123",
		},
		{
			"http://m.youtube.com/watch?v=abc&list=PL123&t=10",
			"https://youtube.com/watch?list=PL123&v=abc",
		},
		{
			"https://youtu.be/abc?list=PL123",
			"https://youtube.com/watch?list=PL123&v=abc",
		},
	}
	for _, tc := range cases {
		got, err := Normalize(tc.input)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeErrors(t *testing.T) {
	for _, input := range []string{
		"",
		"https://youtube.com/watch?v=abc",
		"https://example.com/playlist?list=PL1",
	} {
		if _, err := Normalize(input); err == nil {
			t.Errorf("Normalize(%q) expected an error", input)
		}
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		`A/B\C:D*E?F"G<H>I|J`: "ABCDEFGHIJ",
		"  Song   Title.  ":   "Song Title",
		"... ":                "Unknown",
		"夜に駆ける":               "夜に駆ける",
	}
	for input, want := range cases {
		if got := Sanitize(input); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
		}
	}
}
