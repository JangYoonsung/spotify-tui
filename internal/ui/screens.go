package ui

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

type listState struct {
	items     []listItem
	cursor    int
	scrollTop int
	loading   bool
	err       error
}

const listVisibleRows = 12

func (l *listState) moveCursor(delta int) {
	if len(l.items) == 0 {
		return
	}
	l.cursor += delta
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
	if l.cursor < l.scrollTop {
		l.scrollTop = l.cursor
	}
	if l.cursor >= l.scrollTop+listVisibleRows {
		l.scrollTop = l.cursor - listVisibleRows + 1
	}
}

func (l *listState) selected() (listItem, bool) {
	if l.cursor < 0 || l.cursor >= len(l.items) {
		return listItem{}, false
	}
	return l.items[l.cursor], true
}
