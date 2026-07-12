package spotifyapi

// PlaybackState mirrors GET /me/player, trimmed to the fields the UI needs.
type PlaybackState struct {
	IsPlaying    bool
	ProgressMs   int
	Device       Device
	Item         Track
	ShuffleState bool
	RepeatState  string // "off" | "track" | "context"
	// ContextURI is what playback is playing FROM, e.g.
	// "spotify:playlist:<id>" — empty for bare single-URI playback.
	ContextURI string
}

type Device struct {
	ID            string
	Name          string
	Type          string
	IsActive      bool
	VolumePercent int
}

// Image is one entry from Spotify's album/playlist images array — multiple
// sizes are returned, largest first by convention.
type Image struct {
	URL    string
	Width  int
	Height int
}

type Track struct {
	ID         string // "spotify:track:"+ID for PlayURIs
	Name       string
	Artists    []string
	ArtistIDs  []string // parallel to Artists; feeds GET /artists/{id}/top-tracks
	AlbumName  string
	DurationMs int
	Images     []Image // from album.images
}

// rawPlaybackState/rawDevice/rawTrack/rawImage mirror the actual Spotify Web
// API JSON shape (snake_case, nested artist objects) — kept separate from
// the trimmed public types above so callers never depend on API wire shape.
type rawPlaybackState struct {
	IsPlaying    bool      `json:"is_playing"`
	ProgressMs   int       `json:"progress_ms"`
	Device       rawDevice `json:"device"`
	Item         *rawTrack `json:"item"`
	ShuffleState bool      `json:"shuffle_state"`
	RepeatState  string    `json:"repeat_state"`
	Context      *struct {
		URI string `json:"uri"`
	} `json:"context"` // null for bare single-URI playback
}

type rawDevice struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	IsActive      bool   `json:"is_active"`
	VolumePercent int    `json:"volume_percent"`
}

type rawImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type rawTrack struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DurationMs int    `json:"duration_ms"`
	Artists    []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name   string     `json:"name"`
		Images []rawImage `json:"images"`
	} `json:"album"`
}

type rawDevicesResponse struct {
	Devices []rawDevice `json:"devices"`
}

func (d rawDevice) toDevice() Device {
	return Device(d)
}

func (img rawImage) toImage() Image {
	return Image(img)
}

func toImages(raw []rawImage) []Image {
	images := make([]Image, 0, len(raw))
	for _, r := range raw {
		images = append(images, r.toImage())
	}
	return images
}

func (t rawTrack) toTrack() Track {
	artists := make([]string, 0, len(t.Artists))
	artistIDs := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		artists = append(artists, a.Name)
		artistIDs = append(artistIDs, a.ID)
	}
	return Track{
		ID:         t.ID,
		Name:       t.Name,
		Artists:    artists,
		ArtistIDs:  artistIDs,
		AlbumName:  t.Album.Name,
		DurationMs: t.DurationMs,
		Images:     toImages(t.Album.Images),
	}
}

func (s rawPlaybackState) toPlaybackState() PlaybackState {
	ps := PlaybackState{
		IsPlaying:    s.IsPlaying,
		ProgressMs:   s.ProgressMs,
		Device:       s.Device.toDevice(),
		ShuffleState: s.ShuffleState,
		RepeatState:  s.RepeatState,
	}
	if s.Item != nil {
		ps.Item = s.Item.toTrack()
	}
	if s.Context != nil {
		ps.ContextURI = s.Context.URI
	}
	return ps
}
