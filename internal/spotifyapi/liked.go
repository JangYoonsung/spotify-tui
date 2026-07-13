package spotifyapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

const (
	savedTracksPageLimit = 50  // Spotify's max per request on /me/tracks
	savedTracksCap       = 200 // 4 sequential fetches — enough for a widget list
)

// GetSavedTracks fetches the user's Liked Songs (GET /me/tracks), following
// offset pagination up to savedTracksCap. Needs user-library-read.
func (c *Client) GetSavedTracks() ([]Track, error) {
	var tracks []Track
	for offset := 0; offset < savedTracksCap; offset += savedTracksPageLimit {
		q := url.Values{
			"limit":  {strconv.Itoa(savedTracksPageLimit)},
			"offset": {strconv.Itoa(offset)},
		}
		resp, err := c.do(http.MethodGet, "/me/tracks?"+q.Encode(), nil, "")
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
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
			return nil, fmt.Errorf("parse saved tracks: %w", err)
		}
		for _, it := range raw.Items {
			if it.Track == nil {
				continue
			}
			tracks = append(tracks, it.Track.toTrack())
		}
		if len(raw.Items) < savedTracksPageLimit {
			break
		}
	}
	return tracks, nil
}

// SaveTrack likes a track (PUT /me/tracks?ids=). Needs user-library-modify.
func (c *Client) SaveTrack(trackID string) error {
	q := url.Values{"ids": {trackID}}
	return c.simpleAction(http.MethodPut, "/me/tracks?"+q.Encode())
}

// RemoveSavedTrack unlikes a track (DELETE /me/tracks?ids=).
func (c *Client) RemoveSavedTrack(trackID string) error {
	q := url.Values{"ids": {trackID}}
	return c.simpleAction(http.MethodDelete, "/me/tracks?"+q.Encode())
}

// CheckSavedTrack reports whether a track is in the user's Liked Songs
// (GET /me/tracks/contains?ids=).
func (c *Client) CheckSavedTrack(trackID string) (bool, error) {
	q := url.Values{"ids": {trackID}}
	resp, err := c.do(http.MethodGet, "/me/tracks/contains?"+q.Encode(), nil, "")
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if err := classifyStatus(resp, data); err != nil {
		return false, err
	}

	var saved []bool
	if err := json.Unmarshal(data, &saved); err != nil {
		return false, fmt.Errorf("parse saved check: %w", err)
	}
	return len(saved) > 0 && saved[0], nil
}
