package ec2

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/ec2"
)

// Model is the Bubble Tea model for the EC2 browser.
type Model struct {
	client *ec2.Client

	// Instance list state
	instances []ec2.Instance
	nextToken *string

	// Expanded instance popup
	expandedInst int            // index of expanded instance, -1 if none
	expandedView viewport.Model // viewport for expanded details

	// Navigation
	cursor      int
	listOffset  int
	loadingMore bool

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string
}

// NewModel creates a new EC2 browser model.
func NewModel(client *ec2.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	return Model{
		client:       client,
		search:       ti,
		loading:      true,
		expandedInst: -1,
	}
}

// Init initializes the model by loading instances.
func (m Model) Init() tea.Cmd {
	return m.loadInstancesCmd()
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case instancesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.instances = msg.instances
		m.nextToken = msg.nextToken
		m.statusLine = fmt.Sprintf("Loaded %d instances", len(msg.instances))
		return m, nil

	case moreInstancesLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.instances = append(m.instances, msg.instances...)
		m.nextToken = msg.nextToken
		m.statusLine = fmt.Sprintf("Loaded %d instances", len(m.instances))
		return m, m.loadMoreIfNeeded()

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle expanded instance popup
	if m.expandedInst >= 0 {
		switch msg.String() {
		case "esc", "q", "enter":
			m.expandedInst = -1
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

	// Handle search input
	if m.searching {
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEscape:
			m.searching = false
			m.clampCursor()
			return m, m.loadMoreIfNeeded()
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.clampCursor()
		return m, cmd
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
	case "down", "j":
		maxIdx := len(m.filteredInstances()) - 1
		if m.cursor < maxIdx {
			m.cursor++
			m.ensureCursorVisible()
			// Lazy load more when near the end
			if m.nextToken != nil && !m.loadingMore {
				if m.cursor >= len(m.filteredInstances())-5 {
					m.loadingMore = true
					return m, m.loadMoreInstancesCmd()
				}
			}
		}
	case "enter", " ":
		// Expand instance details in popup
		instances := m.filteredInstances()
		if len(instances) > 0 && m.cursor < len(instances) {
			m.expandedInst = m.cursor
			m.expandedView = initExpandedInstanceView(instances[m.cursor], m.width, m.height)
		}
		return m, nil
	case "/":
		m.searching = true
		m.search.SetValue("")
		return m, m.search.Focus()
	case "y":
		id := m.getInstanceID()
		if id != "" {
			_ = clipboard.WriteAll(id)
			m.statusLine = "Copied: " + id
		}
	}

	return m, nil
}

func (m Model) getInstanceID() string {
	instances := m.filteredInstances()
	if len(instances) == 0 || m.cursor >= len(instances) {
		return ""
	}
	return instances[m.cursor].InstanceID
}

// View renders the model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth
	bodyHeight := m.bodyHeight()

	left := panelStyle.Width(leftWidth - 2).Height(bodyHeight).Render(m.renderLeft())
	right := panelStyle.Width(rightWidth - 2).Height(bodyHeight).Render(m.renderRight())

	view := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	if m.expandedInst >= 0 && m.expandedInst < len(m.filteredInstances()) {
		popup := m.renderExpandedInstance()
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	return view
}

func (m Model) bodyHeight() int {
	h := m.height - 4
	if h < 4 {
		return m.height
	}
	return h
}

func (m Model) listHeight() int {
	h := m.bodyHeight() - 7
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
	effective := visibleHeight
	if m.listOffset > 0 {
		effective--
	}
	if effective < 1 {
		effective = 1
	}

	if m.cursor >= m.listOffset+effective {
		m.listOffset = m.cursor - effective + 1
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

func (m *Model) clampCursor() {
	maxIdx := len(m.filteredInstances()) - 1
	if m.cursor > maxIdx {
		m.cursor = maxIdx
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m Model) filteredInstances() []ec2.Instance {
	if m.search.Value() == "" {
		return m.instances
	}
	q := strings.ToLower(m.search.Value())
	out := make([]ec2.Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		if strings.Contains(strings.ToLower(inst.Name), q) ||
			strings.Contains(strings.ToLower(inst.InstanceID), q) ||
			strings.Contains(strings.ToLower(inst.InstanceType), q) ||
			strings.Contains(strings.ToLower(inst.State), q) ||
			strings.Contains(inst.PrivateIP, q) ||
			strings.Contains(inst.PublicIP, q) {
			out = append(out, inst)
		}
	}
	return out
}

// loadMoreIfNeeded loads the next page if a filter is active and the filtered
// results don't fill the visible list height.
func (m *Model) loadMoreIfNeeded() tea.Cmd {
	if m.search.Value() == "" {
		return nil
	}
	if m.nextToken == nil || m.loadingMore {
		return nil
	}
	if len(m.filteredInstances()) >= m.listHeight() {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreInstancesCmd()
}

// Commands

func (m Model) loadInstancesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		instances, nextToken, err := m.client.ListInstances(ctx, nil)
		return instancesLoadedMsg{instances: instances, nextToken: nextToken, err: err}
	}
}

func (m Model) loadMoreInstancesCmd() tea.Cmd {
	token := m.nextToken
	return func() tea.Msg {
		ctx := context.Background()
		instances, nextToken, err := m.client.ListInstances(ctx, token)
		return moreInstancesLoadedMsg{instances: instances, nextToken: nextToken, err: err}
	}
}

// Searching reports whether the model has an active text input.
func (m Model) Searching() bool {
	return m.searching
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.expandedInst >= 0 {
		return "↑↓ scroll, pgup/pgdn page, esc close"
	}
	if m.searching {
		return "enter/esc close search"
	}
	return "↑↓ move, / search, enter expand, y copy ID"
}
