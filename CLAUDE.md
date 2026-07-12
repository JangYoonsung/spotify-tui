# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A TUI now-playing widget for Spotify (Bubble Tea), controlled via the official Web API with OAuth PKCE. Designed to run as a docked panel inside cmux (like `~/dev/cmux-orchestrator`), which affects design decisions like the `.env` lookup order below.

## Commands

```bash
go build -o bin/spotify-tui ./cmd/spotify-tui   # build
go vet ./... && golangci-lint run ./...          # lint
go test ./...                                    # unit tests (pure rendering/state logic only)
./bin/spotify-tui --once                         # headless check: print playback state once, exit
./bin/spotify-tui --once --diagnose-playlists            # list playlists
./bin/spotify-tui --once --diagnose-search "<query>"     # search tracks
./bin/spotify-tui --once --diagnose-playlist-tracks <id> # list a playlist's tracks
./bin/spotify-tui --once --diagnose-art <image-url>      # print rendered art
./bin/spotify-tui --login                        # force browser PKCE login even if cached token exists
```

`--once`-family flags are the way to verify API-layer changes without launching the alt-screen TUI — this project's sandbox testing has repeatedly found that synthetic keypresses don't reliably reach a real bubbletea program, so interactive keybinding changes need a human testing in a real terminal.

## Architecture

Five packages:

- **`internal/config`** — flags + `.env` loading. `.env` is looked up first in cwd, then `~/.config/spotify-tui-go/.env`; the second path matters because cmux launches the tool with an arbitrary cwd. Real env vars always win over the file.
- **`internal/spotifyauth`** — PKCE OAuth flow (verifier/challenge in `pkce.go`, local callback server in `callback.go`, token exchange/refresh in `oauth.go`). Token persisted at `~/.config/spotify-tui-go/token.json` (0600). `EnsureFresh` refreshes within 60s of expiry; `invalid_grant` means the refresh token is dead and a full browser `Login` is required. Scopes: see README.
- **`internal/spotifyapi`** — Web API client. Decoupled from auth via the `TokenSource` closure created in `main.go`, so this package knows nothing about token storage. `main.go`'s `tokenSource` closure is mutex-guarded — bubbletea dispatches multiple `tea.Cmd`s concurrently (e.g. `Init()`'s `tea.Batch`), and Spotify's PKCE refresh rotates the refresh token, so an unsynchronized race there can persist a dead token to disk.
- **`internal/albumart`** — fetches + renders album art via [go-termimg](https://github.com/blacktop/go-termimg). Default protocol is Halfblocks (plain ANSI text, safe to lay out with the rest of the widget). `--experimental-kitty-art` switches to `termimg.Auto` (real Kitty/Sixel/iTerm2) — tested against a real terminal and found to desync bubbletea's line-based redraw (mitigated with an empirical newline-padding hack, not a real fix). Don't extend the graphics-protocol path without testing on a real terminal; this codebase's sandbox can't verify it visually.
- **`internal/ui`** — standard Elm architecture (`model.go` / `update.go` / `view.go`), box rendering in `widget.go`/`listscreen.go`, lipgloss styles in `styles.go`. Polls playback on `cfg.PollInterval` ticks.

### UI state machine invariants (update.go)

Control keys do an **optimistic update** (flip `m.state` locally) and set `actionInFlight`. While `actionInFlight` is true, tick-driven polls are skipped so a stale poll response can't overwrite the optimistic state — but the tick is always re-armed. On `actionResultMsg` success an immediate refresh is issued; on failure the error goes to `m.lastErr`. Any function returning a `tea.Cmd` for an in-flight action must never return `nil` on a bail-out path — `Model` has value semantics, so a bail-out inside that function can't reset the caller's `actionInFlight`, and a `nil` `tea.Cmd` there means it silently deadlocks: `actionInFlight` stays true forever, which also stops `tickMsg`'s polling and ignores every further keypress. Always route failures through `actionCmd` so they resolve to a real `actionResultMsg`.

### Home screen layout (v3)

There's no separate "playlists screen" or "track list screen" — `screenNowPlaying` renders the now-playing box, an always-visible playlists list, and (once a playlist is picked) that playlist's tracks, all stacked on one screen. `Model.focusTracks` decides whether up/down/enter drive the playlists list or the tracks list; both stay rendered regardless of focus. Secondary screens: `screenSearch` (text input + results) and `screenDevices` (`d`; enter transfers playback to the selected device). The last-opened playlist and last-played track are persisted best-effort to `~/.config/spotify-tui-go/state.json` (`config.UIState`) and restored in `ui.New`/`Init` so the tracks box (cursor included) survives the cmux dock restarting the widget. Track restore is one-shot (`restoreTrackID` cleared on first `playlistTracksResultMsg`); nothing auto-plays.

### Playback API notes (playback.go / playlists.go)

- Device targeting: `PlayWithDeviceQuery` (device_id as a query param on `/me/player/play`) is the confirmed-working approach, verified empirically against real devices. `PlayWithDeviceBody` (device_id in the JSON body) 403s — not a documented field there. `PlayContext`/`PlayURIs` follow the query-param pattern with `context_uri`/`uris` in the body instead. The device picker's "switch device" also uses `PlayWithDeviceQuery` — note its semantics are "play on this device", so switching while paused starts playback.
- `AddToQueue` (`POST /me/player/queue?uri=`) needs an active device (404 → `ErrNoActiveDevice`), like the other control endpoints. Queueing is track-only — device rows have no `trackURI`, which is what the `a`-key guard in `handleListKey` relies on.
- Playlist tracks live at `GET /playlists/<id>/items` (not `/tracks` — renamed in Spotify's February 2026 Web API migration, along with the response's `track` field becoming `item`). This app is also in Development Mode (no Extended Quota Mode), which limits this endpoint to playlists you created or collaborate on.
- `GET /me/playlists`'s `tracks` field returns `null` post-migration — don't display a "(N tracks)" count from it, it's always 0/wrong.

## Conventions

- Secrets never live in committed files; `client_id` comes only from `SPOTIFY_TUI_CLIENT_ID` (`.env` is gitignored).
- Comments explain _why_ (hidden constraints, workarounds for specific bugs, non-obvious API behavior) — not _what_ the code does. Don't add narrative comments referencing how/when something was fixed; that belongs in commit messages.
