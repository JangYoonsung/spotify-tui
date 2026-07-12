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
