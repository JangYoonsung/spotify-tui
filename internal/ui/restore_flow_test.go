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
	// Row 0 is the virtual "♥ Liked Songs" entry — move down to the playlist.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
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

	// Esc clears the applied filter first (screen/focus unchanged); all rows
	// return, including the virtual Liked Songs entry.
	m = pump(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if got := len(m.playlists.list.VisibleItems()); got != 4 {
		t.Fatalf("esc did not clear the filter: %d visible items, want 4 (3 playlists + liked row)", got)
	}
}

// Autoplay must fire only when playback ran off the end of its material —
// not when the user pauses mid-track.
func TestPlaybackEnded(t *testing.T) {
	playingNearEnd := &spotifyapi.PlaybackState{
		IsPlaying: true, ProgressMs: 178_000,
		Item: spotifyapi.Track{ID: "t1", DurationMs: 180_000},
	}
	playingMidTrack := &spotifyapi.PlaybackState{
		IsPlaying: true, ProgressMs: 60_000,
		Item: spotifyapi.Track{ID: "t1", DurationMs: 180_000},
	}
	stoppedAtZero := &spotifyapi.PlaybackState{
		IsPlaying: false, ProgressMs: 0,
		Item: spotifyapi.Track{ID: "t1", DurationMs: 180_000},
	}
	pausedMidTrack := &spotifyapi.PlaybackState{
		IsPlaying: false, ProgressMs: 60_000,
		Item: spotifyapi.Track{ID: "t1", DurationMs: 180_000},
	}

	const nearEnd = 6000
	cases := []struct {
		name      string
		prev, cur *spotifyapi.PlaybackState
		want      bool
	}{
		{"ran off the end, stopped at zero", playingNearEnd, stoppedAtZero, true},
		{"ran off the end, went idle", playingNearEnd, nil, true},
		{"user paused mid-track", playingMidTrack, pausedMidTrack, false},
		{"still playing", playingNearEnd, playingNearEnd, false},
		{"no previous state", nil, stoppedAtZero, false},
		{"paused near end but resumed position kept", playingNearEnd, pausedMidTrack, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := playbackEnded(tc.prev, tc.cur, nearEnd); got != tc.want {
				t.Fatalf("playbackEnded = %v, want %v", got, tc.want)
			}
		})
	}
}

// Cursor movement on the queue screen (and by construction search/devices,
// which share handleListKey). This shipped broken once: handleListKey took
// a *listState pointing into the CALLER's model copy while returning its
// own receiver copy, so every list.Update was silently discarded.
func TestQueueScreenCursorMoves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New(nil, config.Config{})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = next.(Model)
	if m.screen != screenQueue {
		t.Fatalf("u did not open the queue screen")
	}

	next, _ = m.Update(queueViewResultMsg{tracks: []spotifyapi.Track{
		{ID: "t1", Name: "one"}, {ID: "t2", Name: "two"}, {ID: "t3", Name: "three"},
	}})
	m = next.(Model)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if got := m.queueList.list.Index(); got != 1 {
		t.Fatalf("down on queue screen: index = %d, want 1", got)
	}
}

// A fake queue (Spotify pads single-URI playback's queue with the current
// track repeated, repeat off) must leave nextTrack empty — it blocked the
// autoplay chain's queue-empty guard when treated as a real "next". With
// repeat-one, the same-track next is real and shown.
func TestQueueResultFakeVsRepeatOne(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cur := &spotifyapi.PlaybackState{IsPlaying: true, Item: spotifyapi.Track{ID: "t1", Name: "Same"}}
	fakeQueue := queueResultMsg{forTrackID: "t1", tracks: []spotifyapi.Track{
		{ID: "t1", Name: "Same"}, {ID: "t1", Name: "Same"},
	}}

	m := New(nil, config.Config{})
	m.state = cur
	next, _ := m.Update(fakeQueue)
	if got := next.(Model).nextTrack; got != "" {
		t.Fatalf("fake padded queue set nextTrack = %q, want empty (unblocks autoplay)", got)
	}

	m.state = &spotifyapi.PlaybackState{IsPlaying: true, RepeatState: "track", Item: spotifyapi.Track{ID: "t1", Name: "Same"}}
	next, _ = m.Update(fakeQueue)
	if got := next.(Model).nextTrack; got != "Same" {
		t.Fatalf("repeat-one queue: nextTrack = %q, want \"Same\"", got)
	}
}

// End-to-end autoplay trigger: playing near the track's end, next poll says
// stopped, queue was empty -> an action (the similar-tracks play) fires.
func TestAutoplayTriggersOnPlaybackEnd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New(nil, config.Config{})
	m.cfg.PollInterval = 3_000_000_000 // 3s, so nearEnd = 6s

	next, _ := m.Update(refreshResultMsg{state: &spotifyapi.PlaybackState{
		IsPlaying: true, ProgressMs: 178_000,
		Item: spotifyapi.Track{ID: "t1", Name: "Last", Artists: []string{"Artist"}, DurationMs: 180_000},
	}})
	m = next.(Model)

	next, cmd := m.Update(refreshResultMsg{state: &spotifyapi.PlaybackState{
		IsPlaying: false, ProgressMs: 0,
		Item: spotifyapi.Track{ID: "t1", Name: "Last", Artists: []string{"Artist"}, DurationMs: 180_000},
	}})
	m = next.(Model)

	if !m.actionInFlight {
		t.Fatalf("playback end did not set actionInFlight (autoplay not triggered)")
	}
	if cmd == nil {
		t.Fatalf("playback end returned no autoplay command")
	}
}

// Playing a playlist's LAST track with repeat off: Spotify's queue wraps
// around to the playlist's first track, but playback actually stops there —
// the wrap must not become the "next" label (it also blocked autoplay's
// queue-empty guard). With repeat=context the loop is real and stays.
func TestQueueWrapAroundSuppressed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New(nil, config.Config{})
	next, _ := m.Update(playlistTracksResultMsg{tracks: []spotifyapi.Track{
		{ID: "a", Name: "First"}, {ID: "b", Name: "Mid"}, {ID: "c", Name: "Last"},
	}})
	m = next.(Model)

	wrapQueue := queueResultMsg{forTrackID: "c", tracks: []spotifyapi.Track{
		{ID: "a", Name: "First"}, {ID: "b", Name: "Mid"},
	}}
	// The wrap judgment compares against the PLAYING CONTEXT's track list.
	const ctx = "spotify:playlist:pl9"
	m.currentPlaylistID = "pl9"

	m.state = &spotifyapi.PlaybackState{IsPlaying: true, RepeatState: "off", ContextURI: ctx, Item: spotifyapi.Track{ID: "c", Name: "Last"}}
	next, _ = m.Update(wrapQueue)
	if got := next.(Model).nextTrack; got != "" {
		t.Fatalf("repeat off: wrap-around queue set nextTrack = %q, want empty", got)
	}

	m.state = &spotifyapi.PlaybackState{IsPlaying: true, RepeatState: "context", ContextURI: ctx, Item: spotifyapi.Track{ID: "c", Name: "Last"}}
	next, _ = m.Update(wrapQueue)
	if got := next.(Model).nextTrack; got != "First" {
		t.Fatalf("repeat context: nextTrack = %q, want \"First\" (real loop)", got)
	}

	// Shuffle randomizes order — "last position" is meaningless, keep next.
	m.state = &spotifyapi.PlaybackState{IsPlaying: true, RepeatState: "off", ShuffleState: true, ContextURI: ctx, Item: spotifyapi.Track{ID: "c", Name: "Last"}}
	next, _ = m.Update(wrapQueue)
	if got := next.(Model).nextTrack; got != "First" {
		t.Fatalf("shuffle: nextTrack = %q, want \"First\" (no wrap judgment)", got)
	}

	// Mid-playlist the same queue shape is a real "next", not a wrap.
	m.state = &spotifyapi.PlaybackState{IsPlaying: true, RepeatState: "off", ContextURI: ctx, Item: spotifyapi.Track{ID: "b", Name: "Mid"}}
	next, _ = m.Update(queueResultMsg{forTrackID: "b", tracks: []spotifyapi.Track{{ID: "c", Name: "Last"}}})
	if got := next.(Model).nextTrack; got != "Last" {
		t.Fatalf("mid-playlist: nextTrack = %q, want \"Last\"", got)
	}
}

// Full seed chain: last track of the playing context + wrap queue ->
// nextTrack suppressed -> similar-tracks seed command fired, once.
func TestSeedFiresOnLastContextTrack(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := New(nil, config.Config{})
	next, _ := m.Update(playlistTracksResultMsg{tracks: []spotifyapi.Track{
		{ID: "a", Name: "First"}, {ID: "b", Name: "Mid"}, {ID: "c", Name: "Last", Artists: []string{"Artist"}},
	}})
	m = next.(Model)
	m.currentPlaylistID = "pl9"
	m.state = &spotifyapi.PlaybackState{
		IsPlaying: true, RepeatState: "off",
		ContextURI: "spotify:playlist:pl9",
		Item:       spotifyapi.Track{ID: "c", Name: "Last", Artists: []string{"Artist"}},
	}

	wrapQueue := queueResultMsg{forTrackID: "c", tracks: []spotifyapi.Track{
		{ID: "a", Name: "First"}, {ID: "b", Name: "Mid"},
	}}
	next, cmd := m.Update(wrapQueue)
	m = next.(Model)

	if m.nextTrack != "" {
		t.Fatalf("wrap not suppressed: nextTrack = %q", m.nextTrack)
	}
	if m.autoplaySeededFor != "c" {
		t.Fatalf("seed not marked: autoplaySeededFor = %q, want c", m.autoplaySeededFor)
	}
	if cmd == nil {
		t.Fatalf("no seed command fired")
	}

	// Same queue result again: seed must NOT fire twice for the same track.
	next, cmd = m.Update(wrapQueue)
	if cmd != nil {
		t.Fatalf("seed fired twice for the same track")
	}
	_ = next
}
