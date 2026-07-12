package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Monochromatic palette: one hue family (green) at three lightness steps
// carries every "active/emphasis" signal, rather than either a single flat
// accent or scattered unrelated hues (both tried and rejected earlier —
// see git history). Grayscale carries plain text hierarchy (title > meta >
// dim), and red is reserved for errors, the one deliberate non-monotone
// exception since errors need to interrupt, not blend in.
//
//	accentBright (120) — the one thing happening right now: play icon,
//	                      progress fill, selected row's cursor + label
//	accentMid    ( 78) — one step down: now-playing box's border, marking
//	                      it as the primary box among the stack
var (
	accentBright = lipgloss.Color("120")
	accentMid    = lipgloss.Color("78")

	titleTextStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")) // track name, selected row label
	metaStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))            // artist/album, secondary info
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))            // least-important text
	footerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// Three border weights give the stack of boxes a hierarchy: the
	// now-playing box (the one thing actually happening) reads as primary,
	// the keyboard-focused list box brightens a step so it's obvious where
	// up/down/enter will land, and unfocused boxes recede.
	borderStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("238")) // unfocused secondary boxes
	borderStyleFocused = lipgloss.NewStyle().Foreground(lipgloss.Color("246")) // the box keyboard input targets
	borderStylePrimary = lipgloss.NewStyle().Foreground(accentMid)             // now-playing box

	// Box-title typography: the focused box's title pops, unfocused titles
	// recede with their border, the primary box's title carries the accent.
	boxTitleFocused   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	boxTitleUnfocused = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	boxTitlePrimary   = lipgloss.NewStyle().Bold(true).Foreground(accentBright)

	// The cursor row gets a subtle background so the selection reads as a
	// bar, not just a brighter label; the ♪ marker tags the row that's
	// actually playing, independent of where the cursor is.
	cursorRowStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("236"))
	cursorRowMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(lipgloss.Color("236"))
	nowPlayingRowStyle = lipgloss.NewStyle().Foreground(accentMid)

	accentStyle  = lipgloss.NewStyle().Bold(true).Foreground(accentBright)
	playStyle    = accentStyle
	pauseStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	barFillStyle = lipgloss.NewStyle().Foreground(accentBright)
)

// ---- box-drawing helpers ----
//
// Same hand-rolled approach as cmux-orchestrator's internal/ui/styles.go:
// row content mixes lipgloss-colored spans with plain text, and padding
// must be computed on *visible* width via lipgloss.Width/ansi.Truncate —
// bubbles/table measured raw string length instead and mis-truncated
// colored cells, so we don't use it here either.

const (
	boxTL, boxTR = "╭", "╮"
	boxBL, boxBR = "╰", "╯"
	boxH, boxV   = "─", "│"
)

func boxTop(title, trailing string, width int, focused bool) string {
	if focused {
		return boxTopStyled(title, trailing, width, borderStyleFocused, boxTitleFocused)
	}
	return boxTopStyled(title, trailing, width, borderStyle, boxTitleUnfocused)
}

func boxBottom(width int, focused bool) string {
	if focused {
		return boxBottomStyled(width, borderStyleFocused)
	}
	return boxBottomStyled(width, borderStyle)
}

func boxRow(content string, width int, focused bool) string {
	if focused {
		return boxRowStyled(content, width, borderStyleFocused)
	}
	return boxRowStyled(content, width, borderStyle)
}

func boxTopPrimary(title, trailing string, width int) string {
	return boxTopStyled(title, trailing, width, borderStylePrimary, boxTitlePrimary)
}
func boxBottomPrimary(width int) string { return boxBottomStyled(width, borderStylePrimary) }
func boxRowPrimary(content string, width int) string {
	return boxRowStyled(content, width, borderStylePrimary)
}

func boxTopStyled(title, trailing string, width int, border, titleStyle lipgloss.Style) string {
	// Same plain-width arithmetic as before, but the title and trailing
	// counter get their own styles instead of inheriting the border color —
	// segment widths must add up to exactly `width` (see the box tests).
	fill := width - 8 - lipgloss.Width(title) - lipgloss.Width(trailing)
	if fill < 0 {
		fill = 0
	}
	return border.Render(boxTL+boxH+" ") + titleStyle.Render(title) +
		border.Render(" "+strings.Repeat(boxH, fill)+" ") + dimStyle.Render(trailing) +
		border.Render(" "+boxH+boxTR)
}

func boxBottomStyled(width int, style lipgloss.Style) string {
	return style.Render(boxBL + strings.Repeat(boxH, width-2) + boxBR)
}

func boxRowStyled(content string, width int, style lipgloss.Style) string {
	inner := width - 4 // "│ " + content + " │"
	visible := lipgloss.Width(content)
	if visible > inner {
		content = ansi.Truncate(content, inner, "…")
		visible = lipgloss.Width(content)
	}
	pad := inner - visible
	if pad < 0 {
		pad = 0
	}
	return style.Render(boxV) + " " + content + strings.Repeat(" ", pad) + " " + style.Render(boxV)
}
