package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// Regression: bubbles/list pads View() to its full configured height and
// emits a leading blank line for its hidden title bar — boxed verbatim, two
// 14-line boxes overflowed the dock terminal and the frame rendered as
// garbage. The box must hug its content and every line must be exactly the
// box width.
func TestListBoxHugsContentAndWidth(t *testing.T) {
	const width = 56
	l := newListState()
	l.list.SetSize(width-4, listVisibleRows)
	l.setItems([]list.Item{
		listItem{label: "hoge", id: "1"},
		listItem{label: "내 플레이리스트 #1", id: "2"}, // wide runes must not break padding
	})

	out := renderPlaylistsBox(l, width, "x", true)
	lines := strings.Split(out, "\n")
	if len(lines) != 4 { // top border + 2 items + bottom border
		t.Fatalf("box has %d lines, want 4 (must hug content, not pad to list height):\n%s", len(lines), out)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, w, width, line)
		}
	}
}

// The ♪ now-playing marker and the cursor's background bar must not break
// row widths (background styling changes the ANSI structure of the line).
func TestNowPlayingMarkerAndCursorBar(t *testing.T) {
	const width = 56
	l := newListState()
	l.list.SetSize(width-4, listVisibleRows)
	l.setItems([]list.Item{
		listItem{label: "one", id: "t1", duration: "3:20"},
		listItem{label: "two", id: "t2", duration: "4:05"},
	})
	l.setNowPlaying("t2") // cursor sits on t1, ♪ on t2

	out := renderPlaylistsBox(l, width, "x", true)
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, w, width, line)
		}
	}
	if !strings.Contains(out, "♪") {
		t.Fatalf("now-playing row missing ♪ marker:\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Fatalf("cursor row missing ▸ marker:\n%s", out)
	}
}
