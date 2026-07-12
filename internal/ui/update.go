package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jangyoonsung/spotify-tui-go/internal/albumart"
	"github.com/jangyoonsung/spotify-tui-go/internal/config"
	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

type tickMsg time.Time

type marqueeTickMsg time.Time

const marqueeInterval = 400 * time.Millisecond

func marqueeTickCmd() tea.Cmd {
	return tea.Tick(marqueeInterval, func(t time.Time) tea.Msg { return marqueeTickMsg(t) })
}

type refreshResultMsg struct {
	state *spotifyapi.PlaybackState
	err   error
}

type actionResultMsg struct {
	err error
}

type playlistsResultMsg struct {
	playlists []spotifyapi.Playlist
	err       error
}

type playlistTracksResultMsg struct {
	tracks []spotifyapi.Track
	err    error
}

type searchResultMsg struct {
	tracks []spotifyapi.Track
	err    error
}

type artResultMsg struct {
	trackID string
	art     string
	err     error
}

type devicesResultMsg struct {
	devices []spotifyapi.Device
	err     error
}

// queueResultMsg carries the upcoming queue, tagged with the track it was
// fetched for so a stale response can't label the wrong "next" (same
// pattern as artResultMsg).
type queueResultMsg struct {
	forTrackID string
	tracks     []spotifyapi.Track
	err        error
}

// queueViewResultMsg is the same fetch feeding the queue *screen* (u) —
// separate from queueResultMsg so browsing the queue can't clobber the
// widget's "next" label and vice versa.
type queueViewResultMsg struct {
	tracks []spotifyapi.Track
	err    error
}

type recentResultMsg struct {
	tracks []spotifyapi.Track
	err    error
}

// likedResultMsg reports whether the current track is in Liked Songs.
type likedResultMsg struct {
	forTrackID string
	liked      bool
	err        error
}

// likedPlaylistID is the virtual "playlist" id for the Liked Songs row at
// the top of the playlists box — routed to GET /me/tracks instead of a real
// playlist endpoint everywhere a playlist id is consumed.
const likedPlaylistID = "__liked__"

const likedPlaylistLabel = "♥ Liked Songs"

const (
	artCols = 12
	artRows = 6
)

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func refreshCmd(client *spotifyapi.Client) tea.Cmd {
	return func() tea.Msg {
		state, err := client.GetPlaybackState()
		return refreshResultMsg{state: state, err: err}
	}
}

// actionCmd wraps a control call so its result becomes an actionResultMsg,
// clearing actionInFlight and triggering an immediate re-refresh.
func actionCmd(fn func() error) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{err: fn()}
	}
}

func playlistsCmd(client *spotifyapi.Client) tea.Cmd {
	return func() tea.Msg {
		playlists, err := client.GetPlaylists(50)
		return playlistsResultMsg{playlists: playlists, err: err}
	}
}

func playlistTracksCmd(client *spotifyapi.Client, playlistID string) tea.Cmd {
	return func() tea.Msg {
		if playlistID == likedPlaylistID {
			tracks, err := client.GetSavedTracks()
			return playlistTracksResultMsg{tracks: tracks, err: err}
		}
		tracks, err := client.GetPlaylistTracks(playlistID)
		return playlistTracksResultMsg{tracks: tracks, err: err}
	}
}

func checkLikedCmd(client *spotifyapi.Client, trackID string) tea.Cmd {
	return func() tea.Msg {
		liked, err := client.CheckSavedTrack(trackID)
		return likedResultMsg{forTrackID: trackID, liked: liked, err: err}
	}
}

func searchCmd(client *spotifyapi.Client, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := client.SearchTracks(query, 20)
		return searchResultMsg{tracks: results.Tracks, err: err}
	}
}

func devicesCmd(client *spotifyapi.Client) tea.Cmd {
	return func() tea.Msg {
		devices, err := client.GetDevices()
		return devicesResultMsg{devices: devices, err: err}
	}
}

func queueCmd(client *spotifyapi.Client, forTrackID string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := client.GetQueue()
		return queueResultMsg{forTrackID: forTrackID, tracks: tracks, err: err}
	}
}

func queueViewCmd(client *spotifyapi.Client) tea.Cmd {
	return func() tea.Msg {
		tracks, err := client.GetQueue()
		return queueViewResultMsg{tracks: tracks, err: err}
	}
}

func recentCmd(client *spotifyapi.Client) tea.Cmd {
	return func() tea.Msg {
		tracks, err := client.GetRecentlyPlayed(50)
		return recentResultMsg{tracks: tracks, err: err}
	}
}

// dedupTracks collapses repeated track IDs to their first (most recent)
// occurrence — Spotify's recently-played reports every replay separately.
func dedupTracks(tracks []spotifyapi.Track) []spotifyapi.Track {
	seen := make(map[string]bool, len(tracks))
	out := make([]spotifyapi.Track, 0, len(tracks))
	for _, t := range tracks {
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		out = append(out, t)
	}
	return out
}

func artCmd(imageURL, trackID string, useKitty bool) tea.Cmd {
	return func() tea.Msg {
		art, err := albumart.Render(imageURL, artCols, artRows, useKitty)
		return artResultMsg{trackID: trackID, art: art, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w := m.width
		if w <= 0 || w > 90 {
			w = defaultWidgetWidth
		}
		for _, l := range []*listState{&m.playlists, &m.playlistTracks, &m.search, &m.devices} {
			l.list.SetSize(w-4, listVisibleRows) // boxRow reserves 4 cols of border/padding
		}
		return m, nil

	case tickMsg:
		var cmds []tea.Cmd
		if !m.actionInFlight {
			cmds = append(cmds, refreshCmd(m.client))
		}
		cmds = append(cmds, tickCmd(m.cfg.PollInterval))
		return m, tea.Batch(cmds...)

	case marqueeTickMsg:
		m.marqueeTick++
		return m, marqueeTickCmd()

	case spinner.TickMsg:
		if !m.anyListLoading() {
			// Nothing loading: drop the ticker instead of re-arming it. The
			// next fetch re-arms via m.spin.Tick alongside its command.
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case refreshResultMsg:
		m.lastRefresh = time.Now()
		m.lastErr = msg.err
		if msg.err != nil {
			return m, nil
		}
		if msg.state != nil && (m.state == nil || msg.state.Item.ID != m.state.Item.ID) {
			m.marqueeTick = 0
		}
		prev := m.state
		m.state = msg.state
		// Follow the playback context: if what's playing switched to a
		// playlist this box isn't already showing (started from the phone,
		// autoplay chain, another client…), load its tracks. Edge-triggered
		// on the context URI so the user's own browsing isn't overridden.
		if msg.state != nil && msg.state.ContextURI != m.lastContextURI {
			m.lastContextURI = msg.state.ContextURI
			if id, ok := strings.CutPrefix(msg.state.ContextURI, "spotify:playlist:"); ok && id != m.currentPlaylistID {
				m.currentPlaylistID = id
				m.playlistTracksTitle = m.playlistNameByID(id)
				m.playlistTracks = loadingListState()
				return m, tea.Batch(playlistTracksCmd(m.client, id), m.spin.Tick)
			}
		}
		// Playlist/queue ran out mid-listen: chain into similar tracks, like
		// the official clients' autoplay. Search-based because Spotify
		// removed GET /recommendations for development-mode apps (probed:
		// 404 with valid auth and an active device).
		if !m.actionInFlight && m.nextTrack == "" &&
			playbackEnded(prev, msg.state, 2*int(m.cfg.PollInterval.Milliseconds())) {
			m.actionInFlight = true
			return m, m.autoplaySimilarCmd(prev.Item)
		}
		if msg.state != nil && msg.state.Item.ID != "" && msg.state.Item.ID != m.artTrackID {
			// Track changed: clear the stale "next" label and refetch the
			// queue for the new track (not every poll — one extra call per
			// track change, not per 3s tick).
			m.nextTrack = ""
			cmds := []tea.Cmd{queueCmd(m.client, msg.state.Item.ID), checkLikedCmd(m.client, msg.state.Item.ID)}
			imageURL := albumart.PickImageURL(msg.state.Item.Images, artCols*8, artRows*2*8)
			if imageURL != "" {
				// Record artTrackID now, before the fetch resolves — the
				// fetch (HTTP + decode) can take up to the http.Client's
				// timeout, easily longer than one 3s poll interval. Leaving
				// artTrackID at the old value until artResultMsg arrives
				// meant every poll tick in between saw "track changed,
				// haven't fetched art yet" and fired a duplicate artCmd for
				// the same track.
				m.artTrackID = msg.state.Item.ID
				cmds = append(cmds, artCmd(imageURL, msg.state.Item.ID, m.cfg.ExperimentalKittyArt))
			} else {
				m.artTrackID, m.artRendered = msg.state.Item.ID, ""
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case artResultMsg:
		if m.state != nil && m.state.Item.ID == msg.trackID {
			if msg.err == nil {
				m.artRendered = msg.art
			} else {
				// Without this, a failed fetch for a new track left the
				// *previous* track's art rendered on screen — wrong album
				// cover displayed as if it were current.
				m.artRendered = ""
			}
		}
		return m, nil

	case actionResultMsg:
		m.actionInFlight = false
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		// Refetch the queue too: actions like queue-add change what's next
		// without changing the current track, so the track-change trigger
		// in refreshResultMsg never fires for them.
		cmds := []tea.Cmd{refreshCmd(m.client)}
		if m.state != nil && m.state.Item.ID != "" {
			cmds = append(cmds, queueCmd(m.client, m.state.Item.ID))
		}
		return m, tea.Batch(cmds...)

	case queueResultMsg:
		// Silent on error — "next" is a garnish, not worth an error banner.
		if msg.err == nil && m.state != nil && m.state.Item.ID == msg.forTrackID {
			m.nextTrack = ""
			// The queue endpoint lies in several confirmed ways (single-URI
			// playback pads it with the current track; a playlist's last
			// track reports a wrap-around to the first; librespot devices
			// report a queue that differs from what they actually play
			// next, verified by skipping). When the current track sits in
			// the playing context's track list (which the tracks box
			// follows), the LIST ORDER is the trustworthy "next" — the
			// queue is only a fallback for out-of-context playback
			// (autoplay chains, single tracks). Shuffle randomizes order,
			// so position means nothing there; repeat-one's real "next" is
			// itself.
			ctxID, hasCtx := strings.CutPrefix(m.state.ContextURI, "spotify:playlist:")
			contextOrdered := false
			// repeat=off only: with repeat=context the wrap to the first
			// track is real and the queue reports it correctly.
			if m.state.RepeatState == "off" && !m.state.ShuffleState && hasCtx && ctxID == m.currentPlaylistID {
				items := m.playlistTracks.list.Items()
				for i, it := range items {
					li, ok := it.(listItem)
					if !ok || li.id != msg.forTrackID {
						continue
					}
					contextOrdered = true
					if i+1 < len(items) {
						if nxt, ok := items[i+1].(listItem); ok {
							m.nextTrack = nxt.label
						}
					}
					// Last list entry: leave nextTrack empty — the queue's
					// wrap-around claim is fake, and empty is what lets the
					// autoplay seed fire below.
					break
				}
			}
			if !contextOrdered {
				for _, t := range msg.tracks {
					if t.ID != msg.forTrackID {
						m.nextTrack = trackLabel(t)
						break
					}
				}
				if m.nextTrack == "" && len(msg.tracks) > 0 && m.state.RepeatState == "track" {
					m.nextTrack = trackLabel(msg.tracks[0])
				}
			}
			// Queue genuinely empty while still playing: seed similar
			// tracks into the real queue now, so playback rolls straight
			// into them (and they show up as "next" / on the queue screen).
			// Once per track — a failed seed leaves the playbackEnded
			// backup path to catch the stop.
			if m.nextTrack == "" && m.state.IsPlaying &&
				m.state.RepeatState != "track" && m.autoplaySeededFor != m.state.Item.ID {
				m.autoplaySeededFor = m.state.Item.ID
				return m, m.seedAutoplayCmd(m.state.Item)
			}
		}
		return m, nil

	case autoplaySeedResultMsg:
		// Success: refetch the queue so the seeded tracks appear as "next".
		// Failure is silent — the playbackEnded backup still fires.
		if msg.err == nil && m.state != nil && m.state.Item.ID == msg.forTrackID {
			return m, queueCmd(m.client, msg.forTrackID)
		}
		return m, nil

	case playlistsResultMsg:
		m.playlists.loading = false
		m.playlists.err = msg.err
		if msg.err == nil {
			// R re-fetches playlists too (alongside playback state), so
			// this fires on every manual refresh, not just the initial
			// load. Resetting cursor/scrollTop unconditionally snapped the
			// selection back to the top every time — preserve position by
			// re-finding the previously-selected item's ID when it's still
			// present in the new list.
			prevID := ""
			if item, ok := m.playlists.selected(); ok {
				prevID = item.id
			}
			if prevID == "" {
				// Initial load: land the cursor on the playlist restored from
				// the previous run, if any.
				prevID = m.restorePlaylistID
			}
			items := append(
				[]list.Item{listItem{label: likedPlaylistLabel, id: likedPlaylistID}},
				playlistItems(msg.playlists)...,
			)
			m.playlists.setItems(items)
			m.playlists.selectID(prevID)
		}
		return m, nil

	case playlistTracksResultMsg:
		m.playlistTracks.loading = false
		m.playlistTracks.err = msg.err
		if msg.err == nil {
			m.playlistTracks.setItems(trackItems(msg.tracks))
			// One-shot: only the restart restore — playlists opened later
			// (or a re-fetch) start at the top as usual.
			m.playlistTracks.selectID(m.restoreTrackID)
			m.restoreTrackID = ""
		}
		return m, nil

	case searchResultMsg:
		m.search.loading = false
		m.search.err = msg.err
		if msg.err == nil {
			m.search.setItems(trackItems(msg.tracks))
		}
		return m, nil

	case devicesResultMsg:
		m.devices.loading = false
		m.devices.err = msg.err
		if msg.err == nil {
			m.devices.setItems(deviceItems(msg.devices))
		}
		return m, nil

	case queueViewResultMsg:
		m.queueList.loading = false
		m.queueList.err = msg.err
		if msg.err == nil {
			m.queueList.setItems(trackItems(msg.tracks))
		}
		return m, nil

	case recentResultMsg:
		m.recentList.loading = false
		m.recentList.err = msg.err
		if msg.err == nil {
			m.recentList.setItems(trackItems(dedupTracks(msg.tracks)))
		}
		return m, nil

	case likedResultMsg:
		// Silent on error — the heart is decoration, like the next label.
		if msg.err == nil && m.state != nil && m.state.Item.ID == msg.forTrackID {
			m.likedCurrent = msg.liked
			m.likedForID = msg.forTrackID
		}
		return m, nil

	case list.FilterMatchesMsg:
		// bubbles/list computes fuzzy-filter matches asynchronously: typing
		// returns a cmd whose FilterMatchesMsg must be fed back into the
		// list, or the visible items never actually narrow.
		al := m.activeList()
		var cmd tea.Cmd
		al.list, cmd = al.list.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) anyListLoading() bool {
	return m.playlists.loading || m.playlistTracks.loading || m.search.loading || m.devices.loading
}

// activeList returns whichever list keyboard input currently targets.
func (m *Model) activeList() *listState {
	switch m.screen {
	case screenSearch:
		return &m.search
	case screenDevices:
		return &m.devices
	case screenQueue:
		return &m.queueList
	case screenRecent:
		return &m.recentList
	default:
		if m.focusTracks {
			return &m.playlistTracks
		}
		return &m.playlists
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Search screen's textinput must see keystrokes before any global
	// binding — otherwise typing "q" while composing a query would quit
	// the app instead of being typed.
	if m.screen == screenSearch && m.searchInput.Focused() {
		return m.handleSearchTypingKey(msg)
	}
	// Same rule while typing into a list's fuzzy filter (bound to "f"):
	// bubbles/list owns every keystroke until the filter is accepted/canceled.
	if al := m.activeList(); al.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		al.list, cmd = al.list.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Escape) && m.activeList().list.FilterState() == list.FilterApplied:
		// First esc clears an applied filter; the next one backs out of the
		// screen/focus as usual.
		m.activeList().list.ResetFilter()
		return m, nil
	case key.Matches(msg, keys.Escape) && m.screen == screenNowPlaying && m.focusTracks:
		// Tracks box stays visible either way — Esc just hands keyboard
		// focus back to the playlists box, no screen change involved.
		m.focusTracks = false
		return m, nil
	case key.Matches(msg, keys.Escape) && m.screen != screenNowPlaying:
		m.screen = screenNowPlaying
		return m, nil
	case key.Matches(msg, keys.Search) && m.screen == screenNowPlaying:
		m.screen = screenSearch
		m.search = newListState()
		m.searchInput.Reset()
		return m, m.searchInput.Focus()
	case key.Matches(msg, keys.Devices) && m.screen == screenNowPlaying:
		m.screen = screenDevices
		m.devices = loadingListState()
		return m, tea.Batch(devicesCmd(m.client), m.spin.Tick)
	case key.Matches(msg, keys.Queue) && m.screen == screenNowPlaying:
		m.screen = screenQueue
		m.queueList = loadingListState()
		return m, tea.Batch(queueViewCmd(m.client), m.spin.Tick)
	case key.Matches(msg, keys.Recent) && m.screen == screenNowPlaying:
		m.screen = screenRecent
		m.recentList = loadingListState()
		return m, tea.Batch(recentCmd(m.client), m.spin.Tick)
	case key.Matches(msg, keys.Refresh):
		return m, tea.Batch(refreshCmd(m.client), playlistsCmd(m.client))
	case key.Matches(msg, keys.Help):
		m.helpView.ShowAll = !m.helpView.ShowAll
		return m, nil
	}

	switch m.screen {
	case screenSearch:
		return m.handleListKey(msg, m.playTrackSelection)
	case screenDevices:
		return m.handleListKey(msg, m.transferToDevice)
	case screenQueue, screenRecent:
		return m.handleListKey(msg, m.playTrackSelection)
	default:
		return m.handleNowPlayingKey(msg)
	}
}

// handleNowPlayingKey is the (only) home screen. Up/down/enter route to
// whichever of the playlists/tracks boxes has focus (see Model.focusTracks)
// — both boxes are always rendered, nothing is hidden behind a separate
// screen. Everything else is the v2 playback controls.
func (m Model) handleNowPlayingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focusTracks {
		switch {
		case listNavMatches(m.playlistTracks.list, msg):
			var cmd tea.Cmd
			m.playlistTracks.list, cmd = m.playlistTracks.list.Update(msg)
			return m, cmd
		case key.Matches(msg, keys.Enter):
			item, ok := m.playlistTracks.selected()
			if !ok {
				return m, nil
			}
			m.actionInFlight = true
			// Persist which track was played so the next launch puts the
			// tracks-box cursor back on it (best-effort, like the playlist).
			_ = config.SaveUIState(config.UIState{
				LastPlaylistID:   m.currentPlaylistID,
				LastPlaylistName: m.playlistTracksTitle,
				LastTrackID:      item.id,
			})
			return m, m.playPlaylistTrackSelection(item)
		case key.Matches(msg, keys.QueueAdd):
			item, ok := m.playlistTracks.selected()
			if !ok {
				return m, nil
			}
			m.actionInFlight = true
			return m, m.queueTrack(item)
		}
	} else {
		switch {
		case listNavMatches(m.playlists.list, msg):
			var cmd tea.Cmd
			m.playlists.list, cmd = m.playlists.list.Update(msg)
			return m, cmd
		case key.Matches(msg, keys.PlayPlaylist):
			item, ok := m.playlists.selected()
			if !ok {
				return m, nil
			}
			// P = play the whole playlist AND drill into its tracks box —
			// same open-and-focus behavior as enter, plus playback.
			m.actionInFlight = true
			m.focusTracks = true
			m.playlistTracks = loadingListState()
			m.playlistTracksTitle = item.label
			m.currentPlaylistID = item.id
			_ = config.SaveUIState(config.UIState{LastPlaylistID: item.id, LastPlaylistName: item.label})
			return m, tea.Batch(m.playContextSelection(item), playlistTracksCmd(m.client, item.id), m.spin.Tick)
		case key.Matches(msg, keys.Enter):
			item, ok := m.playlists.selected()
			if !ok {
				return m, nil
			}
			m.focusTracks = true
			m.playlistTracks = loadingListState()
			m.playlistTracksTitle = item.label
			m.currentPlaylistID = item.id
			// Best-effort persist so the tracks box survives a restart (the
			// widget lives in a cmux dock and restarts with it) — a failed
			// write only costs the convenience, not worth surfacing. No
			// LastTrackID: picking a playlist starts its tracks at the top.
			_ = config.SaveUIState(config.UIState{LastPlaylistID: item.id, LastPlaylistName: item.label})
			return m, tea.Batch(playlistTracksCmd(m.client, item.id), m.spin.Tick)
		}
	}

	if m.actionInFlight {
		return m, nil
	}
	return m.handleControlKey(msg)
}

// handleSearchTypingKey routes keystrokes to the textinput while composing
// a query; Enter submits the search (and blurs, switching to result
// browsing), Esc backs out to now-playing entirely.
func (m Model) handleSearchTypingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.screen = screenNowPlaying
		m.searchInput.Blur()
		return m, nil
	case key.Matches(msg, keys.Enter):
		query := m.searchInput.Value()
		m.searchInput.Blur()
		if query == "" {
			return m, nil
		}
		m.search.loading = true
		return m, tea.Batch(searchCmd(m.client, query), m.spin.Tick)
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// handleListKey drives selection for the Search/Devices/Queue screens:
// enter and queue-add are this app's actions, everything else (cursor
// movement, paging, "f" fuzzy filter) is delegated to bubbles/list.
//
// The list is resolved from THIS receiver via activeList — Model has value
// semantics, so accepting a *listState from the caller silently discards
// every list.Update: the pointer aims at the caller's copy while the method
// returns its own. That exact bug shipped once (queue/search/devices
// cursors frozen); see TestQueueScreenCursorMoves.
func (m Model) handleListKey(msg tea.KeyMsg, play func(listItem) tea.Cmd) (tea.Model, tea.Cmd) {
	l := m.activeList()
	switch {
	case key.Matches(msg, keys.Search) && m.screen == screenSearch:
		m.searchInput.Reset()
		return m, m.searchInput.Focus()
	case key.Matches(msg, keys.Enter):
		item, ok := l.selected()
		if !ok {
			return m, nil
		}
		m.screen = screenNowPlaying
		m.actionInFlight = true
		return m, play(item)
	case key.Matches(msg, keys.QueueAdd):
		item, ok := l.selected()
		if !ok || item.trackURI == "" {
			// Device rows have no trackURI — queueing is track-only.
			return m, nil
		}
		// Stay on the current screen so several tracks can be queued in a row.
		m.actionInFlight = true
		return m, m.queueTrack(item)
	}
	var cmd tea.Cmd
	l.list, cmd = l.list.Update(msg)
	return m, cmd
}

// queueTrack appends the selected track to the active device's queue —
// playback keeps going, nothing is interrupted.
func (m Model) queueTrack(item listItem) tea.Cmd {
	uri := item.trackURI
	client := m.client
	return actionCmd(func() error {
		if uri == "" {
			return fmt.Errorf("selected item has no track URI")
		}
		return client.AddToQueue(uri)
	})
}

// transferToDevice moves playback to the selected Connect device using
// PlayWithDeviceQuery — the empirically confirmed targeting slot (the
// documented TransferPlayback endpoint 404s, see spotifyapi/playback.go).
// Note this also resumes playback if it was paused; that's the endpoint's
// semantics ("play on this device"), acceptable for a "switch device" action.
func (m Model) transferToDevice(item listItem) tea.Cmd {
	deviceID := item.id
	client := m.client
	return actionCmd(func() error {
		if deviceID == "" {
			return fmt.Errorf("selected device has no ID")
		}
		return client.PlayWithDeviceQuery(deviceID)
	})
}

// playbackEnded reports whether playback ran off the end of its material
// between two polls — previous poll: playing and within nearEndMs of the
// track's end; this poll: stopped at position 0 (or gone idle). A user
// pausing mid-track fails the near-end check, so it doesn't trigger this.
func playbackEnded(prev, cur *spotifyapi.PlaybackState, nearEndMs int) bool {
	if prev == nil || !prev.IsPlaying || prev.Item.DurationMs == 0 {
		return false
	}
	if prev.Item.DurationMs-prev.ProgressMs > nearEndMs {
		return false
	}
	if cur == nil {
		return true
	}
	// Spotify reports "stopped at the end" as either position 0 or parked at
	// the track's full duration, depending on device — accept both.
	return !cur.IsPlaying && (cur.ProgressMs == 0 || cur.ProgressMs >= cur.Item.DurationMs)
}

// autoplayExcludes is what similar-track picks must avoid: the reference
// track itself plus everything in the open playlist (chaining back into
// what was just listened to isn't "similar", it's a rerun).
func (m Model) autoplayExcludes(last spotifyapi.Track) map[string]bool {
	exclude := map[string]bool{last.ID: true}
	for _, it := range m.playlistTracks.list.Items() {
		if li, ok := it.(listItem); ok {
			exclude[li.id] = true
		}
	}
	return exclude
}

// similarTrackURIs assembles up to n track URIs "similar" to the query
// artist: artist search first, the user's own top tracks as filler. The
// endpoints that would do this properly are dead for development-mode apps
// (GET /recommendations 404s, /artists/{id}/top-tracks 403s — both probed).
func similarTrackURIs(client *spotifyapi.Client, query string, exclude map[string]bool, n int) []string {
	var uris []string
	add := func(tracks []spotifyapi.Track) {
		for _, t := range tracks {
			if len(uris) >= n || exclude[t.ID] {
				continue
			}
			exclude[t.ID] = true
			uris = append(uris, "spotify:track:"+t.ID)
		}
	}
	if results, err := client.SearchTracks(query, 10); err == nil {
		add(results.Tracks)
	}
	if len(uris) < n {
		if top, err := client.GetMyTopTracks(20); err == nil {
			add(top)
		}
	}
	return uris
}

// autoplaySimilarCmd starts similar-tracks playback outright — the backup
// path for when playback already stopped (the queue seed either wasn't
// possible or didn't land in time).
func (m Model) autoplaySimilarCmd(last spotifyapi.Track) tea.Cmd {
	client := m.client
	exclude := m.autoplayExcludes(last)
	query := last.Name
	if len(last.Artists) > 0 {
		query = last.Artists[0]
	}
	return m.resolveDeviceAndRun(func(deviceID string) error {
		uris := similarTrackURIs(client, query, exclude, 10)
		if len(uris) == 0 {
			return fmt.Errorf("autoplay: no similar tracks found for %q", query)
		}
		return client.PlayURIs(deviceID, uris)
	})
}

// autoplaySeedResultMsg reports the queue-seeding attempt (below).
type autoplaySeedResultMsg struct {
	forTrackID string
	err        error
}

// seedAutoplayCmd is the primary autoplay path: while the LAST queued track
// is still playing, push similar tracks into the real Spotify queue
// (AddToQueue) — playback then continues seamlessly without this widget
// having to detect the stop, and the "next" label and queue screen show the
// seeded tracks like any other queue content.
func (m Model) seedAutoplayCmd(last spotifyapi.Track) tea.Cmd {
	client := m.client
	exclude := m.autoplayExcludes(last)
	query := last.Name
	if len(last.Artists) > 0 {
		query = last.Artists[0]
	}
	forID := last.ID
	return func() tea.Msg {
		uris := similarTrackURIs(client, query, exclude, 2)
		if len(uris) == 0 {
			return autoplaySeedResultMsg{forTrackID: forID, err: fmt.Errorf("no similar tracks for %q", query)}
		}
		for _, u := range uris {
			if err := client.AddToQueue(u); err != nil {
				return autoplaySeedResultMsg{forTrackID: forID, err: err}
			}
		}
		return autoplaySeedResultMsg{forTrackID: forID}
	}
}

// resolveDeviceAndRun wraps run with device resolution: prefer the device
// from the last-known playback state, else pick the active (or first)
// device from GetDevices — playback-start endpoints all need a target when
// nothing is actively playing. Always returns a real Cmd resolving to an
// actionResultMsg — never nil (see CLAUDE.md's actionInFlight invariant).
func (m Model) resolveDeviceAndRun(run func(deviceID string) error) tea.Cmd {
	client := m.client
	knownDeviceID := ""
	if m.state != nil {
		knownDeviceID = m.state.Device.ID
	}
	return actionCmd(func() error {
		deviceID := knownDeviceID
		if deviceID == "" {
			devices, err := client.GetDevices()
			if err != nil {
				return fmt.Errorf("no device known, and listing devices failed: %w", err)
			}
			for _, d := range devices {
				if deviceID == "" || d.IsActive {
					deviceID = d.ID
				}
				if d.IsActive {
					break
				}
			}
			if deviceID == "" {
				return fmt.Errorf("no Spotify devices available — open Spotify on a device first")
			}
		}
		return run(deviceID)
	})
}

func (m Model) playTrackSelection(item listItem) tea.Cmd {
	trackURI := item.trackURI
	client := m.client
	return m.resolveDeviceAndRun(func(deviceID string) error {
		if trackURI == "" {
			return fmt.Errorf("selected item has no track URI")
		}
		return client.PlayURIs(deviceID, []string{trackURI})
	})
}

// playPlaylistTrackSelection plays a track *in its playlist context*
// (PlayContextAt) so playback continues into the following tracks — a bare
// single-URI play stops after the track and reports a bogus queue. Falls
// back to single-URI play if the playlist ID is somehow unknown.
func (m Model) playPlaylistTrackSelection(item listItem) tea.Cmd {
	playlistID := m.currentPlaylistID
	trackURI := item.trackURI
	client := m.client
	return m.resolveDeviceAndRun(func(deviceID string) error {
		if trackURI == "" {
			return fmt.Errorf("selected item has no track URI")
		}
		if playlistID == "" {
			return client.PlayURIs(deviceID, []string{trackURI})
		}
		return client.PlayContextAt(deviceID, "spotify:playlist:"+playlistID, trackURI)
	})
}

// playContextSelection starts the whole playlist as the playback context,
// so track-to-track continuation works — unlike playTrackSelection, which
// plays a single URI with no next-track context. Liked Songs has no
// playable context URI for third-party apps, so it plays as a URI batch.
func (m Model) playContextSelection(item listItem) tea.Cmd {
	playlistID := item.id
	client := m.client
	return m.resolveDeviceAndRun(func(deviceID string) error {
		if playlistID == "" {
			return fmt.Errorf("selected item has no playlist ID")
		}
		if playlistID == likedPlaylistID {
			tracks, err := client.GetSavedTracks()
			if err != nil {
				return err
			}
			if len(tracks) == 0 {
				return fmt.Errorf("no liked songs to play")
			}
			uris := make([]string, 0, len(tracks))
			for _, t := range tracks {
				uris = append(uris, "spotify:track:"+t.ID)
			}
			return client.PlayURIs(deviceID, uris)
		}
		return client.PlayContext(deviceID, "spotify:playlist:"+playlistID)
	})
}

// playlistNameByID resolves a playlist's display name from the loaded
// playlists list, falling back to a generic title when it isn't there
// (another user's playlist, or the list hasn't loaded yet).
func (m Model) playlistNameByID(id string) string {
	for _, it := range m.playlists.list.Items() {
		if li, ok := it.(listItem); ok && li.id == id {
			return li.label
		}
	}
	return "Now Playing"
}

func trackLabel(t spotifyapi.Track) string {
	label := t.Name
	if len(t.Artists) > 0 {
		label += " — " + t.Artists[0]
	}
	return label
}

func trackItems(tracks []spotifyapi.Track) []list.Item {
	items := make([]list.Item, 0, len(tracks))
	for _, t := range tracks {
		items = append(items, listItem{
			label:    trackLabel(t),
			duration: formatMs(t.DurationMs),
			id:       t.ID,
			trackURI: "spotify:track:" + t.ID,
		})
	}
	return items
}

func deviceItems(devices []spotifyapi.Device) []list.Item {
	items := make([]list.Item, 0, len(devices))
	for _, d := range devices {
		label := d.Name
		if d.Type != "" {
			label += " · " + d.Type
		}
		if d.IsActive {
			label += "  ● active"
		}
		items = append(items, listItem{label: label, id: d.ID})
	}
	return items
}

func playlistItems(playlists []spotifyapi.Playlist) []list.Item {
	items := make([]list.Item, 0, len(playlists))
	for _, p := range playlists {
		// TracksTotal isn't shown: Spotify's Feb 2026 API migration made
		// GET /me/playlists return a null "tracks" field (confirmed via
		// curl), so this count is always 0 now — a fake "(0 tracks)" label
		// would be actively misleading rather than just missing data.
		items = append(items, listItem{label: p.Name, id: p.ID})
	}
	return items
}

// handleControlKey is the v2 now-playing control surface (play/pause/next/
// prev/volume/shuffle/repeat).
func (m Model) handleControlKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.PlayPause):
		m.actionInFlight = true
		wasPlaying := m.state != nil && m.state.IsPlaying
		if m.state != nil {
			m.state.IsPlaying = !wasPlaying
		}
		if wasPlaying {
			return m, actionCmd(m.client.Pause)
		}
		deviceID := ""
		if m.state != nil {
			deviceID = m.state.Device.ID
		}
		if deviceID == "" {
			return m, actionCmd(m.client.PlayResume)
		}
		return m, actionCmd(func() error { return m.client.PlayWithDeviceQuery(deviceID) })
	case key.Matches(msg, keys.Next):
		m.actionInFlight = true
		return m, actionCmd(m.client.Next)
	case key.Matches(msg, keys.Previous):
		m.actionInFlight = true
		return m, actionCmd(m.client.Previous)
	case key.Matches(msg, keys.VolumeUp):
		return m.volumeStep(5)
	case key.Matches(msg, keys.VolumeDown):
		return m.volumeStep(-5)
	case key.Matches(msg, keys.Shuffle):
		if m.state == nil {
			return m, nil
		}
		m.actionInFlight = true
		next := !m.state.ShuffleState
		m.state.ShuffleState = next
		return m, actionCmd(func() error { return m.client.SetShuffle(next) })
	case key.Matches(msg, keys.Repeat):
		if m.state == nil {
			return m, nil
		}
		m.actionInFlight = true
		next := nextRepeatMode(m.state.RepeatState)
		m.state.RepeatState = next
		return m, actionCmd(func() error { return m.client.SetRepeat(next) })
	case key.Matches(msg, keys.Like):
		if m.state == nil || m.state.Item.ID == "" {
			return m, nil
		}
		m.actionInFlight = true
		trackID := m.state.Item.ID
		liked := !m.likedCurrent // optimistic flip, like play/pause
		m.likedCurrent = liked
		m.likedForID = trackID
		client := m.client
		if liked {
			return m, actionCmd(func() error { return client.SaveTrack(trackID) })
		}
		return m, actionCmd(func() error { return client.RemoveSavedTrack(trackID) })
	}
	return m, nil
}

func (m Model) volumeStep(delta int) (tea.Model, tea.Cmd) {
	if m.state == nil {
		return m, nil
	}
	next := m.state.Device.VolumePercent + delta
	if next < 0 {
		next = 0
	}
	if next > 100 {
		next = 100
	}
	m.actionInFlight = true
	m.state.Device.VolumePercent = next
	return m, actionCmd(func() error { return m.client.SetVolume(next) })
}

func nextRepeatMode(mode string) string {
	switch mode {
	case "off":
		return "context"
	case "context":
		return "track"
	default:
		return "off"
	}
}
