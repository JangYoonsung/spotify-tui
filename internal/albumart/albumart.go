// Package albumart renders album art for embedding in a bubbletea View()
// string, via github.com/blacktop/go-termimg.
//
// Protocol is forced to Halfblocks (plain ANSI-colored unicode text) by
// default rather than using termimg's Auto-detection of Kitty/Sixel/iTerm2.
// Reason: this app's box layout (lipgloss.JoinHorizontal, then each
// resulting line individually passed through boxRow, which pads/truncates
// on visual width) treats art as ordinary line-oriented text. A real
// graphics-protocol escape sequence is not line-oriented — splitting it on
// '\n' and re-wrapping each fragment through boxRow's truncation could
// corrupt it in ways that show no error, only a broken/missing image, which
// isn't verifiable from raw bytes without a real terminal. Halfblocks output
// is just colored text, so it's fully compatible with the existing
// truncation-safe pipeline and directly verifiable.
//
// Set UseKitty (wired from --experimental-kitty-art) to opt into
// termimg.Auto instead — unverified, at the caller's own risk.
package albumart

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	termimg "github.com/blacktop/go-termimg"

	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

// PickImageURL selects the smallest image whose dimensions are still
// >= (targetW, targetH) — no point fetching more pixels than we'll render.
// If none meet the target, it falls back to the *largest* available image
// instead of the smallest: every candidate is already too small at that
// point, so minimizing upscale loss beats minimizing bytes. "" if images is
// empty (tracks without artwork are a normal case, not an error).
func PickImageURL(images []spotifyapi.Image, targetW, targetH int) string {
	if len(images) == 0 {
		return ""
	}
	best := images[0]
	bestOK := best.Width >= targetW && best.Height >= targetH
	for _, img := range images[1:] {
		ok := img.Width >= targetW && img.Height >= targetH
		switch {
		case ok && !bestOK:
			best, bestOK = img, true
		case ok && bestOK && img.Width < best.Width:
			best = img // both meet target: prefer the smaller (less to fetch)
		case !ok && !bestOK && img.Width > best.Width:
			best = img // neither meets target: prefer the larger (less upscale loss)
		}
	}
	return best.URL
}

// Render fetches imageURL and renders it at cols x rows terminal cells.
// useKitty selects termimg.Auto (real graphics protocol if the terminal
// supports one, otherwise its own halfblock fallback) instead of forcing
// Halfblocks — see package doc for why that's opt-in only.
func Render(imageURL string, cols, rows int, useKitty bool) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("no image URL")
	}

	resp, err := httpClient.Get(imageURL) //nolint:gosec,noctx // fixed Spotify CDN URL from the API response, not user input
	if err != nil {
		return "", fmt.Errorf("fetch image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch image: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	img, err := termimg.From(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	protocol := termimg.Halfblocks
	if useKitty {
		protocol = termimg.Auto
	}
	// Dithering trades resolution for perceived detail via noise, but at
	// this few cells it just reads as washed-out speckle rather than a
	// recognizable image — flat nearest/area sampling looks cleaner at
	// this scale.
	img.Size(cols, rows).Protocol(protocol).Dither(false)

	out, err := img.Render()
	if err != nil {
		return "", fmt.Errorf("render image: %w", err)
	}
	return out, nil
}
