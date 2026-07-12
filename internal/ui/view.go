package ui

import "strings"

func (m Model) View() string {
	var b strings.Builder

	width := m.width
	if width <= 0 || width > 90 {
		width = defaultWidgetWidth
	}

	switch m.screen {
	case screenSearch:
		b.WriteString(renderSearchScreen(m, width))
	default:
		b.WriteString(renderWidget(m.state, m.artRendered, m.cfg.ExperimentalKittyArt, width))
		b.WriteString("\n")
		b.WriteString(renderPlaylistsBox(m.playlists, width))
		if m.playlistTracksTitle != "" {
			b.WriteString("\n")
			b.WriteString(renderPlaylistTracksBox(m, width))
		}
	}
	b.WriteString("\n")

	if m.lastErr != nil {
		b.WriteString(errorStyle.Render("error: " + m.lastErr.Error()))
		b.WriteString("\n")
	}

	b.WriteString(helpLine(m.screen, m.focusTracks))
	return b.String()
}
