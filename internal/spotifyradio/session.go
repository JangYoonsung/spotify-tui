// Package spotifyradio reaches Spotify's autoplay/radio recommendations —
// which the Web API doesn't expose — via go-librespot's internal spclient
// REST endpoints, authenticated with this app's own Web API access token
// (needs the streaming scope). It's the real algorithm behind the official
// clients' autoplay, and unlike GET /recommendations it works for
// development-mode apps.
//
// It deliberately does NOT use go-librespot's `session` package: that pulls
// in the audio player (vorbis/flac), which needs CGo and system codec
// libraries. Instead it reassembles just the auth chain that leads to an
// spclient — accesspoint token auth → login5 → spclient — from the
// CGo-free subpackages. Keep it that way; importing `session` (or `player`)
// silently reintroduces the CGo dependency and breaks `CGO_ENABLED=0`.
package spotifyradio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/devgianlu/go-librespot/ap"
	"github.com/devgianlu/go-librespot/apresolve"
	"github.com/devgianlu/go-librespot/login5"
	credentialspb "github.com/devgianlu/go-librespot/proto/spotify/login5/v3/credentials"
	"github.com/devgianlu/go-librespot/spclient"
)

// newDeviceID returns a random 20-byte device id as hex (librespot requires
// exactly 20 bytes).
func newDeviceID() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// newSpclient reproduces go-librespot's session assembly up to the spclient,
// authenticating the accesspoint with a Spotify Web API access token. It
// stops before the audio/dealer/mercury pieces the session package adds —
// autoplay only needs the spclient REST surface.
func newSpclient(ctx context.Context, username, accessToken string) (*spclient.Spclient, error) {
	deviceID, err := newDeviceID()
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	log := nopLog{}

	clientToken, err := retrieveClientToken(client, deviceID)
	if err != nil {
		return nil, fmt.Errorf("client token: %w", err)
	}

	resolver := apresolve.NewApResolver(log, client)

	apAddr, err := resolver.GetAccesspoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve accesspoint: %w", err)
	}
	accessPoint := ap.NewAccesspoint(log, apAddr, deviceID)
	if err := accessPoint.ConnectSpotifyToken(ctx, username, accessToken); err != nil {
		return nil, fmt.Errorf("accesspoint auth (needs streaming scope): %w", err)
	}

	l5 := login5.NewLogin5(log, client, deviceID, clientToken)
	if err := l5.Login(ctx, &credentialspb.StoredCredential{
		Username: accessPoint.Username(),
		Data:     accessPoint.StoredCredentials(),
	}); err != nil {
		return nil, fmt.Errorf("login5: %w", err)
	}

	spAddr, err := resolver.GetSpclient(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve spclient: %w", err)
	}
	sp, err := spclient.NewSpclient(ctx, log, client, spAddr, l5.AccessToken(), deviceID, clientToken)
	if err != nil {
		return nil, fmt.Errorf("spclient: %w", err)
	}
	return sp, nil
}
