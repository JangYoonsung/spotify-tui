package ui

import (
	"fmt"
	"strings"
)

// renderPlaylistsBox draws the always-visible playlists list under the
// now-playing box (screenNowPlaying) — no key needed to reveal it.
func renderPlaylistsBox(l listState, width int) string {
	var b strings.Builder
	b.WriteString(boxTop("Playlists", listTrailing(l), width))
	b.WriteString("\n")
	for _, line := range renderListRows(l) {
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
	for _, line := range renderListRows(m.search) {
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
	for _, line := range renderListRows(m.playlistTracks) {
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

func renderListRows(l listState) []string {
	switch {
	case l.loading:
		return []string{dimStyle.Render("⠋ loading…")}
	case l.err != nil:
		return []string{errorStyle.Render("⚠ " + l.err.Error())}
	case len(l.items) == 0:
		return []string{dimStyle.Render("· no results")}
	}

	end := min(l.scrollTop+listVisibleRows, len(l.items))
	lines := make([]string, 0, end-l.scrollTop)
	for i := l.scrollTop; i < end; i++ {
		if i == l.cursor {
			lines = append(lines, accentStyle.Render("▸ ")+titleTextStyle.Render(l.items[i].label))
			continue
		}
		lines = append(lines, "  "+metaStyle.Render(l.items[i].label))
	}
	return lines
}
