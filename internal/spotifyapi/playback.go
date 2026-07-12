package spotifyapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// GetPlaybackState fetches GET /me/player. Returns (nil, nil) on 204 — that
// means "nothing playing anywhere", a normal idle state, not an error.
func (c *Client) GetPlaybackState() (*PlaybackState, error) {
	resp, err := c.do(http.MethodGet, "/me/player", nil, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if err := classifyStatus(resp, data); err != nil {
		return nil, err
	}

	var raw rawPlaybackState
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse playback state: %w", err)
	}
	ps := raw.toPlaybackState()
	return &ps, nil
}

// GetQueue fetches GET /me/player/queue and returns the upcoming tracks
// (first entry = the next track). Entries that aren't tracks (podcast
// episodes lack an id) still unmarshal into rawTrack well enough for a
// display label, which is all callers use this for.
func (c *Client) GetQueue() ([]Track, error) {
	resp, err := c.do(http.MethodGet, "/me/player/queue", nil, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := classifyStatus(resp, data); err != nil {
		return nil, err
	}

	var raw struct {
		Queue []rawTrack `json:"queue"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse queue: %w", err)
	}
	tracks := make([]Track, 0, len(raw.Queue))
	for _, t := range raw.Queue {
		tracks = append(tracks, t.toTrack())
	}
	return tracks, nil
}

// GetRecentlyPlayed fetches GET /me/player/recently-played (needs the
// user-read-recently-played scope — probe with --diagnose-recent before
// relying on it; the cached token may predate the scope being requested).
func (c *Client) GetRecentlyPlayed(limit int) ([]Track, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	resp, err := c.do(http.MethodGet, "/me/player/recently-played?"+q.Encode(), nil, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := classifyStatus(resp, data); err != nil {
		return nil, err
	}

	var raw struct {
		Items []struct {
			Track *rawTrack `json:"track"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse recently played: %w", err)
	}
	tracks := make([]Track, 0, len(raw.Items))
	for _, it := range raw.Items {
		if it.Track == nil {
			continue
		}
		tracks = append(tracks, it.Track.toTrack())
	}
	return tracks, nil
}

// GetDevices fetches GET /me/player/devices.
func (c *Client) GetDevices() ([]Device, error) {
	resp, err := c.do(http.MethodGet, "/me/player/devices", nil, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := classifyStatus(resp, data); err != nil {
		return nil, err
	}

	var raw rawDevicesResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse devices: %w", err)
	}
	devices := make([]Device, 0, len(raw.Devices))
	for _, d := range raw.Devices {
		devices = append(devices, d.toDevice())
	}
	return devices, nil
}

func (c *Client) simpleAction(method, path string) error {
	resp, err := c.do(method, path, nil, "")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return classifyStatus(resp, data)
}

func (c *Client) Pause() error    { return c.simpleAction(http.MethodPut, "/me/player/pause") }
func (c *Client) Next() error     { return c.simpleAction(http.MethodPost, "/me/player/next") }
func (c *Client) Previous() error { return c.simpleAction(http.MethodPost, "/me/player/previous") }

func (c *Client) SetVolume(percent int) error {
	q := url.Values{"volume_percent": {fmt.Sprint(percent)}}
	return c.simpleAction(http.MethodPut, "/me/player/volume?"+q.Encode())
}

func (c *Client) SetShuffle(on bool) error {
	q := url.Values{"state": {fmt.Sprint(on)}}
	return c.simpleAction(http.MethodPut, "/me/player/shuffle?"+q.Encode())
}

func (c *Client) SetRepeat(mode string) error {
	q := url.Values{"state": {mode}}
	return c.simpleAction(http.MethodPut, "/me/player/repeat?"+q.Encode())
}

// AddToQueue appends a track to the active device's playback queue:
// POST /me/player/queue?uri=<uri>. Requires an active device (404 →
// ErrNoActiveDevice otherwise), same as the other control endpoints.
func (c *Client) AddToQueue(uri string) error {
	q := url.Values{"uri": {uri}}
	return c.simpleAction(http.MethodPost, "/me/player/queue?"+q.Encode())
}

// PlayResume resumes playback on the currently active device, no device
// targeting.
func (c *Client) PlayResume() error {
	return c.simpleAction(http.MethodPut, "/me/player/play")
}

// PlayWithDeviceQuery is the confirmed-working device-targeting approach
// (device_id as a query param): PUT /me/player/play?device_id=<id>, empty
// body. PlayWithDeviceBody below 403s and TransferPlayback 404s/is
// unreliable — verified empirically against real devices, not from docs.
func (c *Client) PlayWithDeviceQuery(deviceID string) error {
	q := url.Values{"device_id": {deviceID}}
	return c.simpleAction(http.MethodPut, "/me/player/play?"+q.Encode())
}

// PlayWithDeviceBody: PUT /me/player/play, JSON body {"device_id": "<id>"}.
func (c *Client) PlayWithDeviceBody(deviceID string) error {
	body, _ := json.Marshal(map[string]string{"device_id": deviceID})
	resp, err := c.do(http.MethodPut, "/me/player/play", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return classifyStatus(resp, data)
}

// playWithDeviceAndBody: PUT /me/player/play?device_id=<id>, with a JSON
// body — device_id stays in the query string (the confirmed-working slot;
// it's not a documented body field), the body carries only context_uri or
// uris, so this doesn't repeat PlayWithDeviceBody's mistake.
func (c *Client) playWithDeviceAndBody(deviceID string, body []byte) error {
	q := url.Values{"device_id": {deviceID}}
	resp, err := c.do(http.MethodPut, "/me/player/play?"+q.Encode(), bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return classifyStatus(resp, data)
}

// PlayContext plays a whole context (e.g. a playlist) on deviceID:
// PUT /me/player/play?device_id=<id>, body {"context_uri": "<uri>"}.
func (c *Client) PlayContext(deviceID, contextURI string) error {
	body, _ := json.Marshal(map[string]string{"context_uri": contextURI})
	return c.playWithDeviceAndBody(deviceID, body)
}

// PlayContextAt starts contextURI positioned at trackURI:
// PUT /me/player/play?device_id=<id>, body {"context_uri": ..., "offset":
// {"uri": ...}}. Unlike PlayURIs with a single track, playback continues
// through the rest of the context afterwards — and the queue endpoint
// returns the real upcoming tracks instead of the same track repeated
// (Spotify fills the queue with the current track when playing a bare
// single-URI "context", even with repeat off — confirmed via
// --diagnose-queue).
func (c *Client) PlayContextAt(deviceID, contextURI, trackURI string) error {
	body, _ := json.Marshal(map[string]any{
		"context_uri": contextURI,
		"offset":      map[string]string{"uri": trackURI},
	})
	return c.playWithDeviceAndBody(deviceID, body)
}

// PlayURIs plays specific track(s) on deviceID:
// PUT /me/player/play?device_id=<id>, body {"uris": ["<uri>", ...]}.
func (c *Client) PlayURIs(deviceID string, uris []string) error {
	body, _ := json.Marshal(map[string][]string{"uris": uris})
	return c.playWithDeviceAndBody(deviceID, body)
}

// TransferPlayback: standalone PUT /me/player, JSON body
// {"device_ids": ["<id>"], "play": true|false}. Unreliable in testing
// (404s intermittently); prefer PlayWithDeviceQuery/PlayContext/PlayURIs.
func (c *Client) TransferPlayback(deviceID string, play bool) error {
	body, _ := json.Marshal(map[string]any{"device_ids": []string{deviceID}, "play": play})
	resp, err := c.do(http.MethodPut, "/me/player", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return classifyStatus(resp, data)
}
