package spotifyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const baseURL = "https://api.spotify.com/v1"

// ErrNoActiveDevice/ErrForbidden are typed so the UI can render a helpful
// message instead of a raw HTTP error.
var (
	ErrNoActiveDevice = errors.New("no active Spotify device — open Spotify on a device first")
	ErrForbidden      = errors.New("forbidden (need Premium, or restricted device)")
	ErrRateLimited    = errors.New("rate limited")
)

// RateLimitError carries the Retry-After delay from a 429 so the UI can back
// off exactly that long instead of hammering the API.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}
func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

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
	// Background context: request lifetime is bounded by c.http's timeout;
	// the TUI has no per-call cancellation to thread through here.
	req, err := http.NewRequestWithContext(context.Background(), method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.http.Do(req)
}

// AccessToken exposes the current bearer token. The go-librespot autoplay
// path (internal/spotifyradio) authenticates its own session with the raw
// token — this hands it over without leaking token storage, which stays
// behind the TokenSource closure.
func (c *Client) AccessToken() (string, error) {
	return c.tokenSource()
}

// CurrentUserID fetches GET /me and returns the Spotify user id (needed as
// the username for the autoplay librespot session).
func (c *Client) CurrentUserID() (string, error) {
	resp, err := c.do(http.MethodGet, "/me", nil, "")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := classifyStatus(resp, data); err != nil {
		return "", err
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &me); err != nil {
		return "", fmt.Errorf("parse /me: %w", err)
	}
	return me.ID, nil
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
	case http.StatusTooManyRequests:
		// Retry-After is in seconds; default to 10s if absent/unparseable.
		// Capped at 60s: Spotify can hand out multi-hour windows after heavy
		// abuse, but honoring those verbatim would freeze the widget for the
		// day — better to keep quietly re-probing at a light cadence and
		// recover the moment the limit lifts.
		retry := 10 * time.Second
		if s := resp.Header.Get("Retry-After"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				retry = time.Duration(n) * time.Second
			}
		}
		if retry > 60*time.Second {
			retry = 60 * time.Second
		}
		return &RateLimitError{RetryAfter: retry}
	default:
		return fmt.Errorf("spotify API error: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
}
