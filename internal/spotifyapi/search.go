package spotifyapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SearchResults is trimmed to track-only search — the minimum needed for
// "find a song, play it." Album/artist/playlist search is out of v3 scope.
type SearchResults struct {
	Tracks []Track
}

type rawSearchResponse struct {
	Tracks struct {
		Items []rawTrack `json:"items"`
	} `json:"tracks"`
}

// searchLimitCap: Spotify's docs claim limit can go up to 50, but this
// account/app combination empirically 400s ("Invalid limit") on any value
// above 10 — reproduced directly with curl, not a bug in this client.
// Clamping here protects every caller uniformly.
const searchLimitCap = 10

// GetRecommendations: GET /recommendations?seed_tracks=<id>&limit=<n>.
// Availability is uncertain for this app: Spotify removed this endpoint for
// new/development-mode apps in the Nov 2024 API restrictions — probe with
// --once --diagnose-recommend before building features on it.
func (c *Client) GetRecommendations(seedTrackID string, limit int) ([]Track, error) {
	q := url.Values{
		"seed_tracks": {seedTrackID},
		"limit":       {fmt.Sprint(limit)},
	}
	resp, err := c.do(http.MethodGet, "/recommendations?"+q.Encode(), nil, "")
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
		Tracks []rawTrack `json:"tracks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse recommendations: %w", err)
	}
	tracks := make([]Track, 0, len(raw.Tracks))
	for _, t := range raw.Tracks {
		tracks = append(tracks, t.toTrack())
	}
	return tracks, nil
}

// GetArtistTopTracks: GET /artists/{id}/top-tracks. Alive for
// development-mode apps (unlike /recommendations — probe history in
// CLAUDE.md); the market defaults to the token's country.
func (c *Client) GetArtistTopTracks(artistID string) ([]Track, error) {
	resp, err := c.do(http.MethodGet, "/artists/"+artistID+"/top-tracks", nil, "")
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
		Tracks []rawTrack `json:"tracks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse artist top tracks: %w", err)
	}
	tracks := make([]Track, 0, len(raw.Tracks))
	for _, t := range raw.Tracks {
		tracks = append(tracks, t.toTrack())
	}
	return tracks, nil
}

// GetMyTopTracks: GET /me/top/tracks (needs user-top-read).
func (c *Client) GetMyTopTracks(limit int) ([]Track, error) {
	q := url.Values{"limit": {fmt.Sprint(limit)}}
	resp, err := c.do(http.MethodGet, "/me/top/tracks?"+q.Encode(), nil, "")
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
		Items []rawTrack `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse my top tracks: %w", err)
	}
	tracks := make([]Track, 0, len(raw.Items))
	for _, t := range raw.Items {
		tracks = append(tracks, t.toTrack())
	}
	return tracks, nil
}

// SearchTracks: GET /search?q=<query>&type=track&limit=<limit>.
func (c *Client) SearchTracks(query string, limit int) (SearchResults, error) {
	if limit > searchLimitCap {
		limit = searchLimitCap
	}
	q := url.Values{
		"q":     {query},
		"type":  {"track"},
		"limit": {fmt.Sprint(limit)},
	}
	resp, err := c.do(http.MethodGet, "/search?"+q.Encode(), nil, "")
	if err != nil {
		return SearchResults{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return SearchResults{}, err
	}
	if err := classifyStatus(resp, data); err != nil {
		return SearchResults{}, err
	}

	var raw rawSearchResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return SearchResults{}, fmt.Errorf("parse search results: %w", err)
	}
	tracks := make([]Track, 0, len(raw.Tracks.Items))
	for _, t := range raw.Tracks.Items {
		tracks = append(tracks, t.toTrack())
	}
	return SearchResults{Tracks: tracks}, nil
}
