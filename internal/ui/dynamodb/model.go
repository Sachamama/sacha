package dynamodb

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string
}

// NewModel creates a new DynamoDB browser model.
func NewModel(client *dynamodb.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	return Model{
		client:  client,
		search:  ti,
		loading: true,
	}
}

// Init initializes the model by loading tables.
func (m Model) Init() tea.Cmd {
	return m.loadTablesCmd()
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tablesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.tables = msg.tables
		m.tableToken = msg.nextToken
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
		m.statusLine = fmt.Sprintf("Loaded %d tables", len(m.tables))
		return m, nil

	case tableDescriptionMsg:
		if msg.err != nil {
			return m, nil
		}
		if m.currentTableName() == msg.tableName || m.table == msg.tableName {
			m.description = msg.desc
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
			return m, nil
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
		maxIdx := m.maxCursorIndex()
		if m.cursor < maxIdx {
			m.cursor++
			m.ensureCursorVisible()
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
	case "enter":
		return m.handleEnter()
	case "backspace", "esc":
		return m.handleBack()
	case "/":
		m.searching = true
		m.search.SetValue("")
		return m, m.search.Focus()
	}

	return m, nil
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

	return lipglossJoinHorizontal(left, right)
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
	if m.cursor >= m.listOffset+visibleHeight {
		m.listOffset = m.cursor - visibleHeight + 1
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
		m.table = tables[m.cursor].Name
		m.cursor = 0
		m.listOffset = 0
		m.search.SetValue("")
		m.loading = true
		m.items = nil
		m.lastEvaluatedKey = nil
		m.itemColumns = nil
		return m, tea.Batch(m.scanItemsCmd(), m.fetchDescriptionForTableCmd(m.table))
	}
	return m, nil
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	if m.table == "" {
		return m, nil
	}
	m.table = ""
	m.cursor = 0
	m.listOffset = 0
	m.items = nil
	m.lastEvaluatedKey = nil
	m.itemColumns = nil
	m.description = nil
	m.search.SetValue("")
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

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.searching {
		return "enter/esc close search"
	}
	if m.table == "" {
		return "↑↓ move, / search, enter open"
	}
	return "↑↓ move, / search, esc back"
}
