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
	// screenQueue shows the actual upcoming play queue (GET /me/player/queue)
	// — distinct from a playlist's track list. Enter plays the selected
	// entry directly (single-URI play, same as picking a search result).
	screenQueue
	// screenRecent shows listening history (GET /me/player/recently-played,
	// duplicates collapsed to the most recent occurrence).
	screenRecent
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
	// delegate inputs, kept so either can change without recomputing the
	// other (SetDelegate replaces the whole value).
	durationCol int
	nowPlaying  string
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
	l.durationCol = 0
	for _, it := range items {
		if li, ok := it.(listItem); ok {
			if w := lipgloss.Width(li.duration); w > l.durationCol {
				l.durationCol = w
			}
		}
	}
	l.refreshDelegate()
	l.list.ResetFilter()
	l.list.SetItems(items)
	l.list.Select(0)
}

// setNowPlaying tags the row that's actually playing (♪ marker) — called on
// every track change, so it must not disturb items/cursor/filter.
func (l *listState) setNowPlaying(trackID string) {
	if l.nowPlaying == trackID {
		return
	}
	l.nowPlaying = trackID
	l.refreshDelegate()
}

func (l *listState) refreshDelegate() {
	l.list.SetDelegate(compactDelegate{durationCol: l.durationCol, nowPlayingID: l.nowPlaying})
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

// compactDelegate renders one-line rows in this app's box style (cursor row
// as a background bar, ♪ on the actually-playing row, duration right-aligned
// to a fixed column) — list.DefaultDelegate's two-line title/description
// items are too tall for a docked widget.
type compactDelegate struct {
	durationCol  int
	nowPlayingID string
}

func (d compactDelegate) Height() int                               { return 1 }
func (d compactDelegate) Spacing() int                              { return 0 }
func (d compactDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(listItem)
	if !ok {
		return
	}
	labelWidth := m.Width() - 2 // "▸ "/"♪ "/"  " prefix
	if d.durationCol > 0 {
		labelWidth -= d.durationCol + 1 // space before the duration column
	}
	if labelWidth < 4 {
		labelWidth = 4
	}
	label := ansi.Truncate(it.label, labelWidth, "…")
	label += strings.Repeat(" ", labelWidth-lipgloss.Width(label))

	cursor := index == m.Index()
	playing := d.nowPlayingID != "" && it.id == d.nowPlayingID

	// Cursor wins the prefix slot; a playing row that isn't under the
	// cursor shows ♪ instead of the padding.
	prefix := "  "
	switch {
	case cursor:
		prefix = accentStyle.Render("▸ ")
	case playing:
		prefix = accentStyle.Render("♪ ")
	}

	labelStyle, durStyle := metaStyle, metaStyle
	switch {
	case cursor:
		// Background bar across label and duration so the selection reads
		// as one row, not disconnected bright fragments.
		labelStyle, durStyle = cursorRowStyle, cursorRowMetaStyle
	case playing:
		labelStyle = nowPlayingRowStyle
	}

	line := prefix + labelStyle.Render(label)
	if d.durationCol > 0 {
		dur := strings.Repeat(" ", d.durationCol-lipgloss.Width(it.duration)) + it.duration
		line += durStyle.Render(" " + dur)
	}
	_, _ = fmt.Fprint(w, line)
}
