# spotify-tui-go

A terminal now-playing widget for Spotify, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). It talks to the official Spotify Web API using OAuth PKCE (no client secret needed) and is designed to run as a small docked panel inside cmux.

```
╭─ Spotify ──────────────────────────────── playing  ──╮
│ ▶  Song Title — Artist Name                          │
│ ████████████░░░░░░░░░░░░░░░░░░░░░░░░  1:23/3:45      │
│ Vol 60%  ·  MacBook Pro Speakers                     │
╰──────────────────────────────────────────────────────╯
╭─ Playlists ─────────────────────────────────── 1/3  ─╮
│ ▸ Discover Weekly                                    │
│   On Repeat                                          │
│   Road Trip                                          │
╰──────────────────────────────────────────────────────╯
╭─ Discover Weekly ───────────────────────────── 2/30 ─╮
│   Track One — Artist A                               │
│ ▸ Track Two — Artist B                               │
│   Track Three — Artist C                             │
╰──────────────────────────────────────────────────────╯
```

Playlists load automatically and stay visible on the main screen — no key needed to reveal them. Picking one (`enter`) opens a third box, inline on the same screen, listing that playlist's tracks (title = playlist name); picking a track plays it. `esc` closes the tracks box and returns focus to Playlists.

## Setup

1. Create an app on the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard).
2. Add `http://127.0.0.1:8942/callback` as a Redirect URI (the port must match `--port`, default `8942`).
3. Put the app's Client ID in a `.env` file:

   ```
   SPOTIFY_TUI_CLIENT_ID=your_client_id_here
   ```

   The file is looked up first in the current directory, then in `~/.config/spotify-tui-go/.env`. The second location is what you want when launching from a cmux dock, where the working directory is arbitrary. A real environment variable always wins over the file.

4. Build and run:

   ```bash
   go build -o bin/spotify-tui ./cmd/spotify-tui
   ./bin/spotify-tui
   ```

On first run a browser opens for Spotify login. The token is cached at `~/.config/spotify-tui-go/token.json` (mode 0600) and refreshed automatically; you won't see the browser again unless the refresh token dies or you pass `--login`.

## Usage

```bash
spotify-tui                        # TUI widget (alt screen), polls every 3s
spotify-tui --once                 # print playback state once as plain text, exit
spotify-tui --once --show-devices  # list available devices, exit
spotify-tui --login                # force the browser login flow
spotify-tui --poll-interval 5s     # change the polling cadence
spotify-tui --port 9000            # OAuth callback port (must match the dashboard redirect URI)
spotify-tui --experimental-kitty-art  # real Kitty/Sixel/iTerm2 image instead of ANSI half-block art (see Notes)
```

### Keys

| Key              | Action                                                                            |
| ---------------- | --------------------------------------------------------------------------------- |
| `space`          | play/pause                                                                        |
| `n` / `p`        | next / previous track                                                             |
| `+` / `-`        | volume up/down                                                                    |
| `s`              | toggle shuffle                                                                    |
| `r`              | cycle repeat mode                                                                 |
| `↑`/`k`, `↓`/`j` | move selection — playlists box by default, or the tracks box once open            |
| `enter`          | on a playlist: open its tracks (inline, same screen). on a track: play it         |
| `esc`            | close the open tracks box and return focus to the playlists box                   |
| `/`              | open search (type a query, `enter` to search, then `↑↓`/`enter` to play a result) |
| `d`              | open the device list; `enter` switches playback to the selected device           |
| `a`              | in search results or a tracks list: add the selected track to the queue          |
| `R`              | force refresh (playback state + playlists)                                        |
| `q` / `ctrl+c`   | quit                                                                              |

Controlling playback requires Spotify Premium.

## Requirements

- Go 1.26+
- A Spotify account (Premium required for the control endpoints, not for viewing)

## Notes

- Requested OAuth scopes: `user-read-playback-state`, `user-modify-playback-state`, `user-read-currently-playing`, `playlist-read-private`, `playlist-read-collaborative`.
- "nothing playing" is a normal idle state (the API returns 204), not an error.
- Secrets never live in the repo: `.env` is gitignored and only `SPOTIFY_TUI_CLIENT_ID` is read from it.
- Playlist tracks use `GET /playlists/<id>/items` — Spotify renamed this from `/tracks` (and the response's `track` field to `item`) in a February 2026 Web API migration; the old path 403s even for a playlist you own with valid scopes. This app is also in Spotify's Development Mode (not Extended Quota Mode), which limits `GET /playlists/<id>/items` to playlists you created or collaborate on — other users' playlists and Spotify-owned/algorithmic playlists will 403/404 regardless.
- `--experimental-kitty-art` is genuinely experimental: album art renders as a real bitmap via [go-termimg](https://github.com/blacktop/go-termimg)'s auto-detected graphics protocol, but the image occupies real terminal rows that bubbletea's string-based line-diffing renderer doesn't account for, causing redraw desync (mitigated with an empirical newline-padding hack, not a real fix). ANSI half-block art (the default) is just colored text and doesn't have this problem — it's the stable choice.
