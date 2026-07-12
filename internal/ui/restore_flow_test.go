package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jangyoonsung/spotify-tui-go/internal/config"
	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

// pump runs Update and then executes returned commands the way the
// bubbletea runtime would, feeding resulting messages back in (recursing
// through batches) — needed for async list behaviors like fuzzy filtering,
// whose matches arrive via a FilterMatchesMsg produced by a command.
func pump(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	return runCmd(t, next.(Model), cmd)
}

func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = runCmd(t, m, tea.Cmd(c))
		}
		return m
	}
	// Only feed back message types Update actually consumes — recycling
	// tick messages would loop forever.
	if _, ok := msg.(list.FilterMatchesMsg); ok {
		return pump(t, m, msg)
	}
	return m
}

// Simulates the post-restart flow: state restored (tracks box focused, like
// the state the user quit in), playlists+tracks arrive, up/down picks a
// track; esc hands focus back to playlists for switching. HOME is isolated
// because the enter-on-playlist handler persists UIState — without that,
// this test once overwrote the real state.json with fixture IDs, which the
// next real launch 400'd on ("Invalid base62 id").
func TestRestoreFocusesTracksBox(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := config.SaveUIState(config.UIState{
		LastPlaylistID: "pl2", LastPlaylistName: "B", LastTrackID: "t2",
	}); err != nil {
		t.Fatal(err)
	}

	m := New(nil, config.Config{})
	if !m.focusTracks {
		t.Fatalf("restore did not focus the tracks box")
	}
	if !m.playlistTracks.loading {
		t.Fatalf("restore did not mark the tracks box loading")
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

	m := New(nil, config.Config{})
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

// The "f" fuzzy filter narrows the playlists list, and while typing into the
// filter, global bindings (like q=quit) must not fire.
func TestListFuzzyFilter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New(nil, config.Config{})
	next, _ := m.Update(playlistsResultMsg{playlists: []spotifyapi.Playlist{
		{ID: "pl1", Name: "Road Trip"}, {ID: "pl2", Name: "Quiet Focus"}, {ID: "pl3", Name: "Workout"},
	}})
	m = next.(Model)

	// Start filtering and type "q" — must go into the filter, not quit
	// (pump would surface a tea.QuitMsg as an untouched model; the visible-
	// items assertion below fails loudly if filtering didn't engage).
	m = pump(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = pump(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if got := m.playlists.list.FilterState(); got != list.Filtering {
		t.Fatalf("after f+q: filter state = %v, want Filtering (q must not quit)", got)
	}

	// Accept the filter; only "Quiet Focus" should remain visible.
	m = pump(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := len(m.playlists.list.VisibleItems()); got != 1 {
		t.Fatalf("filter 'q' left %d visible items, want 1", got)
	}
	if it, _ := m.playlists.selected(); it.id != "pl2" {
		t.Fatalf("filtered selection = %q, want pl2 (Quiet Focus)", it.id)
	}

	// Esc clears the applied filter first (screen/focus unchanged).
	m = pump(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if got := len(m.playlists.list.VisibleItems()); got != 3 {
		t.Fatalf("esc did not clear the filter: %d visible items, want 3", got)
	}
}
