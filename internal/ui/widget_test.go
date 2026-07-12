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
	lines := nowPlayingLines(s, "", 56)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (no art -> track/progress/status only)", len(lines))
	}
	for _, l := range lines {
		if strings.Contains(l, "\n") {
			t.Fatalf("line contains embedded newline, would break boxRow: %q", l)
		}
	}
}

func TestNowPlayingLinesWithArt(t *testing.T) {
	s := spotifyapi.PlaybackState{
		Item:       spotifyapi.Track{Name: "Song", DurationMs: 100000},
		ProgressMs: 0,
	}
	art := strings.Join([]string{"AAAA", "BBBB", "CCCC", "DDDD"}, "\n")
	lines := nowPlayingLines(s, art, 56)
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

func TestBoxRowNeverExceedsWidth(t *testing.T) {
	long := strings.Repeat("x", 200)
	row := boxRow(long, 40)
	if w := lipgloss.Width(row); w != 40 {
		t.Fatalf("boxRow width = %d, want 40 (long content should be truncated to fit)", w)
	}
}

func TestBoxRowPadsShortContent(t *testing.T) {
	row := boxRow("hi", 40)
	if w := lipgloss.Width(row); w != 40 {
		t.Fatalf("boxRow width = %d, want 40 (short content should be padded)", w)
	}
}
