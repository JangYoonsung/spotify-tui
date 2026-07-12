package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
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
	queueList   listState
	recentList  listState
	// spin animates the "loading…" rows. Its ticker is gated: armed whenever
	// a fetch flips some listState.loading on, and dropped (not re-armed) by
	// spinner.TickMsg once nothing is loading anymore.
	spin spinner.Model
	// helpView renders the footer from the active screen's key bindings
	// (keysFor) — ? toggles the expanded view.
	helpView help.Model

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
	// nextTrack is the "up next" label from GET /me/player/queue — fetched
	// on track change and after control actions, cleared while unknown.
	nextTrack string
	// radioTracks holds Spotify's real autoplay/radio for when playback sits
	// at the end of a playlist: there the queue endpoint lies (it wraps
	// around to the playlist's first track), so both the "next" label and
	// the Up Next screen show these instead. radioForContext tags which
	// context they were fetched for, so a stale fetch isn't reused.
	radioTracks     []spotifyapi.Track
	radioForContext string
	// lastTracksReload throttles the tick-driven track-list recovery so a
	// persistent failure (e.g. a rate limit) isn't hammered every poll.
	lastTracksReload time.Time
	// rateLimitedUntil suspends polling after a 429 (Spotify's Retry-After),
	// so the widget backs off instead of hammering the API and keeping the
	// limit alive.
	rateLimitedUntil time.Time
	// likedCurrent/likedForID: whether the CURRENT track is in Liked Songs
	// — checked once per track change, toggled optimistically by the l key.
	likedCurrent bool
	likedForID   string

	// lastContextURI tracks what playback is playing FROM — when it changes
	// to a playlist this box isn't showing (e.g. playback started from the
	// phone), the tracks box follows it. Only on the change edge, so the
	// user's own browsing isn't overridden every poll.
	lastContextURI string

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

	hv := help.New()
	hv.Styles.ShortKey = metaStyle
	hv.Styles.ShortDesc = footerStyle
	hv.Styles.ShortSeparator = dimStyle
	hv.Styles.FullKey = metaStyle
	hv.Styles.FullDesc = footerStyle
	hv.Styles.FullSeparator = dimStyle

	m := Model{
		client:      client,
		cfg:         cfg,
		searchInput: ti,
		helpView:    hv,
		spin:        spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(dimStyle)),
		// Playlists start fetching in Init — show the spinner from frame one
		// instead of a misleading "no results" until the first response.
		playlists:      loadingListState(),
		playlistTracks: newListState(),
		search:         newListState(),
		devices:        newListState(),
		queueList:      newListState(),
		recentList:     newListState(),
	}

	if st := config.LoadUIState(); st.LastPlaylistID != "" {
		m.restorePlaylistID = st.LastPlaylistID
		m.restoreTrackID = st.LastTrackID
		m.currentPlaylistID = st.LastPlaylistID
		m.playlistTracksTitle = st.LastPlaylistName
		m.playlistTracks = loadingListState()
		// Focus the restored tracks box, matching the state the user quit in
		// (they had a playlist open): up/down/enter pick a track right away.
		// Esc hands focus back to the playlists box as usual.
		m.focusTracks = true
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{refreshCmd(m.client), playlistsCmd(m.client), tickCmd(m.cfg.PollInterval), marqueeTickCmd(), m.spin.Tick}
	if m.restorePlaylistID != "" {
		cmds = append(cmds, playlistTracksCmd(m.client, m.restorePlaylistID))
	}
	return tea.Batch(cmds...)
}
