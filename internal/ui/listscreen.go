package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// renderPlaylistsBox draws the always-visible playlists list under the
// now-playing box (screenNowPlaying) — no key needed to reveal it.
func renderPlaylistsBox(l listState, width int, spin string, focused bool) string {
	var b strings.Builder
	b.WriteString(boxTop("Playlists", listTrailing(l), width, focused))
	b.WriteString("\n")
	for _, line := range renderListRows(l, width, spin) {
		b.WriteString(boxRow(line, width, focused))
		b.WriteString("\n")
	}
	b.WriteString(boxBottom(width, focused))
	return b.String()
}

func renderSearchScreen(m Model, width int) string {
	var b strings.Builder
	b.WriteString(boxTop("Search", listTrailing(m.search), width, true))
	b.WriteString("\n")

	if m.searchInput.Focused() || m.searchInput.Value() == "" {
		b.WriteString(boxRow(m.searchInput.View(), width, true))
		b.WriteString("\n")
	}
	for _, line := range renderListRows(m.search, width, m.spin.View()) {
		b.WriteString(boxRow(line, width, true))
		b.WriteString("\n")
	}

	b.WriteString(boxBottom(width, true))
	return b.String()
}

// renderDevicesScreen lists Spotify Connect devices for playback transfer.
func renderDevicesScreen(m Model, width int) string {
	var b strings.Builder
	b.WriteString(boxTop("Devices", listTrailing(m.devices), width, true))
	b.WriteString("\n")
	for _, line := range renderListRows(m.devices, width, m.spin.View()) {
		b.WriteString(boxRow(line, width, true))
		b.WriteString("\n")
	}
	b.WriteString(boxBottom(width, true))
	return b.String()
}

// renderQueueScreen shows the actual upcoming play queue — what plays next,
// regardless of which playlist it came from.
func renderQueueScreen(m Model, width int) string {
	var b strings.Builder
	title := "Up Next"
	if m.atPlaylistEnd() && len(m.radioTracks) > 0 {
		// The playlist ended; these are Spotify's radio picks, not the queue.
		title = "Up Next · radio"
	}
	b.WriteString(boxTop(title, listTrailing(m.queueList), width, true))
	b.WriteString("\n")
	for _, line := range renderListRows(m.queueList, width, m.spin.View()) {
		b.WriteString(boxRow(line, width, true))
		b.WriteString("\n")
	}
	b.WriteString(boxBottom(width, true))
	return b.String()
}

// renderRecentScreen shows listening history, most recent first.
func renderRecentScreen(m Model, width int) string {
	var b strings.Builder
	b.WriteString(boxTop("History", listTrailing(m.recentList), width, true))
	b.WriteString("\n")
	for _, line := range renderListRows(m.recentList, width, m.spin.View()) {
		b.WriteString(boxRow(line, width, true))
		b.WriteString("\n")
	}
	b.WriteString(boxBottom(width, true))
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
	b.WriteString(boxTop(title, listTrailing(m.playlistTracks), width, m.focusTracks))
	b.WriteString("\n")
	for _, line := range renderListRows(m.playlistTracks, width, m.spin.View()) {
		b.WriteString(boxRow(line, width, m.focusTracks))
		b.WriteString("\n")
	}
	b.WriteString(boxBottom(width, m.focusTracks))
	return b.String()
}

func listTrailing(l listState) string {
	total := len(l.list.VisibleItems())
	if total == 0 {
		return "0"
	}
	return fmt.Sprintf("%d/%d", l.list.Index()+1, total)
}

// renderListRows draws the fetch lifecycle states itself (bubbles/list has
// no loading/error concept), then defers to list.View() — which renders the
// filter input inline while filtering — and re-wraps its lines in boxRow.
func renderListRows(l listState, width int, spin string) []string {
	switch {
	case l.loading:
		return []string{spin + dimStyle.Render(" loading…")}
	case l.err != nil:
		return []string{errorStyle.Render("⚠ " + l.err.Error())}
	case len(l.list.Items()) == 0:
		return []string{dimStyle.Render("· no results")}
	}
	// list.View() pads its output to the full configured height (blank rows
	// below the items) and emits a leading blank line for the hidden title
	// bar. Boxed at face value, each list became a 14-line box regardless of
	// content — two of those overflow a small dock terminal and the frame
	// gets cropped into visual garbage. Trim blank edge lines so the box
	// hugs its content again; the filter input line, when present, is
	// non-blank and survives.
	lines := strings.Split(l.list.View(), "\n")
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(ansi.Strip(lines[start])) == "" {
		start++
	}
	for end > start && strings.TrimSpace(ansi.Strip(lines[end-1])) == "" {
		end--
	}
	if start == end {
		// Every line blank — e.g. a filter that matches nothing.
		return []string{dimStyle.Render("· no matches")}
	}
	return lines[start:end]
}
