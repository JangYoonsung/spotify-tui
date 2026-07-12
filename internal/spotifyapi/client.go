package spotifyapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.spotify.com/v1"

// ErrNoActiveDevice/ErrForbidden are typed so the UI can render a helpful
// message instead of a raw HTTP error.
var (
	ErrNoActiveDevice = errors.New("no active Spotify device — open Spotify on a device first")
	ErrForbidden      = errors.New("forbidden (need Premium, or restricted device)")
)

// TokenSource returns a valid bearer access token, refreshing it if needed.
// Implemented by a closure over spotifyauth.EnsureFresh so this package
// doesn't need to know about token storage.
type TokenSource func() (string, error)

type Client struct {
	http        *http.Client
	tokenSource TokenSource
}

func New(tokenSource TokenSource) *Client {
	return &Client{http: &http.Client{}, tokenSource: tokenSource}
}

// do issues an authenticated request against the Web API. body may be nil.
// Returns the raw response for the caller to decode/inspect status codes —
// callers are expected to close resp.Body.
func (c *Client) do(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	token, err := c.tokenSource()
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.http.Do(req)
}

// classifyStatus turns a non-2xx status into a typed/descriptive error.
// 204 is treated as success by callers directly, not passed here.
func classifyStatus(resp *http.Response, respBody []byte) error {
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNoActiveDevice, string(respBody))
	case http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrForbidden, string(respBody))
	default:
		return fmt.Errorf("spotify API error: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
}
