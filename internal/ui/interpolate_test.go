package ui

import (
	"testing"
	"time"

	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

func TestInterpolatedState(t *testing.T) {
	base := time.Now()
	playing := &spotifyapi.PlaybackState{
		IsPlaying:  true,
		ProgressMs: 10_000,
		Item:       spotifyapi.Track{DurationMs: 60_000},
	}

	got := interpolatedState(playing, base, base.Add(2*time.Second))
	if got.ProgressMs != 12_000 {
		t.Fatalf("playing: ProgressMs = %d, want 12000", got.ProgressMs)
	}
	if playing.ProgressMs != 10_000 {
		t.Fatalf("original state mutated: ProgressMs = %d", playing.ProgressMs)
	}

	got = interpolatedState(playing, base, base.Add(5*time.Minute))
	if got.ProgressMs != 60_000 {
		t.Fatalf("clamp: ProgressMs = %d, want 60000 (track duration)", got.ProgressMs)
	}

	paused := &spotifyapi.PlaybackState{IsPlaying: false, ProgressMs: 10_000}
	if got := interpolatedState(paused, base, base.Add(2*time.Second)); got.ProgressMs != 10_000 {
		t.Fatalf("paused: ProgressMs = %d, want 10000 (untouched)", got.ProgressMs)
	}

	if got := interpolatedState(nil, base, base); got != nil {
		t.Fatalf("nil state: got %+v, want nil", got)
	}
	if got := interpolatedState(playing, time.Time{}, base); got.ProgressMs != 10_000 {
		t.Fatalf("zero lastRefresh: ProgressMs = %d, want 10000 (no interpolation)", got.ProgressMs)
	}
}
