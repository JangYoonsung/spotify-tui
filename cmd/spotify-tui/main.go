// Command spotify-tui is a TUI now-playing widget for Spotify, controlled
// via the official Web API (OAuth PKCE). Intended to run as a docked panel
// inside cmux, same as ~/dev/cmux-orchestrator.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jangyoonsung/spotify-tui-go/internal/albumart"
	"github.com/jangyoonsung/spotify-tui-go/internal/config"
	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyauth"
	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyradio"
	"github.com/jangyoonsung/spotify-tui-go/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spotify-tui:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	tok, err := ensureToken(cfg)
	if err != nil {
		return err
	}

	// tea.Cmds run on their own goroutines, and bubbletea dispatches several
	// at once from a single tea.Batch (e.g. Init()'s refreshCmd +
	// playlistsCmd). Both would call tokenSource() concurrently; without a
	// lock, two near-simultaneous refreshes could race on reading/writing
	// tok, and since Spotify's PKCE refresh grant rotates the refresh
	// token, whichever response is saved last could persist a dead refresh
	// token to disk — a data race that manifests as an intermittent forced
	// re-login, not a crash, which made it easy to miss.
	var tokMu sync.Mutex
	tokenSource := func() (string, error) {
		tokMu.Lock()
		defer tokMu.Unlock()
		fresh, err := spotifyauth.EnsureFresh(cfg.ClientID, tok)
		if err != nil {
			return "", err
		}
		tok = fresh
		return tok.AccessToken, nil
	}
	client := spotifyapi.New(tokenSource)

	switch {
	case cfg.DiagnoseDeviceID != "" && cfg.DiagnosePlayCtx != "" && cfg.DiagnosePlayURI != "":
		return report1("PlayContextAt", client.PlayContextAt(cfg.DiagnoseDeviceID, cfg.DiagnosePlayCtx, cfg.DiagnosePlayURI))
	case cfg.DiagnoseDeviceID != "" && cfg.DiagnosePlayCtx != "":
		return report1("PlayContext", client.PlayContext(cfg.DiagnoseDeviceID, cfg.DiagnosePlayCtx))
	case cfg.DiagnoseDeviceID != "" && cfg.DiagnosePlayURI != "":
		return report1("PlayURIs", client.PlayURIs(cfg.DiagnoseDeviceID, []string{cfg.DiagnosePlayURI}))
	case cfg.DiagnoseDeviceID != "":
		return runDiagnose(client, cfg.DiagnoseDeviceID)
	case cfg.DiagnosePlaylists:
		return runDiagnosePlaylists(client)
	case cfg.DiagnoseSearch != "":
		return runDiagnoseSearch(client, cfg.DiagnoseSearch)
	case cfg.DiagnoseArt != "":
		return runDiagnoseArt(cfg.DiagnoseArt, cfg.ExperimentalKittyArt)
	case cfg.DiagnosePlaylistID != "":
		return runDiagnosePlaylistTracks(client, cfg.DiagnosePlaylistID)
	case cfg.DiagnoseQueue:
		return runDiagnoseQueue(client)
	case cfg.DiagnoseAddQueue != "":
		return report1("AddToQueue", client.AddToQueue(cfg.DiagnoseAddQueue))
	case cfg.DiagnoseSkip:
		return report1("Next", client.Next())
	case cfg.DiagnoseAutoplay != "":
		return runDiagnoseAutoplay(client, cfg.DiagnoseAutoplay)
	case cfg.DiagnoseRecent:
		tracks, err := client.GetRecentlyPlayed(10)
		if err != nil {
			return err
		}
		for _, t := range tracks {
			fmt.Printf("recent: %-24s %s — %v %v\n", t.ID, t.Name, t.Artists, t.ArtistIDs)
		}
		return nil
	case cfg.DiagnoseTopTracks != "":
		tracks, err := client.GetArtistTopTracks(cfg.DiagnoseTopTracks)
		if err != nil {
			return err
		}
		for _, t := range tracks {
			fmt.Printf("top: %-24s %s — %v\n", t.ID, t.Name, t.Artists)
		}
		return nil
	case cfg.DiagnoseMyTop:
		tracks, err := client.GetMyTopTracks(10)
		if err != nil {
			return err
		}
		for _, t := range tracks {
			fmt.Printf("mytop: %-24s %s — %v\n", t.ID, t.Name, t.Artists)
		}
		return nil
	case cfg.DiagnoseRecommend != "":
		tracks, err := client.GetRecommendations(cfg.DiagnoseRecommend, 5)
		if err != nil {
			return err
		}
		for _, t := range tracks {
			fmt.Printf("rec: %-24s %s — %v\n", t.ID, t.Name, t.Artists)
		}
		return nil
	}

	if cfg.Once {
		return runOnce(client, cfg)
	}

	m := ui.New(client, cfg)
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// ensureToken loads a cached token and refreshes it if needed, or runs the
// full browser login flow if none exists / it's dead / --login was passed.
func ensureToken(cfg config.Config) (*spotifyauth.Token, error) {
	if !cfg.ForceLogin {
		cached, err := spotifyauth.LoadToken()
		if err != nil {
			return nil, err
		}
		if cached != nil {
			if fresh, err := spotifyauth.EnsureFresh(cfg.ClientID, cached); err == nil {
				return fresh, nil
			}
			fmt.Fprintln(os.Stderr, "cached token invalid, starting login...")
		}
	}
	return spotifyauth.Login(cfg.ClientID, cfg.CallbackPort)
}

// runDiagnose compares the three device-targeting approaches the Spotify
// Web API docs suggest (plan section 7), against a real device ID, to find
// out which one (if any) actually works against this account — spotify_player
// hit a 404 on the standalone transfer endpoint (Test C) despite the device
// being listed by GetDevices().
func runDiagnose(client *spotifyapi.Client, deviceID string) error {
	report := func(name string, err error) {
		if err == nil {
			fmt.Printf("%-45s OK\n", name)
			return
		}
		fmt.Printf("%-45s FAIL: %v\n", name, err)
	}

	report("Test A: PUT /me/player/play?device_id=", client.PlayWithDeviceQuery(deviceID))
	report("Test B: PUT /me/player/play {device_id}", client.PlayWithDeviceBody(deviceID))
	report("Test C: PUT /me/player {device_ids,play}", client.TransferPlayback(deviceID, true))
	return nil
}

func report1(name string, err error) error {
	if err == nil {
		fmt.Printf("%s: OK\n", name)
		return nil
	}
	fmt.Printf("%s: FAIL: %v\n", name, err)
	return nil
}

func runDiagnosePlaylists(client *spotifyapi.Client) error {
	playlists, err := client.GetPlaylists(50)
	if err != nil {
		return err
	}
	for _, p := range playlists {
		fmt.Printf("%-24s %-30s tracks=%d owner=%s\n", p.ID, p.Name, p.TracksTotal, p.OwnerName)
	}
	return nil
}

func runDiagnoseSearch(client *spotifyapi.Client, query string) error {
	results, err := client.SearchTracks(query, 20)
	if err != nil {
		return err
	}
	for _, t := range results.Tracks {
		fmt.Printf("spotify:track:%-24s %-30s %v\n", t.ID, t.Name, t.Artists)
	}
	return nil
}

func runDiagnosePlaylistTracks(client *spotifyapi.Client, playlistID string) error {
	tracks, err := client.GetPlaylistTracks(playlistID)
	if err != nil {
		return err
	}
	for _, t := range tracks {
		fmt.Printf("spotify:track:%-24s %-30s %v\n", t.ID, t.Name, t.Artists)
	}
	return nil
}

func runDiagnoseArt(imageURL string, useKitty bool) error {
	art, err := albumart.Render(imageURL, 8, 4, useKitty)
	if err != nil {
		return err
	}
	fmt.Println(art)
	return nil
}

func runDiagnoseAutoplay(client *spotifyapi.Client, seedURI string) error {
	uid, err := client.CurrentUserID()
	if err != nil {
		return fmt.Errorf("get user id: %w", err)
	}
	token, err := client.AccessToken()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ids, err := spotifyradio.AutoplayTracks(ctx, uid, token, seedURI, nil)
	if err != nil {
		return err
	}
	for _, id := range ids {
		fmt.Printf("autoplay: spotify:track:%s\n", id)
	}
	fmt.Printf("total: %d\n", len(ids))
	return nil
}

func runDiagnoseQueue(client *spotifyapi.Client) error {
	state, err := client.GetPlaybackState()
	if err != nil {
		return err
	}
	if state != nil {
		fmt.Printf("current: %-24s %s  (repeat=%s shuffle=%v playing=%v)\n",
			state.Item.ID, state.Item.Name, state.RepeatState, state.ShuffleState, state.IsPlaying)
		fmt.Printf("context: %s  device: %s (id=%s active=%v)\n", state.ContextURI, state.Device.Name, state.Device.ID, state.Device.IsActive)
	} else {
		fmt.Println("current: (nothing playing)")
	}
	queue, err := client.GetQueue()
	if err != nil {
		return err
	}
	for i, t := range queue {
		fmt.Printf("queue %2d: %-24s %s\n", i, t.ID, t.Name)
	}
	return nil
}

func runOnce(client *spotifyapi.Client, cfg config.Config) error {
	if cfg.ShowDevices {
		devices, err := client.GetDevices()
		if err != nil {
			return err
		}
		for _, d := range devices {
			fmt.Printf("%-12s %-25s active=%-5v vol=%d%%\n", d.ID, d.Name, d.IsActive, d.VolumePercent)
		}
		return nil
	}

	state, err := client.GetPlaybackState()
	if err != nil {
		return err
	}
	if state == nil {
		fmt.Println("nothing playing")
		return nil
	}
	fmt.Printf("playing=%v  %s — %v  (%d/%d ms)  device=%s vol=%d%%\n",
		state.IsPlaying, state.Item.Name, state.Item.Artists,
		state.ProgressMs, state.Item.DurationMs, state.Device.Name, state.Device.VolumePercent)
	return nil
}
