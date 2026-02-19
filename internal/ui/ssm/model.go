package ssm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/ssm"
)

// scrollPosition stores cursor state for scroll memory.
type scrollPosition struct {
	cursor     int
	listOffset int
}

// Model is the Bubble Tea model for the SSM Parameter Store browser.
type Model struct {
	client *ssm.Client

	// Location
	path []string // breadcrumb segments (e.g. ["/app/", "/app/prod/"])

	// List state
	params     []ssm.Parameter
	cursor     int
	listOffset int

	// Pagination
	nextToken   *string
	loadingMore bool

	// Right pane - parameter details (fetched with decryption)
	details *ssm.Parameter

	// Cache
	cache    *cache.Cache
	cacheKey cache.Key

	// Expanded parameter popup
	expandedParam int            // index of expanded param, -1 if none
	expandedView  viewport.Model // viewport for scrolling expanded content

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string

	// Scroll memory: stack of saved positions for hierarchical navigation
	scrollStack    []scrollPosition
	pendingRestore *scrollPosition
}

// NewModel creates a new SSM browser model.
func NewModel(client *ssm.Client, c *cache.Cache, cacheKey cache.Key) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	m := Model{
		client:        client,
		search:        ti,
		loading:       true,
		expandedParam: -1,
		cache:         c,
		cacheKey:      cacheKey,
	}
	// Only populate from cache at root level (no path segments)
	if c != nil && len(m.path) == 0 {
		if items, ok := c.Get(cacheKey); ok {
			if params, ok := items.([]ssm.Parameter); ok && len(params) > 0 {
				m.params = params
				m.loading = false
				m.statusLine = fmt.Sprintf("%d items (cached)", len(params))
			}
		}
	}
	return m
}

// Init initializes the model by loading top-level paths.
func (m Model) Init() tea.Cmd {
	if len(m.params) > 0 {
		return nil
	}
	return m.loadParametersCmd()
}

// currentPath returns the current path prefix.
func (m Model) currentPath() string {
	if len(m.path) == 0 {
		return "/"
	}
	return m.path[len(m.path)-1]
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case parametersLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.params = msg.params
		m.nextToken = msg.nextToken
		sortParameters(m.params)
		m.updateCache()
		m.cursor = 0
		m.details = nil
		// Restore scroll position if navigating back
		if m.pendingRestore != nil {
			m.cursor = m.pendingRestore.cursor
			m.listOffset = m.pendingRestore.listOffset
			m.pendingRestore = nil
			m.clampCursor()
		}
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d items (more available)", len(msg.params))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d items", len(msg.params))
		}
		return m, m.fetchDetailsCmd()

	case moreParametersLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.params = append(m.params, msg.params...)
		m.nextToken = msg.nextToken
		sortParameters(m.params)
		m.updateCache()
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d items (more available)", len(m.params))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d items", len(m.params))
		}
		return m, m.loadMoreIfNeeded()

	case parameterDetailMsg:
		if msg.err != nil {
			return m, nil
		}
		if m.currentParamName() == msg.name {
			m.details = msg.param
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	// Handle expanded popup scrolling
	if m.expandedParam >= 0 {
		switch msg.String() {
		case "up", "k":
			m.expandedView.ScrollUp(1)
			return m, nil
		case "down", "j":
			m.expandedView.ScrollDown(1)
			return m, nil
		case "pgup":
			m.expandedView.PageUp()
			return m, nil
		case "pgdn":
			m.expandedView.PageDown()
			return m, nil
		case "esc", "q":
			m.expandedParam = -1
			return m, nil
		}
		return m, nil
	}

	// Normal navigation
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
			return m, m.onCursorMove()
		}
	case "down", "j":
		maxIdx := m.maxCursorIndex()
		if m.cursor < maxIdx {
			m.cursor++
			m.ensureCursorVisible()
			cmd := m.onCursorMove()
			// Lazy load more when near the end
			if m.nextToken != nil && !m.loadingMore {
				if m.cursor >= len(m.filteredParams())-5 {
					m.loadingMore = true
					return m, tea.Batch(cmd, m.loadMoreCmd())
				}
			}
			return m, cmd
		}
	case "enter", " ":
		return m.handleEnter()
	case "backspace", "h", "esc":
		return m.handleBack()
	case "/":
		m.searching = true
		m.search.SetValue("")
		return m, m.search.Focus()
	case "y":
		text := m.getCopyText()
		if text != "" {
			_ = clipboard.WriteAll(text)
			m.statusLine = "Copied: " + text
		}
	case "ctrl+r":
		if m.cache != nil {
			m.cache.Delete(m.cacheKey)
		}
		m.params = nil
		m.nextToken = nil
		m.cursor = 0
		m.listOffset = 0
		m.details = nil
		m.loading = true
		m.statusLine = "Refreshing..."
		return m, m.loadParametersCmd()
	}

	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	items := m.filteredParams()
	if len(items) == 0 || m.cursor >= len(items) {
		return m, nil
	}
	item := items[m.cursor]

	if item.IsPrefix {
		// Navigate into this path prefix
		m.scrollStack = append(m.scrollStack, scrollPosition{cursor: m.cursor, listOffset: m.listOffset})
		m.path = append(m.path, item.Name)
		m.cursor = 0
		m.listOffset = 0
		m.search.SetValue("")
		m.loading = true
		m.details = nil
		return m, m.loadParametersCmd()
	}

	// Expand parameter in a popup
	m.expandedParam = m.cursor
	content := m.renderExpandedContent(item)
	m.expandedView = viewport.New(m.expandedWidth(), m.expandedHeight())
	m.expandedView.SetContent(content)
	return m, nil
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	if len(m.path) == 0 {
		return m, nil // Already at root
	}

	// Restore saved scroll position
	var saved scrollPosition
	if len(m.scrollStack) > 0 {
		saved = m.scrollStack[len(m.scrollStack)-1]
		m.scrollStack = m.scrollStack[:len(m.scrollStack)-1]
	}

	m.path = m.path[:len(m.path)-1]
	m.pendingRestore = &saved
	m.cursor = 0
	m.listOffset = 0
	m.search.SetValue("")
	m.loading = true
	m.details = nil
	return m, m.loadParametersCmd()
}

// View renders the model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Split width for two panels (40/60)
	leftWidth := m.width * 2 / 5
	rightWidth := m.width - leftWidth
	bodyHeight := m.bodyHeight()

	left := panelStyle.Width(leftWidth - 2).Height(bodyHeight).Render(m.renderLeft())
	right := panelStyle.Width(rightWidth - 2).Height(bodyHeight).Render(m.renderRight())

	view := joinHorizontal(left, right)

	// Overlay expanded popup if active
	if m.expandedParam >= 0 {
		view = m.renderExpandedPopup(view)
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
	// bodyHeight minus: header(1) + search hint(1) + blank(1) + footer(1) + status(1) + borders(2)
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

func (m Model) maxCursorIndex() int {
	return len(m.filteredParams()) - 1
}

func (m *Model) clampCursor() {
	maxIdx := m.maxCursorIndex()
	if m.cursor > maxIdx {
		m.cursor = maxIdx
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m Model) currentParamName() string {
	items := m.filteredParams()
	if len(items) == 0 || m.cursor >= len(items) {
		return ""
	}
	return items[m.cursor].Name
}

func (m Model) onCursorMove() tea.Cmd {
	m.details = nil
	return m.fetchDetailsCmd()
}

func (m Model) getCopyText() string {
	if m.details != nil {
		return m.details.Value
	}
	items := m.filteredParams()
	if len(items) == 0 || m.cursor >= len(items) {
		return ""
	}
	item := items[m.cursor]
	if item.IsPrefix {
		return item.Name
	}
	return item.Name
}

func (m Model) filteredParams() []ssm.Parameter {
	if m.search.Value() == "" {
		return m.params
	}
	q := strings.ToLower(m.search.Value())
	out := make([]ssm.Parameter, 0, len(m.params))
	for _, p := range m.params {
		displayName := p.DisplayName()
		if p.IsPrefix {
			displayName = p.Value // prefix display name stored in Value
		}
		if strings.Contains(strings.ToLower(displayName), q) {
			out = append(out, p)
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
	if len(m.filteredParams()) >= m.listHeight() {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreCmd()
}

// updateCache stores the current params in the cache (only at root level).
func (m *Model) updateCache() {
	if m.cache != nil && len(m.path) == 0 {
		m.cache.Set(m.cacheKey, m.params)
	}
}

// sortParameters sorts parameters with prefixes (IsPrefix=true) first, then by Name case-insensitive.
func sortParameters(params []ssm.Parameter) {
	sort.Slice(params, func(i, j int) bool {
		// Prefixes (folders) come first
		if params[i].IsPrefix != params[j].IsPrefix {
			return params[i].IsPrefix
		}
		return strings.ToLower(params[i].Name) < strings.ToLower(params[j].Name)
	})
}

// Commands

func (m Model) loadParametersCmd() tea.Cmd {
	path := m.currentPath()
	return func() tea.Msg {
		ctx := context.Background()
		if path == "/" {
			params, nextToken, err := m.client.ListTopLevelPaths(ctx, nil)
			return parametersLoadedMsg{params: params, nextToken: nextToken, err: err}
		}
		params, nextToken, err := m.client.GetParametersByPath(ctx, path, nil)
		return parametersLoadedMsg{params: params, nextToken: nextToken, err: err}
	}
}

func (m Model) loadMoreCmd() tea.Cmd {
	path := m.currentPath()
	token := m.nextToken
	return func() tea.Msg {
		ctx := context.Background()
		if path == "/" {
			params, nextToken, err := m.client.ListTopLevelPaths(ctx, token)
			return moreParametersLoadedMsg{params: params, nextToken: nextToken, err: err}
		}
		params, nextToken, err := m.client.GetParametersByPath(ctx, path, token)
		return moreParametersLoadedMsg{params: params, nextToken: nextToken, err: err}
	}
}

func (m Model) fetchDetailsCmd() tea.Cmd {
	items := m.filteredParams()
	if len(items) == 0 || m.cursor >= len(items) {
		return nil
	}
	item := items[m.cursor]
	if item.IsPrefix {
		return nil
	}
	name := item.Name
	return func() tea.Msg {
		ctx := context.Background()
		param, err := m.client.GetParameter(ctx, name)
		return parameterDetailMsg{param: param, name: name, err: err}
	}
}

func (m Model) expandedWidth() int {
	w := m.width - 8
	if w < 40 {
		return 40
	}
	return w
}

func (m Model) expandedHeight() int {
	h := m.height - 10
	if h < 10 {
		return 10
	}
	return h
}

// Searching reports whether the model has an active text input.
func (m Model) Searching() bool {
	return m.searching
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.searching {
		return "enter/esc close search"
	}
	if m.expandedParam >= 0 {
		return "↑↓ scroll, esc close"
	}
	if len(m.path) == 0 {
		return "↑↓ move, / search, enter open, y copy, ctrl+r refresh"
	}
	return "↑↓ move, / search, enter open/expand, y copy, ctrl+r refresh, esc back"
}
