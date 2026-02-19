package dynamodb

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
	"github.com/sachamama/sacha/internal/dynamodb"
)

const scanPageSize = 25

// Model is the Bubble Tea model for the DynamoDB browser.
type Model struct {
	client *dynamodb.Client

	// Table list state
	tables     []dynamodb.Table
	tableToken *string // pagination for ListTables

	// Current table
	table       string                     // empty = table list view
	description *dynamodb.TableDescription // details for hovered/open table

	// Items state
	items            []dynamodb.Item
	lastEvaluatedKey map[string]interface{} // for scan pagination
	itemColumns      []string               // ordered column names
	loadingMore      bool

	// Navigation
	cursor     int
	listOffset int

	// Cache
	cache    *cache.Cache
	cacheKey cache.Key

	// Scroll memory: saved position when entering a table
	savedCursor     int
	savedListOffset int

	// Expanded item popup
	expandedItem int            // index of expanded item, -1 if none
	expandedView viewport.Model // viewport for expanded item

	// Detail panel (right side) viewport for scrollable details
	detailViewport viewport.Model

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string
}

// NewModel creates a new DynamoDB browser model.
func NewModel(client *dynamodb.Client, c *cache.Cache, cacheKey cache.Key) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	m := Model{
		client:       client,
		search:       ti,
		loading:      true,
		expandedItem: -1,
		cache:        c,
		cacheKey:     cacheKey,
	}
	if c != nil {
		if items, ok := c.Get(cacheKey); ok {
			if tables, ok := items.([]dynamodb.Table); ok && len(tables) > 0 {
				m.tables = tables
				m.loading = false
				m.statusLine = fmt.Sprintf("%d tables (cached)", len(tables))
			}
		}
	}
	return m
}

// Init initializes the model by loading tables.
func (m Model) Init() tea.Cmd {
	if len(m.tables) > 0 {
		return nil
	}
	return m.loadTablesCmd()
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateDetailViewport()

	case tablesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.tables = msg.tables
		m.tableToken = msg.nextToken
		sortTables(m.tables)
		m.updateCache()
		m.statusLine = fmt.Sprintf("Loaded %d tables", len(msg.tables))
		return m, m.fetchDescriptionCmd()

	case moreTablesLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.tables = append(m.tables, msg.tables...)
		m.tableToken = msg.nextToken
		sortTables(m.tables)
		m.updateCache()
		m.statusLine = fmt.Sprintf("Loaded %d tables", len(m.tables))
		return m, m.loadMoreIfNeeded()

	case tableDescriptionMsg:
		if msg.err != nil {
			return m, nil
		}
		if m.currentTableName() == msg.tableName || m.table == msg.tableName {
			m.description = msg.desc
			m.updateDetailViewport()
		}
		return m, nil

	case itemsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		if m.table != msg.tableName {
			return m, nil
		}
		m.items = msg.result.Items
		m.lastEvaluatedKey = msg.result.LastEvaluatedKey
		m.cursor = 0
		m.listOffset = 0
		m.itemColumns = m.buildColumns()
		if m.lastEvaluatedKey != nil {
			m.statusLine = fmt.Sprintf("Loaded %d items (more available)", len(m.items))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d items", len(m.items))
		}
		m.updateDetailViewport()
		return m, nil

	case moreItemsLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.items = append(m.items, msg.result.Items...)
		m.lastEvaluatedKey = msg.result.LastEvaluatedKey
		m.itemColumns = m.buildColumns()
		if m.lastEvaluatedKey != nil {
			m.statusLine = fmt.Sprintf("Loaded %d items (more available)", len(m.items))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d items", len(m.items))
		}
		m.updateDetailViewport()
		return m, m.loadMoreIfNeeded()

	case allTablesLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.tables = append(m.tables, msg.tables...)
		m.tableToken = nil
		sortTables(m.tables)
		m.updateCache()
		m.statusLine = fmt.Sprintf("Loaded all %d tables", len(m.tables))
		return m, nil

	case allItemsLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.items = append(m.items, msg.result.Items...)
		m.lastEvaluatedKey = nil
		m.itemColumns = m.buildColumns()
		m.statusLine = fmt.Sprintf("Loaded all %d items", len(m.items))
		m.updateDetailViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle expanded item popup
	if m.expandedItem >= 0 {
		switch msg.String() {
		case "esc", "q", "enter":
			m.expandedItem = -1
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
			m.updateDetailViewport()
			return m, tea.Batch(m.onCursorMove(), m.loadMoreIfNeeded())
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.clampCursor()
		m.updateDetailViewport()
		return m, cmd
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
			if m.table == "" {
				m.description = nil
			}
			m.updateDetailViewport()
			return m, m.onCursorMove()
		}
	case "down", "j":
		maxIdx := m.maxCursorIndex()
		if m.cursor < maxIdx {
			m.cursor++
			m.ensureCursorVisible()
			if m.table == "" {
				m.description = nil
			}
			m.updateDetailViewport()
			cmd := m.onCursorMove()
			// Lazy load more when near the end
			if m.table == "" && m.tableToken != nil && !m.loadingMore {
				if m.cursor >= len(m.filteredTables())-5 {
					m.loadingMore = true
					return m, tea.Batch(cmd, m.loadMoreTablesCmd())
				}
			}
			if m.table != "" && m.lastEvaluatedKey != nil && !m.loadingMore {
				if m.cursor >= len(m.filteredItems())-5 {
					m.loadingMore = true
					return m, tea.Batch(cmd, m.loadMoreItemsCmd())
				}
			}
			return m, cmd
		}
	case "enter", " ":
		if m.table != "" {
			// In items view - expand the selected item
			items := m.filteredItems()
			if len(items) > 0 && m.cursor < len(items) {
				m.expandedItem = m.cursor
				m.expandedView = initExpandedItemView(items[m.cursor], m.itemColumns, m.width, m.height)
			}
			return m, nil
		}
		if msg.String() == "enter" {
			return m.handleEnter()
		}
	case "backspace", "esc", "h":
		return m.handleBack()
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
	case "A":
		if !m.loadingMore {
			if m.table == "" && m.tableToken != nil {
				m.loadingMore = true
				m.statusLine = "Loading all tables..."
				return m, m.loadAllTablesCmd()
			}
			if m.table != "" && m.lastEvaluatedKey != nil {
				m.loadingMore = true
				m.statusLine = "Loading all items..."
				return m, m.loadAllItemsCmd()
			}
		}
	case "pgup":
		m.detailViewport.HalfPageUp()
	case "pgdn":
		m.detailViewport.HalfPageDown()
	case "ctrl+r":
		if m.table == "" {
			if m.cache != nil {
				m.cache.Delete(m.cacheKey)
			}
			m.tables = nil
			m.tableToken = nil
			m.cursor = 0
			m.listOffset = 0
			m.description = nil
			m.loading = true
			m.statusLine = "Refreshing..."
			return m, m.loadTablesCmd()
		}
		m.items = nil
		m.lastEvaluatedKey = nil
		m.itemColumns = nil
		m.cursor = 0
		m.listOffset = 0
		m.loading = true
		m.statusLine = "Refreshing..."
		m.updateDetailViewport()
		return m, m.scanItemsCmd()
	}

	return m, nil
}

func (m Model) getARN() string {
	if m.table == "" {
		// In table list - copy table ARN
		tables := m.filteredTables()
		if len(tables) == 0 || m.cursor >= len(tables) {
			return ""
		}
		return fmt.Sprintf("arn:aws:dynamodb:*:*:table/%s", tables[m.cursor].Name)
	}
	// In items view - copy table ARN
	return fmt.Sprintf("arn:aws:dynamodb:*:*:table/%s", m.table)
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

	view := lipglossJoinHorizontal(left, right)

	if m.expandedItem >= 0 && m.expandedItem < len(m.filteredItems()) {
		popup := m.renderExpandedItem()
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

func (m Model) maxCursorIndex() int {
	if m.table == "" {
		return len(m.filteredTables()) - 1
	}
	return len(m.filteredItems()) - 1
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

func (m Model) currentTableName() string {
	if m.table != "" {
		return ""
	}
	tables := m.filteredTables()
	if len(tables) == 0 || m.cursor >= len(tables) {
		return ""
	}
	return tables[m.cursor].Name
}

func (m Model) onCursorMove() tea.Cmd {
	if m.table == "" {
		m.description = nil
		return m.fetchDescriptionCmd()
	}
	return nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.table == "" {
		tables := m.filteredTables()
		if len(tables) == 0 || m.cursor >= len(tables) {
			return m, nil
		}
		// Save scroll position before entering table
		m.savedCursor = m.cursor
		m.savedListOffset = m.listOffset
		m.table = tables[m.cursor].Name
		m.cursor = 0
		m.listOffset = 0
		m.search.SetValue("")
		m.loading = true
		m.items = nil
		m.lastEvaluatedKey = nil
		m.itemColumns = nil
		m.updateDetailViewport()
		return m, tea.Batch(m.scanItemsCmd(), m.fetchDescriptionForTableCmd(m.table))
	}
	return m, nil
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	if m.table == "" {
		return m, nil
	}
	m.table = ""
	// Restore saved scroll position
	m.cursor = m.savedCursor
	m.listOffset = m.savedListOffset
	m.items = nil
	m.lastEvaluatedKey = nil
	m.itemColumns = nil
	m.description = nil
	m.search.SetValue("")
	m.clampCursor()
	m.updateDetailViewport()
	return m, m.fetchDescriptionCmd()
}

func (m Model) filteredTables() []dynamodb.Table {
	if m.search.Value() == "" {
		return m.tables
	}
	q := strings.ToLower(m.search.Value())
	out := make([]dynamodb.Table, 0, len(m.tables))
	for _, t := range m.tables {
		if strings.Contains(strings.ToLower(t.Name), q) {
			out = append(out, t)
		}
	}
	return out
}

func (m Model) filteredItems() []dynamodb.Item {
	if m.search.Value() == "" {
		return m.items
	}
	q := strings.ToLower(m.search.Value())
	out := make([]dynamodb.Item, 0, len(m.items))
	for _, item := range m.items {
		for _, v := range item {
			if strings.Contains(strings.ToLower(v), q) {
				out = append(out, item)
				break
			}
		}
	}
	return out
}

func (m Model) buildColumns() []string {
	// Collect all keys across items, keeping key schema attributes first
	keyAttrs := make(map[string]bool)
	if m.description != nil {
		for _, ks := range m.description.KeySchema {
			keyAttrs[ks.AttributeName] = true
		}
	}

	seen := make(map[string]bool)
	var keyColumns []string
	var otherColumns []string

	for _, item := range m.items {
		for k := range item {
			if seen[k] {
				continue
			}
			seen[k] = true
			if keyAttrs[k] {
				keyColumns = append(keyColumns, k)
			} else {
				otherColumns = append(otherColumns, k)
			}
		}
	}

	return append(keyColumns, otherColumns...)
}

// loadMoreIfNeeded loads the next page if a filter is active and the filtered
// results don't fill the visible list height.
func (m *Model) loadMoreIfNeeded() tea.Cmd {
	if m.search.Value() == "" {
		return nil
	}
	if m.loadingMore {
		return nil
	}
	if m.table == "" {
		// Table list pagination
		if m.tableToken == nil {
			return nil
		}
		if len(m.filteredTables()) >= m.listHeight() {
			return nil
		}
		m.loadingMore = true
		return m.loadMoreTablesCmd()
	}
	// Items pagination
	if m.lastEvaluatedKey == nil {
		return nil
	}
	if len(m.filteredItems()) >= m.listHeight() {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreItemsCmd()
}

// updateCache stores the current tables in the cache.
func (m *Model) updateCache() {
	if m.cache != nil {
		m.cache.Set(m.cacheKey, m.tables)
	}
}

// sortTables sorts tables by Name (case-insensitive).
func sortTables(tables []dynamodb.Table) {
	sort.Slice(tables, func(i, j int) bool {
		return strings.ToLower(tables[i].Name) < strings.ToLower(tables[j].Name)
	})
}

// Commands

func (m Model) loadTablesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		tables, nextToken, err := m.client.ListTables(ctx, nil)
		return tablesLoadedMsg{tables: tables, nextToken: nextToken, err: err}
	}
}

func (m Model) loadMoreTablesCmd() tea.Cmd {
	token := m.tableToken
	return func() tea.Msg {
		ctx := context.Background()
		tables, nextToken, err := m.client.ListTables(ctx, token)
		return moreTablesLoadedMsg{tables: tables, nextToken: nextToken, err: err}
	}
}

func (m Model) fetchDescriptionCmd() tea.Cmd {
	name := m.currentTableName()
	if name == "" {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		desc, err := m.client.DescribeTable(ctx, name)
		return tableDescriptionMsg{desc: desc, tableName: name, err: err}
	}
}

func (m Model) fetchDescriptionForTableCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		desc, err := m.client.DescribeTable(ctx, name)
		return tableDescriptionMsg{desc: desc, tableName: name, err: err}
	}
}

func (m Model) scanItemsCmd() tea.Cmd {
	tableName := m.table
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.client.Scan(ctx, tableName, nil, scanPageSize)
		return itemsLoadedMsg{result: result, tableName: tableName, err: err}
	}
}

func (m Model) loadMoreItemsCmd() tea.Cmd {
	tableName := m.table
	lastKey := m.lastEvaluatedKey
	return func() tea.Msg {
		ctx := context.Background()
		startKey := dynamodb.RawLastEvaluatedKey(lastKey)
		result, err := m.client.Scan(ctx, tableName, startKey, scanPageSize)
		return moreItemsLoadedMsg{result: result, err: err}
	}
}

func (m Model) loadAllTablesCmd() tea.Cmd {
	token := m.tableToken
	return func() tea.Msg {
		ctx := context.Background()
		var all []dynamodb.Table
		for token != nil {
			tables, next, err := m.client.ListTables(ctx, token)
			if err != nil {
				return allTablesLoadedMsg{err: err}
			}
			all = append(all, tables...)
			token = next
		}
		return allTablesLoadedMsg{tables: all}
	}
}

func (m Model) loadAllItemsCmd() tea.Cmd {
	tableName := m.table
	lastKey := m.lastEvaluatedKey
	return func() tea.Msg {
		ctx := context.Background()
		var allItems []dynamodb.Item
		key := dynamodb.RawLastEvaluatedKey(lastKey)
		for key != nil {
			result, err := m.client.Scan(ctx, tableName, key, scanPageSize)
			if err != nil {
				return allItemsLoadedMsg{result: &dynamodb.ScanResult{}, err: err}
			}
			allItems = append(allItems, result.Items...)
			key = dynamodb.RawLastEvaluatedKey(result.LastEvaluatedKey)
		}
		return allItemsLoadedMsg{result: &dynamodb.ScanResult{Items: allItems}}
	}
}

// Searching reports whether the model has an active text input.
func (m Model) Searching() bool {
	return m.searching
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.expandedItem >= 0 {
		return "↑↓ scroll, pgup/pgdn page, esc close"
	}
	if m.searching {
		return "enter/esc close search"
	}
	if m.table == "" {
		return "↑↓ move, / search, enter open, A load all, pgup/pgdn details, y copy ARN, ctrl+r refresh"
	}
	return "↑↓ move, / search, enter expand, A load all, pgup/pgdn details, y copy ARN, ctrl+r refresh, esc/h back"
}
