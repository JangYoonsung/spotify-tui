package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

func TestFormatMs(t *testing.T) {
	cases := []struct {
		ms   int
		want string
	}{
		{0, "0:00"},
		{5000, "0:05"},
		{65000, "1:05"},
		{600000, "10:00"},
	}
	for _, tc := range cases {
		if got := formatMs(tc.ms); got != tc.want {
			t.Errorf("formatMs(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

func TestNowPlayingLinesNoArt(t *testing.T) {
	s := spotifyapi.PlaybackState{
		IsPlaying:  true,
		Item:       spotifyapi.Track{Name: "Song", Artists: []string{"Artist"}, DurationMs: 100000},
		ProgressMs: 50000,
		Device:     spotifyapi.Device{Name: "Dev", VolumePercent: 50},
	}
	lines := nowPlayingLines(s, "", "", false, 56, 0)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (no art -> track/progress/status only)", len(lines))
	}
	for _, l := range lines {
		if strings.Contains(l, "\n") {
			t.Fatalf("line contains embedded newline, would break boxRow: %q", l)
		}
	}

	// With a known next track, an extra "up next" line appears.
	lines = nowPlayingLines(s, "", "Next Song — Next Artist", false, 56, 0)
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (track/progress/status/next)", len(lines))
	}
	if !strings.Contains(lines[3], "Next Song") {
		t.Fatalf("next line missing track: %q", lines[3])
	}
}

func TestNowPlayingLinesWithArt(t *testing.T) {
	s := spotifyapi.PlaybackState{
		Item:       spotifyapi.Track{Name: "Song", DurationMs: 100000},
		ProgressMs: 0,
	}
	art := strings.Join([]string{"AAAA", "BBBB", "CCCC", "DDDD"}, "\n")
	lines := nowPlayingLines(s, art, "", false, 56, 0)
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (matches art row count)", len(lines))
	}
	for i, l := range lines {
		if strings.Contains(l, "\n") {
			t.Fatalf("line %d contains embedded newline, would break boxRow: %q", i, l)
		}
	}
	if !strings.Contains(lines[0], "AAAA") {
		t.Fatalf("first line missing art content: %q", lines[0])
	}
}

func TestNowPlayingLinesWithArtColumnWidth(t *testing.T) {
	// A 4-char art string (the previous test's fixture) is narrower than
	// the progress bar's own minimum width, so it couldn't catch the bug
	// where progressLine sized its bar off the full widget width instead
	// of the narrower text column beside a realistically-sized art block —
	// the bar overflowed boxRow's inner width and silently truncated the
	// trailing timestamp. Use a realistic artCols-wide art block instead.
	s := spotifyapi.PlaybackState{
		Item:       spotifyapi.Track{Name: "Song", DurationMs: 200000},
		ProgressMs: 100000,
	}
	artLine := strings.Repeat("█", artCols)
	art := strings.Join([]string{artLine, artLine, artLine, artLine, artLine, artLine}, "\n")
	width := 56
	lines := nowPlayingLines(s, art, "", false, width, 0)
	for _, l := range lines {
		if w := lipgloss.Width(l); w > width-4 {
			t.Fatalf("line exceeds boxRow inner width (%d): width=%d line=%q", width-4, w, l)
		}
	}
	// The progress line (index 1) must still show the full "m:ss/m:ss"
	// timestamp, not have it truncated off the end.
	if !strings.Contains(lines[1], "1:40/3:20") {
		t.Fatalf("progress line missing timestamp, likely truncated: %q", lines[1])
	}
}

func TestPingpong(t *testing.T) {
	cases := []struct {
		tick, span, want int
	}{
		{0, 0, 0}, // no overflow: always 0
		{5, 0, 0},
		{0, 4, 0}, // start at 0
		{4, 4, 4}, // reaches the far end...
		{5, 4, 3}, // ...then bounces back
		{8, 4, 0}, // back to start
		{9, 4, 1}, // and forward again
	}
	for _, tc := range cases {
		if got := pingpong(tc.tick, tc.span); got != tc.want {
			t.Errorf("pingpong(%d, %d) = %d, want %d", tc.tick, tc.span, got, tc.want)
		}
	}
}

func TestWindowByWidth(t *testing.T) {
	cases := []struct {
		name            string
		s               string
		startCol, width int
		want            string
	}{
		{"ascii from start", "hello world", 0, 5, "hello"},
		{"ascii mid-window", "hello world", 6, 5, "world"},
		{"width longer than remaining", "hello", 2, 10, "llo"},
		{"CJK not split mid-glyph", "가나다라마", 0, 4, "가나"}, // each Hangul syllable is 2 cells wide
		{"CJK offset window", "가나다라마", 4, 4, "다라"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := windowByWidth(tc.s, tc.startCol, tc.width); got != tc.want {
				t.Errorf("windowByWidth(%q, %d, %d) = %q, want %q", tc.s, tc.startCol, tc.width, got, tc.want)
			}
		})
	}
}

func TestBoxRowNeverExceedsWidth(t *testing.T) {
	long := strings.Repeat("x", 200)
	row := boxRow(long, 40, false)
	if w := lipgloss.Width(row); w != 40 {
		t.Fatalf("boxRow width = %d, want 40 (long content should be truncated to fit)", w)
	}
}

func TestBoxRowPadsShortContent(t *testing.T) {
	row := boxRow("hi", 40, true)
	if w := lipgloss.Width(row); w != 40 {
		t.Fatalf("boxRow width = %d, want 40 (short content should be padded)", w)
	}
}
