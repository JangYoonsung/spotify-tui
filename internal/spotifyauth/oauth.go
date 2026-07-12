package spotifyauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	authorizeURL = "https://accounts.spotify.com/authorize"
	tokenURL     = "https://accounts.spotify.com/api/token"

	// Scopes needed for read (now-playing/devices), control
	// (play/pause/skip/volume), and v3's playlists/search browsing —
	// requested at login time, nothing to configure on the Spotify
	// dashboard for these. playlist-read-private is required for
	// GET /me/playlists — without it that endpoint 403s with
	// "Insufficient client scope" even though the token is otherwise valid.
	//
	// The last four (recently-played / top / library read+modify) power the
	// listening-history and Liked Songs features. Adding a scope here does
	// NOT upgrade an existing cached token — a full browser --login is
	// required before the endpoints stop 403ing.
	scopes = "user-read-playback-state user-modify-playback-state user-read-currently-playing playlist-read-private playlist-read-collaborative user-read-recently-played user-top-read user-library-read user-library-modify"
)

// Token is an OAuth token with an absolute expiry, ready to persist.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (t Token) Expired() bool {
	return time.Now().Add(60 * time.Second).After(t.ExpiresAt)
}

// BuildAuthorizeURL builds the /authorize URL the user's browser is sent to.
func BuildAuthorizeURL(clientID, redirectURI, codeChallenge, state string) string {
	q := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"code_challenge_method": {"S256"},
		"code_challenge":        {codeChallenge},
		"state":                 {state},
		"scope":                 {scopes},
	}
	return authorizeURL + "?" + q.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func postForm(body url.Values) (Token, error) {
	resp, err := http.PostForm(tokenURL, body)
	if err != nil {
		return Token{}, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("read token response: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return Token{}, fmt.Errorf("parse token response: %w", err)
	}
	if tr.Error != "" {
		return Token{}, fmt.Errorf("spotify token error: %s (%s)", tr.Error, tr.ErrorDesc)
	}
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("token request failed: HTTP %d: %s", resp.StatusCode, string(data))
	}

	return Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}, nil
}

// ExchangeCode trades an authorization code for an initial token pair.
func ExchangeCode(clientID, redirectURI, code, verifier string) (Token, error) {
	tok, err := postForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	})
	if err != nil {
		return Token{}, err
	}
	if tok.RefreshToken == "" {
		return Token{}, fmt.Errorf("token exchange succeeded but no refresh_token was returned")
	}
	return tok, nil
}

// RefreshAccessToken exchanges a refresh_token for a new access token.
// Spotify's PKCE refresh response may or may not include a rotated
// refresh_token; if absent, the caller should keep using the old one.
func RefreshAccessToken(clientID, refreshToken string) (Token, error) {
	tok, err := postForm(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	})
	if err != nil {
		return Token{}, err
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken // not rotated this time, keep the existing one
	}
	return tok, nil
}

// isInvalidGrant reports whether err looks like Spotify's "invalid_grant"
// response, meaning the stored refresh token is dead and a full browser
// login is required.
func isInvalidGrant(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid_grant")
}
