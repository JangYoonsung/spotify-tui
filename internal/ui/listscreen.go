package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// renderPlaylistsBox draws the always-visible playlists list under the
// now-playing box (screenNowPlaying) — no key needed to reveal it.
func renderPlaylistsBox(l listState, width int) string {
	var b strings.Builder
	b.WriteString(boxTop("Playlists", listTrailing(l), width))
	b.WriteString("\n")
	for _, line := range renderListRows(l, width) {
		b.WriteString(boxRow(line, width))
		b.WriteString("\n")
	}
	b.WriteString(boxBottom(width))
	return b.String()
}

func renderSearchScreen(m Model, width int) string {
	var b strings.Builder
	b.WriteString(boxTop("Search", listTrailing(m.search), width))
	b.WriteString("\n")

	if m.searchInput.Focused() || m.searchInput.Value() == "" {
		b.WriteString(boxRow(m.searchInput.View(), width))
		b.WriteString("\n")
	}
	for _, line := range renderListRows(m.search, width) {
		b.WriteString(boxRow(line, width))
		b.WriteString("\n")
	}

	b.WriteString(boxBottom(width))
	return b.String()
}

// renderPlaylistTracksBox draws the selected playlist's tracks — inline on
// the main screen underneath the playlists box, not a separate screen.
// Only rendered once the user has picked a playlist at least once
// (Model.playlistTracksTitle is set at that point and never cleared).
func renderPlaylistTracksBox(m Model, width int) string {
	title := m.playlistTracksTitle
	if title == "" {
		title = "Tracks"
	}
	var b strings.Builder
	b.WriteString(boxTop(title, listTrailing(m.playlistTracks), width))
	b.WriteString("\n")
	for _, line := range renderListRows(m.playlistTracks, width) {
		b.WriteString(boxRow(line, width))
		b.WriteString("\n")
	}
	b.WriteString(boxBottom(width))
	return b.String()
}

func listTrailing(l listState) string {
	if len(l.items) == 0 {
		return "0"
	}
	return fmt.Sprintf("%d/%d", l.cursor+1, len(l.items))
}

func renderListRows(l listState, width int) []string {
	switch {
	case l.loading:
		return []string{dimStyle.Render("⠋ loading…")}
	case l.err != nil:
		return []string{errorStyle.Render("⚠ " + l.err.Error())}
	case len(l.items) == 0:
		return []string{dimStyle.Render("· no results")}
	}

	// Right-align duration to a fixed column (rather than trailing wherever
	// each row's label happens to end) so the whole list reads as a table,
	// not ragged text — measured from the widest duration actually present
	// so playlist rows (no duration at all) don't reserve dead space.
	durationCol := 0
	for _, it := range l.items {
		if w := lipgloss.Width(it.duration); w > durationCol {
			durationCol = w
		}
	}
	labelWidth := width - 4 - 2 // boxRow's border/padding, then "▸ "/"  " prefix
	if durationCol > 0 {
		labelWidth -= durationCol + 1 // space before the duration column
	}
	if labelWidth < 4 {
		labelWidth = 4
	}

	end := min(l.scrollTop+listVisibleRows, len(l.items))
	lines := make([]string, 0, end-l.scrollTop)
	for i := l.scrollTop; i < end; i++ {
		item := l.items[i]
		label := ansi.Truncate(item.label, labelWidth, "…")
		label += strings.Repeat(" ", labelWidth-lipgloss.Width(label))

		prefix, labelStyle := "  ", metaStyle
		if i == l.cursor {
			prefix, labelStyle = accentStyle.Render("▸ "), titleTextStyle
		}
		line := prefix + labelStyle.Render(label)
		if durationCol > 0 {
			dur := strings.Repeat(" ", durationCol-lipgloss.Width(item.duration)) + item.duration
			line += " " + metaStyle.Render(dur)
		}
		lines = append(lines, line)
	}
	return lines
}
