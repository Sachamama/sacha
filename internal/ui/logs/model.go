package logs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/sachamama/sacha/internal/logs"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultTailWindow   = 15 * time.Minute
	defaultPollInterval = 5 * time.Second
)

type logGroupsLoadedMsg struct {
	groups []logs.LogGroup
	err    error
}

type tailUpdateMsg struct {
	events    []logs.TailEvent
	nextStart time.Time
	err       error
}

type pollTailMsg struct{}

type logGroupCreatedMsg struct {
	name string
	err  error
}

type panel int

const (
	panelGroups panel = iota
	panelTail
)

type Model struct {
	client *logs.Client

	width  int
	height int

	logGroups  []logs.LogGroup
	cursor     int
	listOffset int
	selected   map[string]bool
	loading    bool

	searching  bool
	search     textinput.Model
	statusLine string

	creating    bool
	createInput textinput.Model

	tailing      bool
	tailStart    time.Time
	pollInterval time.Duration
	events       []logs.TailEvent
	view         viewport.Model
	jsonView     bool // toggle between JSON table view and plain view

	fullscreen    bool           // fullscreen tail mode
	focus         panel          // which panel has focus
	eventCursor   int            // cursor position in tail events
	expandedEvent int            // index of expanded event, -1 if none
	scrollX       int            // horizontal scroll offset for fullscreen
	expandedView  viewport.Model // viewport for expanded event
}

func NewModel(client *logs.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter log groups"
	ti.Prompt = "/ "

	ci := textinput.New()
	ci.Placeholder = "log group name"
	ci.Prompt = "Name: "

	return Model{
		client:        client,
		selected:      map[string]bool{},
		loading:       true,
		search:        ti,
		createInput:   ci,
		pollInterval:  defaultPollInterval,
		jsonView:      true, // default to table view for JSON logs
		expandedEvent: -1,   // no event expanded
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadLogGroupsCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.setViewportSize(m.bodyHeight())
		m.ensureCursorVisible()
	case logGroupsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.logGroups = msg.groups
		m.statusLine = fmt.Sprintf("loaded %d log groups", len(msg.groups))
	case logGroupCreatedMsg:
		m.creating = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.statusLine = fmt.Sprintf("Created log group: %s", msg.name)
		m.loading = true
		return m, m.loadLogGroupsCmd()

	case tea.KeyMsg:
		if m.creating {
			switch msg.Type {
			case tea.KeyEnter:
				name := m.createInput.Value()
				if name != "" {
					m.statusLine = "Creating log group..."
					return m, m.createLogGroupCmd(name)
				}
				m.creating = false
				return m, nil
			case tea.KeyEscape:
				m.creating = false
				return m, nil
			}
			var cmd tea.Cmd
			m.createInput, cmd = m.createInput.Update(msg)
			return m, cmd
		}

		if m.searching {
			switch msg.Type {
			case tea.KeyEnter, tea.KeyEscape:
				m.searching = false
				return m, nil
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			// Reset cursor and offset when filter changes
			m.cursor = 0
			m.listOffset = 0
			return m, cmd
		}

		// Handle expanded event popup
		if m.expandedEvent >= 0 {
			switch msg.String() {
			case "esc", "q", "enter":
				m.expandedEvent = -1
			case "up", "k":
				m.expandedView.ScrollUp(1)
			case "down", "j", " ":
				m.expandedView.ScrollDown(1)
			case "pgup":
				m.expandedView.HalfPageUp()
			case "pgdn":
				m.expandedView.HalfPageDown()
			}
			return m, nil
		}

		switch msg.String() {
		case "tab":
			if m.tailing && !m.fullscreen {
				if m.focus == panelTail {
					m.focus = panelGroups
				} else {
					m.focus = panelTail
				}
				m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
			}
		case "left", "h":
			if m.tailing && m.fullscreen {
				// Horizontal scroll left in fullscreen
				if m.scrollX > 0 {
					m.scrollX -= 10
					if m.scrollX < 0 {
						m.scrollX = 0
					}
					m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, true, m.scrollX))
				}
			} else if m.tailing && !m.fullscreen {
				if m.focus == panelTail {
					m.focus = panelGroups
				} else {
					m.focus = panelTail
				}
				m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
			}
		case "right", "l":
			if m.tailing && m.fullscreen {
				// Horizontal scroll right in fullscreen
				m.scrollX += 10
				m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, true, m.scrollX))
			} else if m.tailing && !m.fullscreen {
				if m.focus == panelGroups {
					m.focus = panelTail
				} else {
					m.focus = panelGroups
				}
				m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
			}
		case "up", "k":
			if m.tailing && m.focus == panelTail {
				if m.eventCursor > 0 {
					m.eventCursor--
					m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
				}
			} else {
				if m.cursor > 0 {
					m.cursor--
					m.ensureCursorVisible()
				}
			}
		case "down", "j":
			if m.tailing && m.focus == panelTail {
				if m.eventCursor < len(m.events)-1 {
					m.eventCursor++
					m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
				}
			} else {
				if m.cursor < len(m.filteredGroups())-1 {
					m.cursor++
					m.ensureCursorVisible()
				}
			}
		case "/":
			if !m.tailing || m.focus == panelGroups {
				m.searching = true
				return m, m.search.Focus()
			}
		case " ":
			if m.tailing && m.focus == panelTail && len(m.events) > 0 {
				m.expandedEvent = m.eventCursor
				m.expandedView = initExpandedView(m.events[m.eventCursor], m.width, m.height)
			} else {
				m.toggleSelection()
				// Refresh logs when selection changes while tailing
				if m.tailing && len(m.selectedGroups()) > 0 {
					m.events = nil
					m.eventCursor = 0
					m.tailStart = time.Now().Add(-defaultTailWindow)
					m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
					return m, m.pollTailCmd()
				}
			}
		case "enter":
			if m.tailing && m.focus == panelTail && len(m.events) > 0 {
				m.expandedEvent = m.eventCursor
				m.expandedView = initExpandedView(m.events[m.eventCursor], m.width, m.height)
			}
		case "a":
			if !m.tailing || m.focus == panelGroups {
				m.toggleAll()
				// Refresh logs when selection changes while tailing
				if m.tailing && len(m.selectedGroups()) > 0 {
					m.events = nil
					m.eventCursor = 0
					m.tailStart = time.Now().Add(-defaultTailWindow)
					m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
					return m, m.pollTailCmd()
				}
			}
		case "c":
			if !m.tailing || m.focus == panelGroups {
				m.creating = true
				m.createInput.SetValue("")
				return m, m.createInput.Focus()
			}
		case "t":
			if !m.tailing && len(m.selectedGroups()) > 0 {
				m.tailing = true
				m.fullscreen = false
				m.focus = panelTail
				m.events = nil
				m.eventCursor = 0
				m.tailStart = time.Now().Add(-defaultTailWindow)
				m.view = viewport.Model{}
				m.setViewportSize(m.bodyHeight())
				return m, m.pollTailCmd()
			}
		case "f":
			if m.tailing {
				m.fullscreen = !m.fullscreen
				if m.fullscreen {
					m.focus = panelTail
				} else {
					m.scrollX = 0 // Reset horizontal scroll when exiting fullscreen
				}
				m.setViewportSize(m.bodyHeight())
				m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
			}
		case "q", "esc":
			if m.tailing {
				m.tailing = false
				m.fullscreen = false
				m.focus = panelGroups
				m.scrollX = 0
			}
		case "v":
			if m.tailing {
				m.jsonView = !m.jsonView
				m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
				return m, nil
			}
		case "x":
			// Stop watching and reset Log panel
			if m.tailing {
				m.tailing = false
				m.fullscreen = false
				m.focus = panelGroups
				m.events = nil
				m.eventCursor = 0
				m.scrollX = 0
			}
		}
	case pollTailMsg:
		if !m.tailing {
			return m, nil
		}
		return m, m.pollTailCmd()
	case tailUpdateMsg:
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		if len(msg.events) > 0 {
			m.tailStart = msg.nextStart
			m.events = append(m.events, msg.events...)
			if len(m.events) > 1000 {
				m.events = m.events[len(m.events)-1000:]
			}
			// Keep cursor in bounds
			if m.eventCursor >= len(m.events) {
				m.eventCursor = len(m.events) - 1
			}
			m.view.SetContent(renderEvents(m.events, m.jsonView, m.eventCursor, m.view.Width, m.focus == panelTail, m.scrollX))
		}
		if m.tailing {
			return m, tea.Tick(m.pollInterval, func(time.Time) tea.Msg { return pollTailMsg{} })
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	bodyHeight := m.bodyHeight()

	if m.tailing {
		m.setViewportSize(bodyHeight)
	}

	var view string

	if m.tailing && m.fullscreen {
		// Fullscreen tail mode
		view = panelStyle.Width(m.width - 2).Height(bodyHeight).Render(m.renderTail())
	} else {
		// Split width for two panels
		leftWidth := m.width / 2
		rightWidth := m.width - leftWidth

		// Highlight focused panel
		leftStyle := panelStyle
		rightStyle := panelStyle
		if m.tailing {
			if m.focus == panelGroups {
				leftStyle = leftStyle.BorderForeground(lipgloss.Color("212"))
			} else {
				rightStyle = rightStyle.BorderForeground(lipgloss.Color("212"))
			}
		}

		left := leftStyle.Width(leftWidth - 2).Height(bodyHeight).Render(m.renderGroups())
		right := rightStyle.Width(rightWidth - 2).Height(bodyHeight).Render(m.renderTail())

		view = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	if m.creating {
		popup := m.renderCreatePopup()
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	if m.expandedEvent >= 0 && m.expandedEvent < len(m.events) {
		popup := m.renderExpandedEvent(m.events[m.expandedEvent])
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	return view
}

func (m Model) loadLogGroupsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var (
			all   []logs.LogGroup
			token *string
		)
		for {
			groups, next, err := m.client.ListLogGroups(ctx, token)
			if err != nil {
				return logGroupsLoadedMsg{err: err}
			}
			all = append(all, groups...)
			if next == nil || aws.ToString(next) == "" {
				break
			}
			token = next
		}
		return logGroupsLoadedMsg{groups: all}
	}
}

func (m Model) pollTailCmd() tea.Cmd {
	groups := m.selectedGroups()
	start := m.tailStart
	return func() tea.Msg {
		ctx := context.Background()
		events, next, err := m.client.FetchEvents(ctx, groups, start)
		return tailUpdateMsg{events: events, nextStart: next, err: err}
	}
}

func (m Model) createLogGroupCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.client.CreateLogGroup(ctx, name)
		return logGroupCreatedMsg{name: name, err: err}
	}
}

func (m Model) filteredGroups() []logs.LogGroup {
	if !m.searching && m.search.Value() == "" {
		return m.logGroups
	}
	q := strings.ToLower(m.search.Value())
	out := make([]logs.LogGroup, 0, len(m.logGroups))
	for _, g := range m.logGroups {
		if strings.Contains(strings.ToLower(g.Name), q) {
			out = append(out, g)
		}
	}
	return out
}

func (m *Model) toggleSelection() {
	groups := m.filteredGroups()
	if len(groups) == 0 || m.cursor >= len(groups) {
		return
	}
	name := groups[m.cursor].Name
	if m.selected[name] {
		delete(m.selected, name)
	} else {
		m.selected[name] = true
	}
}

func (m *Model) toggleAll() {
	if len(m.selected) == len(m.logGroups) {
		m.selected = map[string]bool{}
		return
	}
	for _, g := range m.logGroups {
		m.selected[g.Name] = true
	}
}

func (m Model) selectedGroups() []string {
	out := make([]string, 0, len(m.selected))
	for name, ok := range m.selected {
		if ok {
			out = append(out, name)
		}
	}
	return out
}

func (m Model) selectedCount() int {
	count := 0
	for _, ok := range m.selected {
		if ok {
			count++
		}
	}
	return count
}

// Tailing reports whether the model is actively tailing logs.
func (m Model) Tailing() bool {
	return m.tailing
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.expandedEvent >= 0 {
		return "↑↓ scroll, pgup/pgdn page, esc close"
	}
	if m.creating {
		return "enter create, esc cancel"
	}
	if m.searching {
		return "enter/esc close search"
	}
	if m.tailing {
		if m.fullscreen {
			viewMode := "table"
			if !m.jsonView {
				viewMode = "plain"
			}
			return fmt.Sprintf("↑↓ move, ←→ scroll, enter expand, v [%s], f exit fullscreen, x/q stop", viewMode)
		}
		if m.focus == panelGroups {
			return "↑↓ move, / search, space select, a all, tab/→ switch, f fullscreen, x/q stop"
		}
		viewMode := "table"
		if !m.jsonView {
			viewMode = "plain"
		}
		return fmt.Sprintf("↑↓ move, enter expand, v [%s], tab/← switch, f fullscreen, x/q stop", viewMode)
	}
	return "↑↓ move, / search, space select, a all, c create, t tail"
}

func (m *Model) setViewportSize(bodyHeight int) {
	if !m.tailing {
		return
	}
	var innerWidth int
	if m.fullscreen {
		innerWidth = m.width - 4 // account for border/padding
	} else {
		rightWidth := m.width - m.width/2
		innerWidth = rightWidth - 4 // account for border/padding
	}
	if innerWidth < 20 {
		innerWidth = m.width - 4
	}
	contentHeight := bodyHeight - 2  // panel borders
	innerHeight := contentHeight - 1 // header inside panel
	if innerHeight < 1 {
		innerHeight = 1
	}
	m.view.Width = innerWidth
	m.view.Height = innerHeight
}

func (m Model) bodyHeight() int {
	h := m.height - 4 // account for header/footer lines in app view
	if h < 4 {
		return m.height
	}
	return h
}

func (m Model) listHeight() int {
	// bodyHeight minus: header(1) + search hint(1) + blank(1) + footer(1) + borders(2)
	h := m.bodyHeight() - 6
	if h < 3 {
		return 3
	}
	return h
}

func (m *Model) ensureCursorVisible() {
	visibleHeight := m.listHeight()
	if visibleHeight <= 0 {
		return
	}

	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}

	if m.cursor >= m.listOffset+visibleHeight {
		m.listOffset = m.cursor - visibleHeight + 1
	}

	if m.listOffset < 0 {
		m.listOffset = 0
	}
}
