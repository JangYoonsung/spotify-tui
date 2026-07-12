package config

import "testing"

func TestUIStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if s := LoadUIState(); s != (UIState{}) {
		t.Fatalf("LoadUIState() on fresh HOME = %+v, want zero value", s)
	}

	want := UIState{LastPlaylistID: "pl123", LastPlaylistName: "Road Trip", LastTrackID: "trk456"}
	if err := SaveUIState(want); err != nil {
		t.Fatalf("SaveUIState: %v", err)
	}
	if got := LoadUIState(); got != want {
		t.Fatalf("LoadUIState() = %+v, want %+v", got, want)
	}
}
