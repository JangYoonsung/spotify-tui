package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

// interpolatedState returns state with ProgressMs advanced by the time
// elapsed since lastRefresh, clamped to the track duration. The data poll
// only lands every PollInterval (3s default) but the marquee ticker redraws
// the view every 400ms — without this the progress bar visibly stalls
// between polls. Paused/unknown playback is returned untouched, and the
// original state is never mutated (the next poll re-syncs the real value).
func interpolatedState(state *spotifyapi.PlaybackState, lastRefresh, now time.Time) *spotifyapi.PlaybackState {
	if state == nil || !state.IsPlaying || lastRefresh.IsZero() {
		return state
	}
	s := *state
	s.ProgressMs += int(now.Sub(lastRefresh).Milliseconds())
	if s.Item.DurationMs > 0 && s.ProgressMs > s.Item.DurationMs {
		s.ProgressMs = s.Item.DurationMs
	}
	return &s
}

// defaultWidgetWidth is the fallback used only when bubbletea hasn't
// reported a real terminal width yet (no WindowSizeMsg received).
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
func renderWidget(state *spotifyapi.PlaybackState, art, nextTrack string, liked, artIsGraphics bool, width, marqueeTick int) string {
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
	b.WriteString(boxTopPrimary("Spotify", trailing, width))
	b.WriteString("\n")

	if state == nil {
		b.WriteString(boxRowPrimary(dimStyle.Render("nothing playing"), width))
		b.WriteString("\n")
	} else {
		inlineArt := art
		if artIsGraphics {
			inlineArt = "" // already emitted above, unsplit
		}
		for _, line := range nowPlayingLines(*state, inlineArt, nextTrack, liked, width, marqueeTick) {
			b.WriteString(boxRowPrimary(line, width))
			b.WriteString("\n")
		}
	}

	b.WriteString(boxBottomPrimary(width))
	return b.String()
}

func nowPlayingLines(s spotifyapi.PlaybackState, art, nextTrack string, liked bool, width, marqueeTick int) []string {
	// boxRow reserves 4 columns for its own "│ " + " │" border/padding —
	// content sized off the full box width (rather than that minus 4)
	// overflows boxRow's actual inner width and gets truncated with "…".
	// This applied even with no art (textWidth used to be the bare width),
	// not just the art-present case.
	textWidth := width - 4
	if art != "" {
		textWidth -= lipgloss.Width(strings.SplitN(art, "\n", 2)[0]) + 2 // art column + its spacer
	}
	if textWidth < 20 {
		textWidth = 20
	}
	textLines := []string{trackLine(s, liked, textWidth, marqueeTick), progressLine(s, textWidth), statusLine(s)}
	if nextTrack != "" {
		textLines = append(textLines, nextLine(nextTrack, textWidth))
	}
	if art == "" {
		return textLines
	}
	text := strings.Join(append(textLines, ""), "\n")
	joined := lipgloss.JoinHorizontal(lipgloss.Top, art, "  ", text)
	return strings.Split(joined, "\n")
}

// nextLine shows what plays after the current track (from the queue).
func nextLine(nextTrack string, width int) string {
	return dimStyle.Render(ansi.Truncate("↳ next  "+nextTrack, width, "…"))
}

func trackLine(s spotifyapi.PlaybackState, liked bool, width, marqueeTick int) string {
	icon := pauseStyle.Render("⏸")
	if s.IsPlaying {
		icon = playStyle.Render("▶")
	}
	if liked {
		icon += " " + accentStyle.Render("♥")
	}
	if s.Item.Name == "" {
		return icon + "  " + dimStyle.Render("(no track)")
	}

	full := s.Item.Name
	if len(s.Item.Artists) > 0 {
		full += " — " + strings.Join(s.Item.Artists, ", ")
	}

	avail := width - lipgloss.Width(icon) - 2 // "  " between icon and text
	if avail < 1 {
		avail = 1
	}
	if lipgloss.Width(full) <= avail {
		return icon + "  " + titleTextStyle.Render(full)
	}
	// Too long to fit: ping-pong scroll a width-avail window across the
	// full text, advancing one column per marqueeTick tick.
	windowed := windowByWidth(full, pingpong(marqueeTick, lipgloss.Width(full)-avail), avail)
	return icon + "  " + titleTextStyle.Render(windowed)
}

// pingpong bounces a position back and forth across [0, span] as tick
// advances — "move side to side" rather than a one-way loop-around.
func pingpong(tick, span int) int {
	if span <= 0 {
		return 0
	}
	period := span * 2
	p := tick % period
	if p > span {
		p = period - p
	}
	return p
}

// windowByWidth returns the substring of s starting at visual column
// startCol and spanning at most width visual columns — rune-based (not
// byte) and visual-width-aware (not rune-count-aware) so it doesn't split
// wide characters (e.g. Hangul/CJK track titles) mid-glyph.
func windowByWidth(s string, startCol, width int) string {
	runes := []rune(s)
	col, startIdx := 0, len(runes)
	for i, r := range runes {
		if col >= startCol {
			startIdx = i
			break
		}
		col += lipgloss.Width(string(r))
	}
	var b strings.Builder
	col = 0
	for _, r := range runes[startIdx:] {
		w := lipgloss.Width(string(r))
		if col+w > width {
			break
		}
		b.WriteRune(r)
		col += w
	}
	return b.String()
}

func progressLine(s spotifyapi.PlaybackState, width int) string {
	ts := formatMs(s.ProgressMs) + "/" + formatMs(s.Item.DurationMs)
	// Reserve exactly what the timestamp actually needs (plus the space
	// before it) instead of a flat guess — a fixed reservation either
	// wastes width the bar could use (guess too generous) or overflows
	// (guess too stingy); measuring the real string avoids both.
	barWidth := width - lipgloss.Width(ts) - 1
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
	return bar + " " + ts
}

func statusLine(s spotifyapi.PlaybackState) string {
	parts := []string{
		metaStyle.Render(volumeBar(s.Device.VolumePercent) + fmt.Sprintf(" %d%%", s.Device.VolumePercent)),
	}
	if s.ShuffleState {
		parts = append(parts, accentStyle.Render("⇄")+metaStyle.Render(" shuffle"))
	}
	switch s.RepeatState {
	case "track":
		parts = append(parts, accentStyle.Render("↻")+metaStyle.Render(" track"))
	case "context":
		parts = append(parts, accentStyle.Render("↻")+metaStyle.Render(" all"))
	}
	if s.Device.Name != "" {
		parts = append(parts, metaStyle.Render(s.Device.Name))
	}
	return strings.Join(parts, metaStyle.Render("  ·  "))
}

// volumeBar renders volume as a 5-segment bar rather than just a number —
// a quick-glance shape reads faster than parsing digits.
func volumeBar(pct int) string {
	const segments = 5
	filled := (pct*segments + 50) / 100
	if filled > segments {
		filled = segments
	}
	if filled < 0 {
		filled = 0
	}
	return accentStyle.Render(strings.Repeat("▮", filled)) + dimStyle.Render(strings.Repeat("▯", segments-filled))
}

func formatMs(ms int) string {
	total := ms / 1000
	m := total / 60
	sec := total % 60
	return fmt.Sprintf("%d:%02d", m, sec)
}
