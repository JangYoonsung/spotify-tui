package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Restrained palette: a single violet accent (141) carries every
// "active/selected/emphasis" signal (play icon, progress fill, list cursor,
// selected label), grayscale carries structure/hierarchy (title text >
// border > meta > footer), and red is reserved for errors only. One
// consistent accent reads as designed; scattering several hues (the
// earlier green/red mix) or flattening everything to one gray (the
// over-corrected monotone pass) both read as less intentional than this.
var (
	titleTextStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")) // track name, selected row label
	metaStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))            // artist/album, secondary info
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))            // least-important text
	borderStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))            // box lines
	footerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	accentStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")) // the one accent hue
	playStyle    = accentStyle
	pauseStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	barFillStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
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
	inner := width - 2
	left := boxH + " " + title + " "
	right := " " + trailing + " " + boxH
	fill := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if fill < 0 {
		fill = 0
	}
	return borderStyle.Render(boxTL+left) + strings.Repeat(boxH, fill) + borderStyle.Render(right+boxTR)
}

func boxBottom(width int) string {
	return borderStyle.Render(boxBL + strings.Repeat(boxH, width-2) + boxBR)
}

func boxRow(content string, width int) string {
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
	return borderStyle.Render(boxV) + " " + content + strings.Repeat(" ", pad) + " " + borderStyle.Render(boxV)
}
