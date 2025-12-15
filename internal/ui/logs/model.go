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

type Model struct {
	client *logs.Client

	width  int
	height int

	logGroups []logs.LogGroup
	cursor    int
	selected  map[string]bool
	loading   bool

	searching  bool
	search     textinput.Model
	statusLine string

	tailing      bool
	tailStart    time.Time
	pollInterval time.Duration
	events       []logs.TailEvent
	view         viewport.Model
}

func NewModel(client *logs.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter log groups"
	ti.Prompt = "/ "
	return Model{
		client:       client,
		selected:     map[string]bool{},
		loading:      true,
		search:       ti,
		pollInterval: defaultPollInterval,
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
	case logGroupsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.logGroups = msg.groups
		m.statusLine = fmt.Sprintf("loaded %d log groups", len(msg.groups))
	case tea.KeyMsg:
		if m.searching {
			switch msg.Type {
			case tea.KeyEnter, tea.KeyEscape:
				m.searching = false
				return m, nil
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filteredGroups())-1 {
				m.cursor++
			}
		case "/":
			m.searching = true
			return m, m.search.Focus()
		case " ":
			m.toggleSelection()
		case "a":
			m.toggleAll()
		case "t":
			if len(m.selectedGroups()) > 0 {
				m.tailing = true
				m.events = nil
				m.tailStart = time.Now().Add(-defaultTailWindow)
				m.view = viewport.Model{}
				m.setViewportSize(m.bodyHeight())
				return m, m.pollTailCmd()
			}
		case "q", "esc":
			if m.tailing {
				m.tailing = false
			}
		case "pgup", "pgdn":
			if m.tailing {
				var cmd tea.Cmd
				m.view, cmd = m.view.Update(msg)
				return m, cmd
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
			m.view.SetContent(renderEvents(m.events))
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

	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth
	bodyHeight := m.bodyHeight()

	if m.tailing {
		m.setViewportSize(bodyHeight)
	}

	left := panelStyle.Width(leftWidth).Height(bodyHeight).Render(m.renderGroups())
	right := panelStyle.Width(rightWidth).Height(bodyHeight).Render(m.renderTail())

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
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
	if m.cursor >= len(out) {
		m.cursor = len(out) - 1
		if m.cursor < 0 {
			m.cursor = 0
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

func (m *Model) setViewportSize(bodyHeight int) {
	if !m.tailing {
		return
	}
	rightWidth := m.width - m.width/2
	innerWidth := rightWidth - 4 // account for border/padding
	if innerWidth < 20 {
		innerWidth = rightWidth
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
