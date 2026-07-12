package ui

import (
	"strings"
	"time"
)

func (m Model) View() string {
	var b strings.Builder

	width := m.width
	if width <= 0 || width > 90 {
		width = defaultWidgetWidth
	}

	switch m.screen {
	case screenSearch:
		b.WriteString(renderSearchScreen(m, width))
	case screenDevices:
		b.WriteString(renderDevicesScreen(m, width))
	default:
		b.WriteString(renderWidget(interpolatedState(m.state, m.lastRefresh, time.Now()), m.artRendered, m.cfg.ExperimentalKittyArt, width, m.marqueeTick))
		b.WriteString("\n")
		b.WriteString(renderPlaylistsBox(m.playlists, width))
		if m.playlistTracksTitle != "" {
			b.WriteString("\n")
			b.WriteString(renderPlaylistTracksBox(m, width))
		}
	}
	b.WriteString("\n")

	if m.lastErr != nil {
		b.WriteString(errorStyle.Render("⚠ " + m.lastErr.Error()))
		b.WriteString("\n")
	}

	b.WriteString(helpLine(m.screen, m.focusTracks))
	return b.String()
}
