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

	out := renderPlaylistsBox(l, width, "x")
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
