package spotifyauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TokenPath returns ~/.config/spotify-tui-go/token.json.
func TokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "spotify-tui-go", "token.json"), nil
}

// LoadToken reads the persisted token, if any. Returns (nil, nil) if no
// token file exists yet (not an error — first run).
func LoadToken() (*Token, error) {
	path, err := TokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read token file: %w", err)
	}
	var tok Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("parse token file: %w", err)
	}
	return &tok, nil
}

// SaveToken persists tok to disk, creating the config directory if needed.
func SaveToken(tok Token) error {
	path, err := TokenPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// EnsureFresh returns a token guaranteed not to be within 60s of expiry,
// refreshing and persisting a new one via the refresh_token grant if
// needed. If the refresh token itself is dead (invalid_grant), the caller
// must fall back to a full browser Login.
func EnsureFresh(clientID string, tok *Token) (*Token, error) {
	if !tok.Expired() {
		return tok, nil
	}
	fresh, err := RefreshAccessToken(clientID, tok.RefreshToken)
	if err != nil {
		if isInvalidGrant(err) {
			return nil, fmt.Errorf("refresh token no longer valid, re-login required: %w", err)
		}
		return nil, err
	}
	if err := SaveToken(fresh); err != nil {
		return nil, fmt.Errorf("persist refreshed token: %w", err)
	}
	return &fresh, nil
}

// Login runs the full browser-based PKCE flow: generates verifier/challenge/
// state, opens the browser, waits for the callback, exchanges the code, and
// persists the resulting token.
func Login(clientID string, port int) (*Token, error) {
	verifier, err := GenerateVerifier()
	if err != nil {
		return nil, err
	}
	state, err := GenerateState()
	if err != nil {
		return nil, err
	}
	challenge := ChallengeFromVerifier(verifier)
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	authURL := BuildAuthorizeURL(clientID, redirectURI, challenge, state)

	// Start the callback server before opening the browser so we can't
	// miss a fast redirect.
	codeCh := make(chan struct {
		code string
		err  error
	}, 1)
	go func() {
		c, err := RunCallbackServer(port, state)
		codeCh <- struct {
			code string
			err  error
		}{c, err}
	}()

	if err := OpenBrowser(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "could not auto-open browser, open manually: %s\n", authURL)
	} else {
		fmt.Fprintln(os.Stderr, "opened browser for Spotify login — waiting for you to approve...")
	}

	res := <-codeCh
	if res.err != nil {
		return nil, res.err
	}

	tok, err := ExchangeCode(clientID, redirectURI, res.code, verifier)
	if err != nil {
		return nil, err
	}
	if err := SaveToken(tok); err != nil {
		return nil, fmt.Errorf("persist token: %w", err)
	}
	return &tok, nil
}
