package lambda

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/lambda"
)

// Model is the Bubble Tea model for the Lambda browser.
type Model struct {
	client *lambda.Client

	// Function list state
	functions []lambda.Function
	nextToken *string // pagination for ListFunctions

	// Current function details
	details *lambda.FunctionDetails

	// Expanded function popup
	expandedFunc int            // index of expanded function, -1 if none
	expandedView viewport.Model // viewport for expanded details

	// Navigation
	cursor      int
	listOffset  int
	loadingMore bool

	// Cache
	cache    *cache.Cache
	cacheKey cache.Key

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string
}

// NewModel creates a new Lambda browser model.
func NewModel(client *lambda.Client, c *cache.Cache, cacheKey cache.Key) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	m := Model{
		client:       client,
		search:       ti,
		loading:      true,
		expandedFunc: -1,
		cache:        c,
		cacheKey:     cacheKey,
	}
	if c != nil {
		if items, ok := c.Get(cacheKey); ok {
			if functions, ok := items.([]lambda.Function); ok && len(functions) > 0 {
				m.functions = functions
				m.loading = false
				m.statusLine = fmt.Sprintf("%d functions (cached)", len(functions))
			}
		}
	}
	return m
}

// Init initializes the model by loading functions.
func (m Model) Init() tea.Cmd {
	if len(m.functions) > 0 {
		return nil
	}
	return m.loadFunctionsCmd()
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case functionsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.functions = msg.functions
		m.nextToken = msg.nextToken
		sortFunctions(m.functions)
		m.updateCache()
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d functions (loading more...)", len(msg.functions))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d functions", len(msg.functions))
		}
		return m, tea.Batch(m.fetchDetailsCmd(), m.loadMoreIfNeeded())

	case moreFunctionsLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.functions = append(m.functions, msg.functions...)
		m.nextToken = msg.nextToken
		sortFunctions(m.functions)
		m.updateCache()
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d functions (loading more...)", len(m.functions))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d functions", len(m.functions))
		}
		return m, m.loadMoreIfNeeded()

	case functionDetailsMsg:
		if msg.err != nil {
			return m, nil
		}
		if m.currentFunctionName() == msg.functionName {
			m.details = msg.details
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle expanded function popup
	if m.expandedFunc >= 0 {
		switch msg.String() {
		case "esc", "q", "enter":
			m.expandedFunc = -1
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
			return m, tea.Batch(m.onCursorMove(), m.loadMoreIfNeeded())
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
			return m, m.onCursorMove()
		}
	case "down", "j":
		maxIdx := len(m.filteredFunctions()) - 1
		if m.cursor < maxIdx {
			m.cursor++
			m.ensureCursorVisible()
			cmd := m.onCursorMove()
			// Lazy load more when near the end
			if m.nextToken != nil && !m.loadingMore {
				if m.cursor >= len(m.filteredFunctions())-5 {
					m.loadingMore = true
					return m, tea.Batch(cmd, m.loadMoreFunctionsCmd())
				}
			}
			return m, cmd
		}
	case "enter", " ":
		// Expand function details in popup
		functions := m.filteredFunctions()
		if len(functions) > 0 && m.cursor < len(functions) {
			m.expandedFunc = m.cursor
			m.expandedView = initExpandedFunctionView(m.details, functions[m.cursor].Name, m.width, m.height)
		}
		return m, nil
	case "/":
		m.searching = true
		m.search.SetValue("")
		return m, m.search.Focus()
	case "y":
		arn := m.getARN()
		if arn != "" {
			_ = clipboard.WriteAll(arn)
			m.statusLine = "Copied: " + arn
		}
	case "ctrl+r":
		if m.cache != nil {
			m.cache.Delete(m.cacheKey)
		}
		m.functions = nil
		m.nextToken = nil
		m.cursor = 0
		m.listOffset = 0
		m.details = nil
		m.loading = true
		m.statusLine = "Refreshing..."
		return m, m.loadFunctionsCmd()
	}

	return m, nil
}

func (m Model) getARN() string {
	if m.details != nil {
		return m.details.ARN
	}
	functions := m.filteredFunctions()
	if len(functions) == 0 || m.cursor >= len(functions) {
		return ""
	}
	return fmt.Sprintf("arn:aws:lambda:*:*:function:%s", functions[m.cursor].Name)
}

// View renders the model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	leftWidth := m.width * 2 / 5
	rightWidth := m.width - leftWidth
	bodyHeight := m.bodyHeight()

	left := panelStyle.Width(leftWidth - 2).Height(bodyHeight).Render(m.renderLeft())
	right := panelStyle.Width(rightWidth - 2).Height(bodyHeight).Render(m.renderRight())

	view := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	if m.expandedFunc >= 0 && m.expandedFunc < len(m.filteredFunctions()) {
		popup := m.renderExpandedFunction()
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
	// When scrolled past the top, the "↑ more above" indicator takes one
	// line from the visible area, so reduce the effective height.
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
	maxIdx := len(m.filteredFunctions()) - 1
	if m.cursor > maxIdx {
		m.cursor = maxIdx
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m Model) currentFunctionName() string {
	functions := m.filteredFunctions()
	if len(functions) == 0 || m.cursor >= len(functions) {
		return ""
	}
	return functions[m.cursor].Name
}

func (m Model) onCursorMove() tea.Cmd {
	m.details = nil
	return m.fetchDetailsCmd()
}

func (m Model) filteredFunctions() []lambda.Function {
	if m.search.Value() == "" {
		return m.functions
	}
	q := strings.ToLower(m.search.Value())
	out := make([]lambda.Function, 0, len(m.functions))
	for _, f := range m.functions {
		if strings.Contains(strings.ToLower(f.Name), q) ||
			strings.Contains(strings.ToLower(f.Runtime), q) {
			out = append(out, f)
		}
	}
	return out
}

func (m *Model) loadMoreIfNeeded() tea.Cmd {
	if m.nextToken == nil || m.loadingMore {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreFunctionsCmd()
}

// updateCache stores the current functions in the cache.
func (m *Model) updateCache() {
	if m.cache != nil {
		m.cache.Set(m.cacheKey, m.functions)
	}
}

// sortFunctions sorts functions by Name (case-insensitive).
func sortFunctions(functions []lambda.Function) {
	sort.Slice(functions, func(i, j int) bool {
		return strings.ToLower(functions[i].Name) < strings.ToLower(functions[j].Name)
	})
}

// Commands

func (m Model) loadFunctionsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		functions, nextToken, err := m.client.ListFunctions(ctx, nil)
		return functionsLoadedMsg{functions: functions, nextToken: nextToken, err: err}
	}
}

func (m Model) loadMoreFunctionsCmd() tea.Cmd {
	token := m.nextToken
	return func() tea.Msg {
		ctx := context.Background()
		functions, nextToken, err := m.client.ListFunctions(ctx, token)
		return moreFunctionsLoadedMsg{functions: functions, nextToken: nextToken, err: err}
	}
}

func (m Model) fetchDetailsCmd() tea.Cmd {
	name := m.currentFunctionName()
	if name == "" {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		details, err := m.client.GetFunction(ctx, name)
		return functionDetailsMsg{details: details, functionName: name, err: err}
	}
}

// Searching reports whether the model has an active text input.
func (m Model) Searching() bool {
	return m.searching
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.expandedFunc >= 0 {
		return "↑↓ scroll, pgup/pgdn page, esc close"
	}
	if m.searching {
		return "enter/esc close search"
	}
	return "↑↓ move, / search, enter expand, y copy ARN, ctrl+r refresh"
}
