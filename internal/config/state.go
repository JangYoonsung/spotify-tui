package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// UIState is small cross-run UI state (not configuration, not a secret):
// currently just the last-opened playlist, so the tracks box survives a
// restart — the widget lives in a cmux dock and restarts along with it.
type UIState struct {
	LastPlaylistID   string `json:"last_playlist_id"`
	LastPlaylistName string `json:"last_playlist_name"`
	// LastTrackID is the track last played from that playlist's tracks box —
	// restored as the cursor position (scrolled into view), not auto-played.
	LastTrackID string `json:"last_track_id,omitempty"`
}

func uiStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "spotify-tui-go", "state.json"), nil
}

// LoadUIState returns the persisted UI state, or the zero value if the file
// doesn't exist yet (first run) or can't be read/parsed — restoring state is
// best-effort convenience, never worth failing or warning at startup over.
func LoadUIState() UIState {
	path, err := uiStatePath()
	if err != nil {
		return UIState{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return UIState{}
	}
	var s UIState
	if err := json.Unmarshal(data, &s); err != nil {
		return UIState{}
	}
	return s
}

// CachedTrack is the minimal track info needed to judge the queue against a
// playlist (wrap-around detection) and to display radio/next labels. Cached
// per playlist so a rate-limited or restarted session can still make those
// judgments from the last good load instead of an empty list.
type CachedTrack struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Artists  []string `json:"artists"`
	Duration int      `json:"duration_ms"`
}

func tracksCachePath(playlistID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// playlistID is a base62 id (or the "__liked__" sentinel) — safe as a
	// filename, but Clean-guard against any stray separators anyway.
	safe := filepath.Base(playlistID)
	return filepath.Join(home, ".config", "spotify-tui-go", "tracks", safe+".json"), nil
}

// SaveTracksCache persists a playlist's tracks; best-effort, errors ignored.
func SaveTracksCache(playlistID string, tracks []CachedTrack) error {
	path, err := tracksCachePath(playlistID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(tracks)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadTracksCache returns a playlist's cached tracks, or nil if none.
func LoadTracksCache(playlistID string) []CachedTrack {
	path, err := tracksCachePath(playlistID)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var tracks []CachedTrack
	if err := json.Unmarshal(data, &tracks); err != nil {
		return nil
	}
	return tracks
}

// SaveUIState persists s, creating the config directory if needed. Callers
// may ignore the error for the same reason LoadUIState never fails.
func SaveUIState(s UIState) error {
	path, err := uiStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
