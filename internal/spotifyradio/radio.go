package spotifyradio

import (
	"context"
	"fmt"
	"strings"

	librespot "github.com/devgianlu/go-librespot"
	playerpb "github.com/devgianlu/go-librespot/proto/spotify/player"
)

// AutoplayTracks returns Spotify's autoplay/radio track IDs seeded by
// contextURI (e.g. "spotify:playlist:<id>" or "spotify:track:<id>"),
// excluding recentTrackURIs from the seed. IDs are bare base62 track IDs,
// ready for the Web API. username is the Spotify user id; accessToken must
// carry the streaming scope.
//
// This opens a fresh librespot session per call — fine for autoplay, which
// fires at most once per track. Keep the call off the UI goroutine (it does
// network I/O and an AP handshake, ~1-2s).
func AutoplayTracks(ctx context.Context, username, accessToken, contextURI string, recentTrackURIs []string) ([]string, error) {
	sp, err := newSpclient(ctx, username, accessToken)
	if err != nil {
		return nil, err
	}

	resolved, err := sp.ContextResolveAutoplay(ctx, &playerpb.AutoplayContextRequest{
		ContextUri:     &contextURI,
		RecentTrackUri: recentTrackURIs,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve autoplay: %w", err)
	}

	var ids []string
	seen := map[string]bool{}
	for _, page := range resolved.GetPages() {
		for _, t := range page.GetTracks() {
			id, ok := strings.CutPrefix(t.GetUri(), "spotify:track:")
			if !ok || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// nopLog is a silent librespot.Logger — the widget surfaces failures through
// its own error line, and librespot's chatter would corrupt the TUI.
type nopLog struct{}

func (nopLog) Tracef(string, ...any)                    {}
func (nopLog) Debugf(string, ...any)                    {}
func (nopLog) Infof(string, ...any)                     {}
func (nopLog) Warnf(string, ...any)                     {}
func (nopLog) Errorf(string, ...any)                    {}
func (nopLog) Trace(...any)                             {}
func (nopLog) Debug(...any)                             {}
func (nopLog) Info(...any)                              {}
func (nopLog) Warn(...any)                              {}
func (nopLog) Error(...any)                             {}
func (n nopLog) WithField(string, any) librespot.Logger { return n }
func (n nopLog) WithError(error) librespot.Logger       { return n }
