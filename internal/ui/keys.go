package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit         key.Binding
	PlayPause    key.Binding
	Next         key.Binding
	Previous     key.Binding
	VolumeUp     key.Binding
	VolumeDown   key.Binding
	Shuffle      key.Binding
	Repeat       key.Binding
	Refresh      key.Binding
	Search       key.Binding
	Devices      key.Binding
	QueueAdd     key.Binding
	PlayPlaylist key.Binding
	Queue        key.Binding
	Recent       key.Binding
	Like         key.Binding
	Help         key.Binding
	Escape       key.Binding
	Up           key.Binding
	Down         key.Binding
	Enter        key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	PlayPause: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "play/pause"),
	),
	Next: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "next"),
	),
	Previous: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "prev"),
	),
	VolumeUp: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+", "vol+"),
	),
	VolumeDown: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "vol-"),
	),
	Shuffle: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "shuffle"),
	),
	Repeat: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "repeat"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Devices: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "devices"),
	),
	QueueAdd: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "queue"),
	),
	PlayPlaylist: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "play playlist"),
	),
	Queue: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "queue view"),
	),
	Recent: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "history"),
	),
	Like: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "♥ like"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "more"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
}

// Display-only enter variants: what enter does differs per screen/focus, but
// a key.Binding carries one fixed help string — these exist purely so
// bubbles/help can show the right verb, they're never matched against.
var (
	enterOpenTracks   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open tracks"))
	enterPlayTrack    = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play track"))
	enterSearchPlay   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "search/play"))
	enterSwitchDevice = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "switch device"))
)

// contextKeys is the help.KeyMap for whichever screen/focus is active —
// bubbles/help renders the footer from these instead of a hand-maintained
// string per screen (which kept drifting from the real bindings).
type contextKeys struct {
	short []key.Binding
	full  [][]key.Binding
}

func (c contextKeys) ShortHelp() []key.Binding  { return c.short }
func (c contextKeys) FullHelp() [][]key.Binding { return c.full }

func keysFor(s screen, focusTracks bool) contextKeys {
	controls := []key.Binding{keys.PlayPause, keys.Next, keys.Previous, keys.VolumeUp, keys.VolumeDown, keys.Shuffle, keys.Repeat, keys.Like}
	global := []key.Binding{keys.Search, keys.Devices, keys.Queue, keys.Recent, keys.Refresh, keys.Help, keys.Quit}

	switch s {
	case screenSearch:
		return contextKeys{
			short: []key.Binding{enterSearchPlay, keys.QueueAdd, keys.Up, keys.Down, keys.Escape, keys.Quit},
			full: [][]key.Binding{
				{enterSearchPlay, keys.QueueAdd, keys.Up, keys.Down},
				{keys.Escape, keys.Help, keys.Quit},
			},
		}
	case screenDevices:
		return contextKeys{
			short: []key.Binding{enterSwitchDevice, keys.Up, keys.Down, keys.Escape, keys.Quit},
			full: [][]key.Binding{
				{enterSwitchDevice, keys.Up, keys.Down},
				{keys.Escape, keys.Help, keys.Quit},
			},
		}
	case screenQueue, screenRecent:
		return contextKeys{
			short: []key.Binding{enterPlayTrack, keys.QueueAdd, keys.Up, keys.Down, keys.Escape, keys.Quit},
			full: [][]key.Binding{
				{enterPlayTrack, keys.QueueAdd, keys.Up, keys.Down},
				{keys.Escape, keys.Help, keys.Quit},
			},
		}
	default:
		if focusTracks {
			return contextKeys{
				short: []key.Binding{enterPlayTrack, keys.QueueAdd, keys.Up, keys.Down, keys.Escape, keys.Help, keys.Quit},
				full: [][]key.Binding{
					{enterPlayTrack, keys.QueueAdd, keys.Up, keys.Down, keys.Escape},
					controls,
					global,
				},
			}
		}
		return contextKeys{
			short: []key.Binding{keys.PlayPause, enterOpenTracks, keys.PlayPlaylist, keys.Search, keys.Help, keys.Quit},
			full: [][]key.Binding{
				{enterOpenTracks, keys.PlayPlaylist, keys.Up, keys.Down},
				controls,
				global,
			},
		}
	}
}
