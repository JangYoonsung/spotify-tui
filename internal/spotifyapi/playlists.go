package spotifyapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Playlist struct {
	ID          string
	Name        string
	OwnerName   string
	TracksTotal int
	Images      []Image
}

type rawPlaylist struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Owner struct {
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	Tracks struct {
		Total int `json:"total"`
	} `json:"tracks"`
	Images []rawImage `json:"images"`
}

func (p rawPlaylist) toPlaylist() Playlist {
	return Playlist{
		ID:          p.ID,
		Name:        p.Name,
		OwnerName:   p.Owner.DisplayName,
		TracksTotal: p.Tracks.Total,
		Images:      toImages(p.Images),
	}
}

type rawPlaylistsResponse struct {
	Items []rawPlaylist `json:"items"`
}

// GetPlaylists fetches the first page of GET /me/playlists (Spotify max
// limit=50). No pagination beyond the first page — a deliberate v3 scope
// limit, not a bug.
func (c *Client) GetPlaylists(limit int) ([]Playlist, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}}
	resp, err := c.do(http.MethodGet, "/me/playlists?"+q.Encode(), nil, "")
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

	var raw rawPlaylistsResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse playlists: %w", err)
	}
	playlists := make([]Playlist, 0, len(raw.Items))
	for _, p := range raw.Items {
		playlists = append(playlists, p.toPlaylist())
	}
	return playlists, nil
}

type rawPlaylistTracksResponse struct {
	Items []struct {
		// Renamed from "track" to "item" in Spotify's February 2026 Web
		// API migration (same migration that renamed the /tracks endpoint
		// itself to /items) — confirmed via curl against a real playlist.
		Item *rawTrack `json:"item"` // null for e.g. local files/removed tracks
	} `json:"items"`
}

const (
	playlistTracksPageLimit = 100  // Spotify's max per request on this endpoint
	playlistTracksCap       = 1000 // safety cap: at most 10 sequential page fetches
)

// GetPlaylistTracks fetches GET /playlists/<id>/items, following offset
// pagination until the playlist is exhausted (or playlistTracksCap, so a
// huge playlist can't stall the UI behind dozens of sequential requests).
// Entries with a null track (local files, removed tracks) are skipped
// rather than surfaced as zero-value Tracks.
//
// NOTE: this endpoint used to be /playlists/<id>/tracks — Spotify's
// February 2026 Web API migration renamed it to /items (confirmed via curl:
// /tracks 403s even for a playlist you own with valid scopes, /items
// succeeds). Development-mode apps (this one, since we haven't applied for
// Extended Quota Mode) also can't read tracks for playlists you don't own
// or collaborate on — Spotify-owned/algorithmic and other users' public
// playlists will 403/404 regardless of endpoint. Own-playlists-only is a
// real, documented limitation, not a bug to chase further here.
func (c *Client) GetPlaylistTracks(playlistID string) ([]Track, error) {
	var tracks []Track
	for offset := 0; offset < playlistTracksCap; offset += playlistTracksPageLimit {
		q := url.Values{
			"limit":  {strconv.Itoa(playlistTracksPageLimit)},
			"offset": {strconv.Itoa(offset)},
		}
		resp, err := c.do(http.MethodGet, "/playlists/"+playlistID+"/items?"+q.Encode(), nil, "")
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

		var raw rawPlaylistTracksResponse
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse playlist tracks: %w", err)
		}
		for _, entry := range raw.Items {
			if entry.Item == nil {
				continue
			}
			tracks = append(tracks, entry.Item.toTrack())
		}
		if len(raw.Items) < playlistTracksPageLimit {
			break // last page
		}
	}
	return tracks, nil
}
