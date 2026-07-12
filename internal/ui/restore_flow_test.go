package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

// Simulates the post-restart flow: state restored (tracks box focused, like
// the state the user quit in), playlists+tracks arrive, up/down picks a
// track; esc hands focus back to playlists for switching.
func TestRestoreFocusesTracksBox(t *testing.T) {
	// The enter-on-playlist handler persists UIState to $HOME — isolate it,
	// or this test overwrites the developer's real state.json with fixture
	// IDs, which the next real launch then 400s on ("Invalid base62 id").
	t.Setenv("HOME", t.TempDir())

	m := Model{
		restorePlaylistID:   "pl2",
		restoreTrackID:      "t2",
		currentPlaylistID:   "pl2",
		playlistTracksTitle: "B",
		playlistTracks:      listState{loading: true},
		focusTracks:         true, // what New() sets when restoring
	}

	next, _ := m.Update(playlistsResultMsg{playlists: []spotifyapi.Playlist{
		{ID: "pl1", Name: "A"}, {ID: "pl2", Name: "B"}, {ID: "pl3", Name: "C"},
	}})
	m = next.(Model)
	if got, _ := m.playlists.selected(); got.id != "pl2" {
		t.Fatalf("playlists cursor after restore = %q, want pl2", got.id)
	}

	next, _ = m.Update(playlistTracksResultMsg{tracks: []spotifyapi.Track{
		{ID: "t1", Name: "one"}, {ID: "t2", Name: "two"}, {ID: "t3", Name: "three"},
	}})
	m = next.(Model)
	if got, _ := m.playlistTracks.selected(); got.id != "t2" {
		t.Fatalf("tracks cursor after restore = %q, want t2", got.id)
	}

	// Down must move the TRACKS cursor (this regressed once: focus stayed on
	// the playlists box after a restore, so up/down picked playlists).
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if got, _ := m.playlistTracks.selected(); got.id != "t3" {
		t.Fatalf("down after restore moved tracks cursor to %q, want t3", got.id)
	}
	if got, _ := m.playlists.selected(); got.id != "pl2" {
		t.Fatalf("down after restore must not move the playlists cursor (got %q)", got.id)
	}

	// Esc returns focus to playlists; enter there opens another playlist.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.playlistTracksTitle != "C" || !m.playlistTracks.loading || cmd == nil {
		t.Fatalf("enter on pl3 after esc: title=%q loading=%v cmd=%v", m.playlistTracksTitle, m.playlistTracks.loading, cmd)
	}
}

// P on a focused playlist starts the whole playlist as the playback context
// AND drills into its tracks box, like enter plus playback.
func TestPlayPlaylistKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := Model{}
	next, _ := m.Update(playlistsResultMsg{playlists: []spotifyapi.Playlist{{ID: "pl1", Name: "A"}}})
	m = next.(Model)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = next.(Model)
	if !m.actionInFlight {
		t.Fatalf("P on a playlist did not set actionInFlight")
	}
	if cmd == nil {
		t.Fatalf("P on a playlist returned no command")
	}
	if !m.focusTracks || !m.playlistTracks.loading || m.playlistTracksTitle != "A" {
		t.Fatalf("P must also open the tracks box: focusTracks=%v loading=%v title=%q",
			m.focusTracks, m.playlistTracks.loading, m.playlistTracksTitle)
	}
}
