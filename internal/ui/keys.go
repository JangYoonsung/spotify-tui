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

func helpLine(s screen, focusTracks bool) string {
	switch s {
	case screenSearch:
		return footerStyle.Render("type query  enter search/play  a queue  ↑/↓ move  esc back  " + keys.Quit.Help().Key + " quit")
	case screenDevices:
		return footerStyle.Render("↑/↓ move  enter switch playback  esc back  " + keys.Quit.Help().Key + " quit")
	default:
		if focusTracks {
			return footerStyle.Render("↑/↓ move  enter play track  a queue  esc back to playlists  " + keys.Quit.Help().Key + " quit")
		}
		return footerStyle.Render(
			keys.PlayPause.Help().Key + " " + keys.PlayPause.Help().Desc + "  " +
				keys.Next.Help().Key + " next  " +
				keys.Previous.Help().Key + " prev  " +
				keys.VolumeUp.Help().Key + "/" + keys.VolumeDown.Help().Key + " vol  " +
				keys.Shuffle.Help().Key + " shuffle  " +
				keys.Repeat.Help().Key + " repeat  " +
				keys.Search.Help().Key + " search  " +
				keys.Devices.Help().Key + " devices  " +
				"↑/↓ playlists  enter open tracks  " +
				keys.PlayPlaylist.Help().Key + " play playlist  " +
				keys.Quit.Help().Key + " quit",
		)
	}
}
