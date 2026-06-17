package logs

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sachamama/sacha/internal/logs"
)

// newTailModel builds a Model in the tailing state with a sized viewport,
// suitable for exercising tail scroll handling without a live AWS client.
func newTailModel(t *testing.T) Model {
	t.Helper()
	m := Model{
		selected:      map[string]bool{"/test/group": true},
		expandedEvent: -1,
		pollInterval:  defaultPollInterval,
		width:         120,
		height:        30,
		tailing:       true,
		focus:         panelTail,
	}
	m.setViewportSize(m.bodyHeight())
	if m.view.Height <= 0 {
		t.Fatalf("expected positive viewport height, got %d", m.view.Height)
	}
	return m
}

// makeEvents returns n events with distinct, identifiable messages.
func makeEvents(start, n int) []logs.TailEvent {
	base := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	out := make([]logs.TailEvent, 0, n)
	for i := 0; i < n; i++ {
		idx := start + i
		out = append(out, logs.TailEvent{
			Timestamp: base.Add(time.Duration(idx) * time.Second),
			LogGroup:  "/test/group",
			LogStream: "stream",
			Message:   fmt.Sprintf("event-%d", idx),
		})
	}
	return out
}

func (m Model) update(t *testing.T, msg interface{}) Model {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected model type %T", next)
	}
	return mm
}

// TestTailFollowPinsToBottom verifies that while following, new events keep
// the cursor and viewport pinned to the latest event.
func TestTailFollowPinsToBottom(t *testing.T) {
	m := newTailModel(t)
	m.autoScroll = true

	m = m.update(t, tailUpdateMsg{events: makeEvents(0, 100), nextStart: time.Now()})
	if m.eventCursor != 99 {
		t.Fatalf("expected cursor at last event 99, got %d", m.eventCursor)
	}

	m = m.update(t, tailUpdateMsg{events: makeEvents(100, 10), nextStart: time.Now()})
	if got := len(m.events); got != 110 {
		t.Fatalf("expected 110 events, got %d", got)
	}
	if m.eventCursor != 109 {
		t.Fatalf("expected cursor to follow to 109, got %d", m.eventCursor)
	}
	// The latest event should sit on the last visible row of the viewport.
	if row := m.eventCursor - m.view.YOffset; row != m.view.Height-1 {
		t.Fatalf("expected latest event on bottom row %d, got row %d", m.view.Height-1, row)
	}
}

// TestTailEventPageAndJumpKeys verifies page-up/down and home/end navigation
// over tail events, including how following (auto-scroll) re-engages only at
// the latest event.
func TestTailEventPageAndJumpKeys(t *testing.T) {
	m := newTailModel(t)
	m.autoScroll = true
	m = m.update(t, tailUpdateMsg{events: makeEvents(0, 100), nextStart: time.Now()})
	if m.eventCursor != 99 {
		t.Fatalf("expected cursor pinned to last event 99, got %d", m.eventCursor)
	}

	// Home jumps to the first event and stops following.
	m = m.update(t, tea.KeyMsg{Type: tea.KeyHome})
	if m.eventCursor != 0 {
		t.Fatalf("home: expected cursor 0, got %d", m.eventCursor)
	}
	if m.autoScroll {
		t.Fatal("home: expected auto-scroll disabled")
	}

	// PageDown advances by one viewport page.
	page := m.eventPageSize()
	m = m.update(t, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.eventCursor != page {
		t.Fatalf("pgdown: expected cursor %d, got %d", page, m.eventCursor)
	}

	// PageUp returns to the top.
	m = m.update(t, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.eventCursor != 0 {
		t.Fatalf("pgup: expected cursor 0, got %d", m.eventCursor)
	}

	// End jumps to the latest event and re-engages following.
	m = m.update(t, tea.KeyMsg{Type: tea.KeyEnd})
	if m.eventCursor != 99 {
		t.Fatalf("end: expected cursor 99, got %d", m.eventCursor)
	}
	if !m.autoScroll {
		t.Fatal("end: expected auto-scroll re-enabled at latest event")
	}
}

// TestGroupListPageAndJumpKeys verifies page-up/down and home/end navigation
// over the log-group list when not tailing.
func TestGroupListPageAndJumpKeys(t *testing.T) {
	m := Model{
		selected:      map[string]bool{},
		expandedEvent: -1,
		pollInterval:  defaultPollInterval,
		width:         120,
		height:        40,
	}
	for i := 0; i < 100; i++ {
		m.logGroups = append(m.logGroups, logs.LogGroup{Name: fmt.Sprintf("/g/%03d", i)})
	}

	m = m.update(t, tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursor != 99 {
		t.Fatalf("end: expected cursor 99, got %d", m.cursor)
	}

	m = m.update(t, tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 {
		t.Fatalf("home: expected cursor 0, got %d", m.cursor)
	}

	page := m.listHeight()
	m = m.update(t, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.cursor != page {
		t.Fatalf("pgdown: expected cursor %d, got %d", page, m.cursor)
	}

	m = m.update(t, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.cursor != 0 {
		t.Fatalf("pgup: expected cursor 0, got %d", m.cursor)
	}
}

// TestTailPreservesScrollOnAppend verifies that when the user has scrolled up
// (not following), appending new events does not move their view.
func TestTailPreservesScrollOnAppend(t *testing.T) {
	m := newTailModel(t)
	m.autoScroll = true
	m = m.update(t, tailUpdateMsg{events: makeEvents(0, 100), nextStart: time.Now()})

	// Scroll up to event 50.
	m.autoScroll = false
	m.eventCursor = 50
	m.view.SetYOffset(40) // cursor sits on screen row 10
	anchorMsg := m.filteredEvents()[m.eventCursor].Message
	anchorOffset := m.view.YOffset

	m = m.update(t, tailUpdateMsg{events: makeEvents(100, 10), nextStart: time.Now()})

	if got := m.filteredEvents()[m.eventCursor].Message; got != anchorMsg {
		t.Fatalf("cursor moved off anchor event: want %q got %q", anchorMsg, got)
	}
	if m.view.YOffset != anchorOffset {
		t.Fatalf("scroll position changed on append: want offset %d got %d", anchorOffset, m.view.YOffset)
	}
}

// TestTailClearBuffer verifies that pressing "C" while tailing empties the
// event buffer, re-engages following, and keeps tailing active so new events
// continue to arrive.
func TestTailClearBuffer(t *testing.T) {
	m := newTailModel(t)
	m.autoScroll = true
	m = m.update(t, tailUpdateMsg{events: makeEvents(0, 100), nextStart: time.Now()})

	// Scroll up so following is disengaged before clearing.
	m.autoScroll = false
	m.eventCursor = 40

	m = m.update(t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	if len(m.events) != 0 {
		t.Fatalf("expected events cleared, got %d", len(m.events))
	}
	if m.eventCursor != 0 {
		t.Fatalf("expected cursor reset to 0, got %d", m.eventCursor)
	}
	if !m.autoScroll {
		t.Fatal("expected auto-scroll re-enabled after clear")
	}
	if !m.tailing {
		t.Fatal("expected tailing to remain active after clear")
	}

	// New events arriving after a clear are shown and followed.
	m = m.update(t, tailUpdateMsg{events: makeEvents(200, 5), nextStart: time.Now()})
	if len(m.events) != 5 {
		t.Fatalf("expected 5 events after clear, got %d", len(m.events))
	}
	if m.eventCursor != 4 {
		t.Fatalf("expected cursor following to 4, got %d", m.eventCursor)
	}
}

// TestTailPreservesScrollOnTrim verifies that when the buffer overflows and is
// trimmed from the front, the user's scroll position is preserved by anchoring
// on the event under the cursor rather than a raw index.
func TestTailPreservesScrollOnTrim(t *testing.T) {
	m := newTailModel(t)
	m.autoScroll = true
	m = m.update(t, tailUpdateMsg{events: makeEvents(0, 1000), nextStart: time.Now()})

	// Scroll up to a known event well away from the bottom.
	m.autoScroll = false
	m.eventCursor = 500
	m.view.SetYOffset(495) // cursor on screen row 5
	anchorMsg := m.filteredEvents()[m.eventCursor].Message
	anchorRow := m.eventCursor - m.view.YOffset

	// Push 50 more events, forcing a 50-event trim from the front.
	m = m.update(t, tailUpdateMsg{events: makeEvents(1000, 50), nextStart: time.Now()})

	if got := len(m.events); got != 1000 {
		t.Fatalf("expected buffer capped at 1000, got %d", got)
	}
	if got := m.filteredEvents()[m.eventCursor].Message; got != anchorMsg {
		t.Fatalf("cursor lost anchor after trim: want %q got %q", anchorMsg, got)
	}
	if m.eventCursor != 450 {
		t.Fatalf("expected cursor shifted to 450 after trim, got %d", m.eventCursor)
	}
	if gotRow := m.eventCursor - m.view.YOffset; gotRow != anchorRow {
		t.Fatalf("screen row changed after trim: want %d got %d", anchorRow, gotRow)
	}
}
