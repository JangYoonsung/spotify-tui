package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jangyoonsung/spotify-tui-go/internal/config"
	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

type Model struct {
	client *spotifyapi.Client
	cfg    config.Config

	state          *spotifyapi.PlaybackState
	lastErr        error
	lastRefresh    time.Time
	actionInFlight bool

	screen      screen
	search      listState
	searchInput textinput.Model
	playlists   listState
	devices     listState

	playlistTracks      listState
	playlistTracksTitle string
	// restorePlaylistID is the last-opened playlist persisted from a previous
	// run (config.UIState) — Init() re-fetches its tracks so the tracks box
	// survives a restart, and playlistsResultMsg uses it to put the playlists
	// cursor back on that entry on the initial load.
	restorePlaylistID string
	// restoreTrackID puts the tracks-box cursor back on the last-played track
	// once the restored playlist's tracks arrive. One-shot: cleared after the
	// first playlistTracksResultMsg so later playlist opens start at the top.
	restoreTrackID string
	// currentPlaylistID is whichever playlist the tracks box is showing —
	// needed when persisting UIState on track play (playlistTracksTitle only
	// keeps the display name).
	currentPlaylistID string
	// focusTracks: false = up/down/enter drive the playlists list; true =
	// they drive the playlistTracks list instead. Both boxes stay visible
	// on screenNowPlaying regardless — this only changes which one keyboard
	// input targets, there's no screen switch involved.
	focusTracks bool

	artTrackID  string
	artRendered string

	// marqueeTick drives the ping-pong scroll for a track title too long
	// to fit — advanced by its own fast ticker (marqueeTickCmd), decoupled
	// from the 3s data-poll ticker so the scroll motion stays smooth
	// regardless of --poll-interval. Reset to 0 whenever the track changes.
	marqueeTick int

	width, height int
}

func New(client *spotifyapi.Client, cfg config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "search tracks..."
	ti.CharLimit = 100

	m := Model{
		client:      client,
		cfg:         cfg,
		searchInput: ti,
	}

	if st := config.LoadUIState(); st.LastPlaylistID != "" {
		m.restorePlaylistID = st.LastPlaylistID
		m.restoreTrackID = st.LastTrackID
		m.currentPlaylistID = st.LastPlaylistID
		m.playlistTracksTitle = st.LastPlaylistName
		m.playlistTracks = listState{loading: true}
		// Focus the restored tracks box, matching the state the user quit in
		// (they had a playlist open): up/down/enter pick a track right away.
		// Esc hands focus back to the playlists box as usual.
		m.focusTracks = true
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{refreshCmd(m.client), playlistsCmd(m.client), tickCmd(m.cfg.PollInterval), marqueeTickCmd()}
	if m.restorePlaylistID != "" {
		cmds = append(cmds, playlistTracksCmd(m.client, m.restorePlaylistID))
	}
	return tea.Batch(cmds...)
}
