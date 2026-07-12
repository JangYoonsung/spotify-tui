package spotifyauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"time"
)

// RunCallbackServer starts a temporary localhost server on port, waits for
// Spotify's OAuth redirect to hit "/callback", validates the state
// parameter, and returns the authorization code. Times out after 120s if
// the browser flow is abandoned.
func RunCallbackServer(port int, expectedState string) (code string, err error) {
	type result struct {
		code string
		err  error
	}
	resultCh := make(chan result, 1)

	// send is best-effort: RunCallbackServer only ever reads resultCh once.
	// If the browser hits /callback a second time (e.g. a tab refresh), a
	// plain unconditional send would block forever on the full buffer —
	// leaking this handler goroutine (and the underlying connection) for
	// the rest of the process's life. The select/default makes a repeat
	// hit a no-op instead.
	send := func(res result) {
		select {
		case resultCh <- res:
		default:
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errParam := q.Get("error"); errParam != "" {
			send(result{err: fmt.Errorf("spotify authorization denied: %s", errParam)})
			_, _ = fmt.Fprint(w, "Authorization failed — you can close this tab.")
			return
		}
		if q.Get("state") != expectedState {
			send(result{err: fmt.Errorf("state mismatch — possible CSRF, aborting")})
			_, _ = fmt.Fprint(w, "Authorization failed (state mismatch) — you can close this tab.")
			return
		}
		c := q.Get("code")
		if c == "" {
			send(result{err: fmt.Errorf("no code in callback")})
			_, _ = fmt.Fprint(w, "Authorization failed (no code) — you can close this tab.")
			return
		}
		send(result{code: c})
		_, _ = fmt.Fprint(w, "Logged in — you can close this tab.")
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("listen on %s: %w", addr, err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck // shut down explicitly below

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx) //nolint:errcheck
	}()

	select {
	case res := <-resultCh:
		return res.code, res.err
	case <-time.After(120 * time.Second):
		return "", fmt.Errorf("timed out waiting for Spotify login (120s)")
	}
}

// OpenBrowser launches the system default browser at url (macOS `open`).
func OpenBrowser(url string) error {
	return exec.Command("open", url).Start()
}
