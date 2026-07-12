package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type screen int

const (
	// screenNowPlaying is the only "home" screen: now-playing box, an
	// always-visible playlists list, and (once a playlist is picked) that
	// playlist's tracks — all stacked on the same screen, no separate
	// drill-in screen. Up/down/enter route to whichever list currently has
	// focus (see Model.focusTracks).
	screenNowPlaying screen = iota
	screenSearch
	// screenDevices lists Spotify Connect devices; enter transfers playback
	// there via PlayWithDeviceQuery (the confirmed-working targeting slot —
	// see the diagnostics notes in spotifyapi/playback.go).
	screenDevices
)

// listItem is a pre-rendered, selectable row. Playlist rows only carry id
// (used to fetch that playlist's tracks — selecting a playlist always
// drills into its track list, never plays the whole playlist directly, so
// there's no "spotify:playlist:<id>" context URI to keep here). Track rows
// carry trackURI (used to play that specific track) and duration, kept
// separate from label rather than concatenated so it can be right-aligned
// to a fixed column instead of trailing wherever the label happens to end.
type listItem struct {
	label    string
	duration string // "" for playlist rows (no per-row duration to show)
	id       string
	trackURI string
}

// FilterValue feeds bubbles/list's fuzzy filtering (bound to "f" here —
// "/" stays the global Spotify search screen).
func (i listItem) FilterValue() string { return i.label }

// listState wraps bubbles/list with the loading/error fetch lifecycle it
// doesn't model itself. The zero value is NOT usable — always construct via
// newListState/loadingListState (list.Model needs its delegate/keymap setup).
type listState struct {
	list    list.Model
	loading bool
	err     error
}

const listVisibleRows = 12

func newListState() listState {
	l := list.New(nil, compactDelegate{}, defaultWidgetWidth-4, listVisibleRows)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.FilterInput.Prompt = "filter: "
	l.FilterInput.PromptStyle = dimStyle
	// "/" (bubbles' default) is taken by the global Spotify search screen.
	l.KeyMap.Filter = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter"))
	return listState{list: l}
}

func loadingListState() listState {
	s := newListState()
	s.loading = true
	return s
}

// setItems installs items and a delegate sized to the widest duration
// present, resetting any leftover filter from the previous item set.
func (l *listState) setItems(items []list.Item) {
	durationCol := 0
	for _, it := range items {
		if li, ok := it.(listItem); ok {
			if w := lipgloss.Width(li.duration); w > durationCol {
				durationCol = w
			}
		}
	}
	l.list.SetDelegate(compactDelegate{durationCol: durationCol})
	l.list.ResetFilter()
	l.list.SetItems(items)
	l.list.Select(0)
}

// selectID moves the cursor to the item with the given id, if present.
func (l *listState) selectID(id string) {
	if id == "" {
		return
	}
	for i, it := range l.list.Items() {
		if li, ok := it.(listItem); ok && li.id == id {
			l.list.Select(i)
			return
		}
	}
}

func (l *listState) selected() (listItem, bool) {
	it, ok := l.list.SelectedItem().(listItem)
	return it, ok
}

// listNavMatches reports whether msg is one of the list's own navigation/
// filtering keys — those are delegated to bubbles/list, everything else
// falls through to this app's handlers (e.g. playback controls on the home
// screen).
func listNavMatches(l list.Model, msg tea.KeyMsg) bool {
	k := l.KeyMap
	return key.Matches(msg, k.CursorUp, k.CursorDown, k.NextPage, k.PrevPage,
		k.GoToStart, k.GoToEnd, k.Filter, k.ClearFilter)
}

// compactDelegate renders one-line rows in this app's box style (accent ▸
// cursor, duration right-aligned to a fixed column) — list.DefaultDelegate's
// two-line title/description items are too tall for a docked widget.
type compactDelegate struct {
	durationCol int
}

func (d compactDelegate) Height() int                               { return 1 }
func (d compactDelegate) Spacing() int                              { return 0 }
func (d compactDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(listItem)
	if !ok {
		return
	}
	labelWidth := m.Width() - 2 // "▸ "/"  " prefix
	if d.durationCol > 0 {
		labelWidth -= d.durationCol + 1 // space before the duration column
	}
	if labelWidth < 4 {
		labelWidth = 4
	}
	label := ansi.Truncate(it.label, labelWidth, "…")
	label += strings.Repeat(" ", labelWidth-lipgloss.Width(label))

	prefix, labelStyle := "  ", metaStyle
	if index == m.Index() {
		prefix, labelStyle = accentStyle.Render("▸ "), titleTextStyle
	}
	line := prefix + labelStyle.Render(label)
	if d.durationCol > 0 {
		dur := strings.Repeat(" ", d.durationCol-lipgloss.Width(it.duration)) + it.duration
		line += " " + metaStyle.Render(dur)
	}
	fmt.Fprint(w, line)
}
