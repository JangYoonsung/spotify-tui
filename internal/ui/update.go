package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jangyoonsung/spotify-tui-go/internal/albumart"
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
			m.playlists.items = playlistItems(msg.playlists)
			m.playlists.cursor, m.playlists.scrollTop = 0, 0
			if prevID != "" {
				for i, it := range m.playlists.items {
					if it.id == prevID {
						m.playlists.cursor = i
						break
					}
				}
			}
		}
		return m, nil

	case playlistTracksResultMsg:
		m.playlistTracks.loading = false
		m.playlistTracks.err = msg.err
		if msg.err == nil {
			m.playlistTracks.items = trackItems(msg.tracks)
			m.playlistTracks.cursor, m.playlistTracks.scrollTop = 0, 0
		}
		return m, nil

	case searchResultMsg:
		m.search.loading = false
		m.search.err = msg.err
		if msg.err == nil {
			m.search.items = trackItems(msg.tracks)
			m.search.cursor, m.search.scrollTop = 0, 0
		}
		return m, nil

	case devicesResultMsg:
		m.devices.loading = false
		m.devices.err = msg.err
		if msg.err == nil {
			m.devices.items = deviceItems(msg.devices)
			m.devices.cursor, m.devices.scrollTop = 0, 0
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Search screen's textinput must see keystrokes before any global
	// binding — otherwise typing "q" while composing a query would quit
	// the app instead of being typed.
	if m.screen == screenSearch && m.searchInput.Focused() {
		return m.handleSearchTypingKey(msg)
	}

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
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
		m.search = listState{}
		m.searchInput.Reset()
		return m, m.searchInput.Focus()
	case key.Matches(msg, keys.Devices) && m.screen == screenNowPlaying:
		m.screen = screenDevices
		m.devices = listState{loading: true}
		return m, devicesCmd(m.client)
	case key.Matches(msg, keys.Refresh):
		return m, tea.Batch(refreshCmd(m.client), playlistsCmd(m.client))
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
		case key.Matches(msg, keys.Up):
			m.playlistTracks.moveCursor(-1)
			return m, nil
		case key.Matches(msg, keys.Down):
			m.playlistTracks.moveCursor(1)
			return m, nil
		case key.Matches(msg, keys.Enter):
			item, ok := m.playlistTracks.selected()
			if !ok {
				return m, nil
			}
			m.actionInFlight = true
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
		case key.Matches(msg, keys.Up):
			m.playlists.moveCursor(-1)
			return m, nil
		case key.Matches(msg, keys.Down):
			m.playlists.moveCursor(1)
			return m, nil
		case key.Matches(msg, keys.Enter):
			item, ok := m.playlists.selected()
			if !ok {
				return m, nil
			}
			m.focusTracks = true
			m.playlistTracks = listState{loading: true}
			m.playlistTracksTitle = item.label
			return m, playlistTracksCmd(m.client, item.id)
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
		return m, searchCmd(m.client, query)
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// handleListKey drives cursor movement and selection for the browsing part
// of Search/PlaylistTracks screens (once the search box, if any, isn't
// focused).
func (m Model) handleListKey(list *listState, msg tea.KeyMsg, play func(listItem) tea.Cmd) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		list.moveCursor(-1)
		return m, nil
	case key.Matches(msg, keys.Down):
		list.moveCursor(1)
		return m, nil
	case key.Matches(msg, keys.Search) && m.screen == screenSearch:
		m.searchInput.Reset()
		return m, m.searchInput.Focus()
	case key.Matches(msg, keys.Enter):
		item, ok := list.selected()
		if !ok {
			return m, nil
		}
		m.screen = screenNowPlaying
		m.actionInFlight = true
		return m, play(item)
	case key.Matches(msg, keys.QueueAdd):
		item, ok := list.selected()
		if !ok || item.trackURI == "" {
			// Device rows have no trackURI — queueing is track-only.
			return m, nil
		}
		// Stay on the current screen so several tracks can be queued in a row.
		m.actionInFlight = true
		return m, m.queueTrack(item)
	}
	return m, nil
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

// playTrackSelection always returns a real Cmd resolving to an
// actionResultMsg — never nil (see CLAUDE.md's actionInFlight invariant).
func (m Model) playTrackSelection(item listItem) tea.Cmd {
	trackURI := item.trackURI
	client := m.client
	knownDeviceID := ""
	if m.state != nil {
		knownDeviceID = m.state.Device.ID
	}
	return actionCmd(func() error {
		if trackURI == "" {
			return fmt.Errorf("selected item has no track URI")
		}
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
		return client.PlayURIs(deviceID, []string{trackURI})
	})
}

func trackItems(tracks []spotifyapi.Track) []listItem {
	items := make([]listItem, 0, len(tracks))
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

func deviceItems(devices []spotifyapi.Device) []listItem {
	items := make([]listItem, 0, len(devices))
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

func playlistItems(playlists []spotifyapi.Playlist) []listItem {
	items := make([]listItem, 0, len(playlists))
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
