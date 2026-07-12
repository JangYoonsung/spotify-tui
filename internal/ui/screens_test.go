package ui

import "testing"

func TestListStateMoveCursor(t *testing.T) {
	mk := func(n int) listState {
		items := make([]listItem, n)
		return listState{items: items}
	}

	t.Run("empty list is a no-op", func(t *testing.T) {
		l := mk(0)
		l.moveCursor(1)
		if l.cursor != 0 {
			t.Fatalf("cursor = %d, want 0", l.cursor)
		}
	})

	t.Run("clamped at zero", func(t *testing.T) {
		l := mk(5)
		l.moveCursor(-3)
		if l.cursor != 0 {
			t.Fatalf("cursor = %d, want 0", l.cursor)
		}
	})

	t.Run("clamped at len-1", func(t *testing.T) {
		l := mk(3)
		l.moveCursor(10)
		if l.cursor != 2 {
			t.Fatalf("cursor = %d, want 2", l.cursor)
		}
	})

	t.Run("scrollTop follows cursor past the visible window", func(t *testing.T) {
		l := mk(20)
		for i := 0; i < 12; i++ {
			l.moveCursor(1)
		}
		if l.cursor != 12 {
			t.Fatalf("cursor = %d, want 12", l.cursor)
		}
		if l.scrollTop != l.cursor-listVisibleRows+1 {
			t.Fatalf("scrollTop = %d, want %d", l.scrollTop, l.cursor-listVisibleRows+1)
		}
	})

	t.Run("scrollTop follows cursor moving back up", func(t *testing.T) {
		l := mk(20)
		l.cursor, l.scrollTop = 15, 8
		l.moveCursor(-10)
		if l.cursor != 5 {
			t.Fatalf("cursor = %d, want 5", l.cursor)
		}
		if l.scrollTop != 5 {
			t.Fatalf("scrollTop = %d, want 5 (cursor moved above old scrollTop)", l.scrollTop)
		}
	})
}

func TestListStateSelected(t *testing.T) {
	l := listState{items: []listItem{{label: "a"}, {label: "b"}}, cursor: 1}
	item, ok := l.selected()
	if !ok || item.label != "b" {
		t.Fatalf("selected() = (%+v, %v), want (b, true)", item, ok)
	}

	empty := listState{}
	if _, ok := empty.selected(); ok {
		t.Fatalf("selected() on empty list should return ok=false")
	}
}
