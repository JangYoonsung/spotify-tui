package ui

import (
	"fmt"
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
		tracks, err := client.GetPlaylistTracks(playlistID, 100)
		return playlistTracksResultMsg{tracks: tracks, err: err}
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
		m.state = msg.state
		if msg.state != nil && msg.state.Item.ID != "" && msg.state.Item.ID != m.artTrackID {
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
				return m, artCmd(imageURL, msg.state.Item.ID, m.cfg.ExperimentalKittyArt)
			}
			m.artTrackID, m.artRendered = msg.state.Item.ID, ""
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
		return m, refreshCmd(m.client)

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
			m.playlists.setItems(playlistItems(msg.playlists))
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
	case key.Matches(msg, keys.Refresh):
		return m, tea.Batch(refreshCmd(m.client), playlistsCmd(m.client))
	case key.Matches(msg, keys.Help):
		m.helpView.ShowAll = !m.helpView.ShowAll
		return m, nil
	}

	switch m.screen {
	case screenSearch:
		return m.handleListKey(&m.search, msg, m.playTrackSelection)
	case screenDevices:
		return m.handleListKey(&m.devices, msg, m.transferToDevice)
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
			return m, m.playTrackSelection(item)
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

// handleListKey drives selection for the Search/Devices screens: enter and
// queue-add are this app's actions, everything else (cursor movement,
// paging, "f" fuzzy filter) is delegated to bubbles/list.
func (m Model) handleListKey(l *listState, msg tea.KeyMsg, play func(listItem) tea.Cmd) (tea.Model, tea.Cmd) {
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

// playContextSelection starts the whole playlist as the playback context,
// so track-to-track continuation works — unlike playTrackSelection, which
// plays a single URI with no next-track context.
func (m Model) playContextSelection(item listItem) tea.Cmd {
	playlistID := item.id
	client := m.client
	return m.resolveDeviceAndRun(func(deviceID string) error {
		if playlistID == "" {
			return fmt.Errorf("selected item has no playlist ID")
		}
		return client.PlayContext(deviceID, "spotify:playlist:"+playlistID)
	})
}

func trackItems(tracks []spotifyapi.Track) []list.Item {
	items := make([]list.Item, 0, len(tracks))
	for _, t := range tracks {
		label := t.Name
		if len(t.Artists) > 0 {
			label += " — " + t.Artists[0]
		}
		items = append(items, listItem{
			label:    label,
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
