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

	playlistTracks      listState
	playlistTracksTitle string
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

	return Model{
		client:      client,
		cfg:         cfg,
		searchInput: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(refreshCmd(m.client), playlistsCmd(m.client), tickCmd(m.cfg.PollInterval), marqueeTickCmd())
}
