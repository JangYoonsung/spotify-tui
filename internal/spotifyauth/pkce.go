// Package spotifyauth implements Spotify's Authorization Code with PKCE
// flow for a local CLI/TUI: no client secret, a temporary localhost
// callback server, and a persisted refresh token so the browser flow only
// runs once.
package spotifyauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateVerifier returns a PKCE code_verifier: 32 random bytes,
// base64url-encoded without padding (43 chars — within Spotify's required
// 43-128 char range).
func GenerateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ChallengeFromVerifier derives the PKCE code_challenge (S256 method) from
// a code_verifier: SHA256 hash, base64url-encoded without padding.
func ChallengeFromVerifier(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GenerateState returns a random per-login CSRF token for the OAuth
// `state` parameter.
func GenerateState() (string, error) {
	return GenerateVerifier() // same shape requirement, reuse
}
