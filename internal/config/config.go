// Package config resolves runtime settings from flags and environment
// variables (via a .env file — see env.go). Secrets (client_id) never live
// in a committed file; only SPOTIFY_TUI_CLIENT_ID.
package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	ClientID         string
	CallbackPort     int
	PollInterval     time.Duration
	Once             bool
	ForceLogin       bool
	ShowDevices      bool
	DiagnoseDeviceID string

	DiagnosePlaylists  bool
	DiagnoseSearch     string
	DiagnosePlayCtx    string
	DiagnosePlayURI    string
	DiagnoseArt        string
	DiagnosePlaylistID string
	DiagnoseQueue      bool
	DiagnoseRecommend  string
	DiagnoseRecent     bool
	DiagnoseTopTracks  string
	DiagnoseMyTop      bool
	DiagnoseAddQueue   string
	DiagnoseSkip       bool

	// ExperimentalKittyArt switches album art from ANSI half-block text to
	// termimg.Auto (real Kitty/Sixel/iTerm2 graphics protocol if the
	// terminal supports one). Wired up (internal/albumart + widget.go's
	// graphics-art layout path), but genuinely experimental: tested against
	// a real Kitty-capable terminal and found to desync bubbletea's
	// line-based redraw (the image occupies real terminal rows the string
	// diffing renderer doesn't account for) — mitigated with an empirical
	// newline-padding hack, not a real fix. Half-block is the stable
	// default for a reason; opt into this expecting rough edges.
	ExperimentalKittyArt bool
}

// Parse loads .env (if present), then flags — flags never override a
// required secret that's missing; ClientID must come from the environment.
func Parse(args []string) (Config, error) {
	if err := LoadEnv(); err != nil {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	cfg := Config{}
	fs := flag.NewFlagSet("spotify-tui", flag.ContinueOnError)
	fs.IntVar(&cfg.CallbackPort, "port", 8942, "local OAuth callback port (must match the redirect URI registered on the Spotify dashboard)")
	fs.DurationVar(&cfg.PollInterval, "poll-interval", 3*time.Second, "how often to poll playback state")
	fs.BoolVar(&cfg.Once, "once", false, "fetch playback state once, print plain text, and exit (no TUI)")
	fs.BoolVar(&cfg.ForceLogin, "login", false, "force the browser login flow even if a cached token exists")
	fs.BoolVar(&cfg.ShowDevices, "show-devices", false, "with --once, print devices instead of playback state")
	fs.StringVar(&cfg.DiagnoseDeviceID, "diagnose-device", "", "with --once, run the play/transfer diagnostic (plan section 7) against this device ID and exit")
	fs.BoolVar(&cfg.DiagnosePlaylists, "diagnose-playlists", false, "with --once, print GetPlaylists() results and exit")
	fs.StringVar(&cfg.DiagnoseSearch, "diagnose-search", "", "with --once, run SearchTracks() with this query and print results")
	fs.StringVar(&cfg.DiagnosePlayCtx, "diagnose-play-context", "", "with --once and --diagnose-device, call PlayContext() with this context URI")
	fs.StringVar(&cfg.DiagnosePlayURI, "diagnose-play-uri", "", "with --once and --diagnose-device, call PlayURIs() with this track URI")
	fs.StringVar(&cfg.DiagnoseArt, "diagnose-art", "", "with --once, render this image URL as ANSI art and print it")
	fs.StringVar(&cfg.DiagnosePlaylistID, "diagnose-playlist-tracks", "", "with --once, print GetPlaylistTracks() results for this playlist ID")
	fs.BoolVar(&cfg.DiagnoseQueue, "diagnose-queue", false, "with --once, print the playback state and GetQueue() results")
	fs.StringVar(&cfg.DiagnoseRecommend, "diagnose-recommend", "", "with --once, probe GET /recommendations with this seed track ID (availability check)")
	fs.BoolVar(&cfg.DiagnoseRecent, "diagnose-recent", false, "with --once, probe GET /me/player/recently-played (scope check)")
	fs.StringVar(&cfg.DiagnoseTopTracks, "diagnose-top-tracks", "", "with --once, probe GET /artists/<id>/top-tracks")
	fs.BoolVar(&cfg.DiagnoseMyTop, "diagnose-my-top", false, "with --once, probe GET /me/top/tracks")
	fs.StringVar(&cfg.DiagnoseAddQueue, "diagnose-add-queue", "", "with --once, POST this track URI to the playback queue")
	fs.BoolVar(&cfg.DiagnoseSkip, "diagnose-skip", false, "with --once, skip to the next track (queue accuracy check)")
	fs.BoolVar(&cfg.ExperimentalKittyArt, "experimental-kitty-art", false, "placeholder — not implemented in v3, ANSI half-block art is used regardless")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.ClientID = os.Getenv("SPOTIFY_TUI_CLIENT_ID")
	if cfg.ClientID == "" {
		return Config{}, fmt.Errorf("SPOTIFY_TUI_CLIENT_ID not set — put it in ./.env or ~/.config/spotify-tui-go/.env")
	}

	return cfg, nil
}
