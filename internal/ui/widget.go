package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

const defaultWidgetWidth = 56

// renderWidget draws the now-playing box.
//
// Two distinct art rendering paths, and they are NOT interchangeable:
//
//   - Halfblock art (artIsGraphics=false) is just ANSI-colored unicode
//     text. It's safe to lay out side-by-side with the track/progress/status
//     text via lipgloss.JoinHorizontal, split into lines, and feed each line
//     through boxRow — boxRow's ansi.Truncate trims overflow from the end of
//     the line (the text side), so the art column is never at risk of being
//     cut mid-sequence.
//   - Graphics-protocol art (artIsGraphics=true, e.g. Kitty via
//     --experimental-kitty-art) is a single escape sequence carrying binary
//     image data, positioned at the cursor location where it's emitted. It
//     is NOT line-oriented text — splitting it on '\n' and re-wrapping
//     fragments through boxRow's padding/truncation corrupts it (confirmed:
//     produced overlapping/duplicated UI). go-termimg's own docs mark the
//     "virtual placement" feature that would make this safe to interleave
//     with text as not production-ready, so it isn't used here. Instead the
//     raw art string is emitted once, completely untouched, as its own
//     paragraph before the boxed widget — the widget itself renders without
//     an art column in this mode.
func renderWidget(state *spotifyapi.PlaybackState, art string, artIsGraphics bool, width int) string {
	if width <= 0 {
		width = defaultWidgetWidth
	}

	var b strings.Builder

	if artIsGraphics && art != "" {
		// The Kitty escape sequence itself contains no printable newlines,
		// so from bubbletea's line-diffing renderer's perspective this is
		// "one line" — but the terminal actually advances the cursor down
		// by the image's real height (artRows) once it processes the
		// placement. That mismatch between bubbletea's line count and the
		// terminal's real cursor position is what caused the box below to
		// render at the wrong offset (confirmed via screenshot: duplicated/
		// misaligned boxes). Padding with artRows newlines here keeps
		// bubbletea's internal accounting in sync with reality.
		b.WriteString(art)
		// Calibrated empirically against a real terminal (the only way to
		// verify this — no way to measure it from here): artRows newlines
		// left too large a gap, suggesting termimg's own Render() output
		// already accounts for some of the vertical advance itself. Halving
		// it as the next data point.
		b.WriteString(strings.Repeat("\n", artRows/2))
	}

	trailing := "idle"
	if state != nil && state.IsPlaying {
		trailing = "playing"
	}
	b.WriteString(boxTop("Spotify", trailing, width))
	b.WriteString("\n")

	if state == nil {
		b.WriteString(boxRow(dimStyle.Render("nothing playing"), width))
		b.WriteString("\n")
	} else {
		inlineArt := art
		if artIsGraphics {
			inlineArt = "" // already emitted above, unsplit
		}
		for _, line := range nowPlayingLines(*state, inlineArt, width) {
			b.WriteString(boxRow(line, width))
			b.WriteString("\n")
		}
	}

	b.WriteString(boxBottom(width))
	return b.String()
}

func nowPlayingLines(s spotifyapi.PlaybackState, art string, width int) []string {
	// progressLine sizes its bar off the width it's given — when art sits
	// beside the text (JoinHorizontal), the text column is narrower than
	// the full widget width by the art column plus its spacer, and passing
	// the full width here made the bar overflow boxRow's inner width,
	// silently truncating the timestamp off the end.
	textWidth := width
	if art != "" {
		textWidth = width - lipgloss.Width(strings.SplitN(art, "\n", 2)[0]) - 2
		if textWidth < 20 {
			textWidth = 20
		}
	}
	text := strings.Join([]string{trackLine(s), progressLine(s, textWidth), statusLine(s), ""}, "\n")
	if art == "" {
		return strings.Split(text, "\n")[:3]
	}
	joined := lipgloss.JoinHorizontal(lipgloss.Top, art, "  ", text)
	return strings.Split(joined, "\n")
}

func trackLine(s spotifyapi.PlaybackState) string {
	icon := pauseStyle.Render("⏸")
	if s.IsPlaying {
		icon = playStyle.Render("▶")
	}
	if s.Item.Name == "" {
		return icon + "  " + dimStyle.Render("(no track)")
	}
	return icon + "  " + titleTextStyle.Render(s.Item.Name) + metaStyle.Render(" — "+strings.Join(s.Item.Artists, ", "))
}

func progressLine(s spotifyapi.PlaybackState, width int) string {
	barWidth := width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	filled := 0
	if s.Item.DurationMs > 0 {
		filled = barWidth * s.ProgressMs / s.Item.DurationMs
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := barFillStyle.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", barWidth-filled))
	return fmt.Sprintf("%s %s/%s", bar, formatMs(s.ProgressMs), formatMs(s.Item.DurationMs))
}

func statusLine(s spotifyapi.PlaybackState) string {
	parts := []string{
		fmt.Sprintf("Vol %d%%", s.Device.VolumePercent),
	}
	if s.ShuffleState {
		parts = append(parts, "Shuffle on")
	}
	if s.RepeatState != "" && s.RepeatState != "off" {
		parts = append(parts, "Repeat "+s.RepeatState)
	}
	if s.Device.Name != "" {
		parts = append(parts, s.Device.Name)
	}
	return metaStyle.Render(strings.Join(parts, "  ·  "))
}

func formatMs(ms int) string {
	total := ms / 1000
	m := total / 60
	sec := total % 60
	return fmt.Sprintf("%d:%02d", m, sec)
}
