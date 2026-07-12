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

	// Two border weights give the stack of boxes a hierarchy: the
	// now-playing box (the one thing actually happening) reads as primary,
	// the playlists/tracks/search boxes underneath it as secondary.
	borderStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("238")) // secondary boxes
	borderStylePrimary = lipgloss.NewStyle().Foreground(accentMid)             // now-playing box

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

func boxTop(title, trailing string, width int) string {
	return boxTopStyled(title, trailing, width, borderStyle)
}
func boxBottom(width int) string              { return boxBottomStyled(width, borderStyle) }
func boxRow(content string, width int) string { return boxRowStyled(content, width, borderStyle) }

func boxTopPrimary(title, trailing string, width int) string {
	return boxTopStyled(title, trailing, width, borderStylePrimary)
}
func boxBottomPrimary(width int) string { return boxBottomStyled(width, borderStylePrimary) }
func boxRowPrimary(content string, width int) string {
	return boxRowStyled(content, width, borderStylePrimary)
}

func boxTopStyled(title, trailing string, width int, style lipgloss.Style) string {
	inner := width - 2
	left := boxH + " " + title + " "
	right := " " + trailing + " " + boxH
	fill := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if fill < 0 {
		fill = 0
	}
	return style.Render(boxTL + left + strings.Repeat(boxH, fill) + right + boxTR)
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
