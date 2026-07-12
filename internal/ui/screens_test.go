package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"github.com/jangyoonsung/spotify-tui-go/internal/spotifyapi"
)

func TestListStateSelection(t *testing.T) {
	l := newListState()
	l.setItems([]list.Item{
		listItem{label: "a", id: "1"},
		listItem{label: "b", id: "2"},
		listItem{label: "c", id: "3"},
	})

	if it, ok := l.selected(); !ok || it.id != "1" {
		t.Fatalf("selected() after setItems = (%+v, %v), want first item", it, ok)
	}

	l.selectID("3")
	if it, _ := l.selected(); it.id != "3" {
		t.Fatalf("selectID(3): selected = %q, want 3", it.id)
	}

	l.selectID("missing") // unknown id: cursor must not move
	if it, _ := l.selected(); it.id != "3" {
		t.Fatalf("selectID(missing) moved the cursor to %q", it.id)
	}

	empty := newListState()
	if _, ok := empty.selected(); ok {
		t.Fatalf("selected() on an empty list must return ok=false")
	}
}

func TestDeviceItems(t *testing.T) {
	devices := []spotifyapi.Device{
		{ID: "dev1", Name: "MacBook", Type: "Computer", IsActive: true},
		{ID: "dev2", Name: "Kitchen", Type: "Speaker"},
	}
	items := deviceItems(devices)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	first, second := items[0].(listItem), items[1].(listItem)
	if first.id != "dev1" || second.id != "dev2" {
		t.Fatalf("ids not preserved: %+v", items)
	}
	if !strings.Contains(first.label, "active") {
		t.Fatalf("active device label missing marker: %q", first.label)
	}
	if strings.Contains(second.label, "active") {
		t.Fatalf("inactive device label wrongly marked active: %q", second.label)
	}
	if first.trackURI != "" {
		t.Fatalf("device rows must have no trackURI (queue-add guard relies on it): %q", first.trackURI)
	}
}
