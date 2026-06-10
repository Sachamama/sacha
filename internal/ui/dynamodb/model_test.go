package dynamodb

import (
	"errors"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/dynamodb"
)

func newTestModel(tables []dynamodb.Table) Model {
	m := NewModel(nil, nil, cache.Key{}) // client not needed for direct state tests
	m.tables = tables
	m.loading = false
	m.width = 120
	m.height = 40
	m.updateDetailViewport()
	return m
}

func sendKey(m Model, key string) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(Model)
}

func sendSpecialKey(m Model, keyType tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(Model)
}

func sendMsg(m Model, msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestNewModel(t *testing.T) {
	m := NewModel(nil, nil, cache.Key{})
	if !m.loading {
		t.Error("expected loading to be true")
	}
	if m.expandedItem != -1 {
		t.Error("expected expandedItem to be -1")
	}
	if m.table != "" {
		t.Error("expected table to be empty")
	}
}

func TestTablesLoadedMsg(t *testing.T) {
	m := newTestModel(nil)
	m.loading = true

	tables := []dynamodb.Table{
		{Name: "users"},
		{Name: "orders"},
		{Name: "products"},
	}
	m = sendMsg(m, tablesLoadedMsg{tables: tables})

	if m.loading {
		t.Error("expected loading to be false")
	}
	if len(m.tables) != 3 {
		t.Errorf("expected 3 tables, got %d", len(m.tables))
	}
	if m.statusLine != "Loaded 3 tables" {
		t.Errorf("unexpected status: %q", m.statusLine)
	}
}

func TestTablesLoadedError(t *testing.T) {
	m := newTestModel(nil)
	m.loading = true

	m = sendMsg(m, tablesLoadedMsg{err: errors.New("access denied")})

	if m.loading {
		t.Error("expected loading to be false")
	}
	if m.statusLine != "access denied" {
		t.Errorf("unexpected status: %q", m.statusLine)
	}
}

func TestCursorNavigation(t *testing.T) {
	tables := []dynamodb.Table{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}
	m := newTestModel(tables)

	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down
	m = sendKey(m, "j")
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	m = sendKey(m, "j")
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.cursor)
	}

	// Can't go past end
	m = sendKey(m, "j")
	if m.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.cursor)
	}

	// Move up
	m = sendKey(m, "k")
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Can't go above 0
	m = sendKey(m, "k")
	m = sendKey(m, "k")
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}
}

func TestPageAndJumpNavigation(t *testing.T) {
	tables := make([]dynamodb.Table, 100)
	for i := range tables {
		tables[i] = dynamodb.Table{Name: fmt.Sprintf("t%03d", i)}
	}
	m := newTestModel(tables)

	// End jumps to the last item.
	m = sendSpecialKey(m, tea.KeyEnd)
	if m.cursor != 99 {
		t.Fatalf("end: expected cursor 99, got %d", m.cursor)
	}

	// Home jumps back to the first item.
	m = sendSpecialKey(m, tea.KeyHome)
	if m.cursor != 0 {
		t.Fatalf("home: expected cursor 0, got %d", m.cursor)
	}

	// PageDown advances by a full page.
	page := m.listHeight()
	m = sendSpecialKey(m, tea.KeyPgDown)
	if m.cursor != page {
		t.Fatalf("pgdown: expected cursor %d, got %d", page, m.cursor)
	}

	// PageUp returns to the top (single page back from page-sized offset).
	m = sendSpecialKey(m, tea.KeyPgUp)
	if m.cursor != 0 {
		t.Fatalf("pgup: expected cursor 0, got %d", m.cursor)
	}
}

func TestPageNavigationClampsAndNoopsWhenEmpty(t *testing.T) {
	// Empty list: page/jump keys must not move the cursor or panic.
	m := newTestModel(nil)
	for _, k := range []tea.KeyType{tea.KeyEnd, tea.KeyHome, tea.KeyPgDown, tea.KeyPgUp} {
		m = sendSpecialKey(m, k)
		if m.cursor != 0 {
			t.Fatalf("expected cursor to stay 0 on empty list, got %d", m.cursor)
		}
	}
}

func TestEnterTable(t *testing.T) {
	tables := []dynamodb.Table{
		{Name: "users"},
		{Name: "orders"},
	}
	m := newTestModel(tables)

	// Move to "orders" and enter
	m = sendKey(m, "j")
	m = sendSpecialKey(m, tea.KeyEnter)

	if m.table != "orders" {
		t.Errorf("expected table 'orders', got %q", m.table)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor reset to 0, got %d", m.cursor)
	}
	if !m.loading {
		t.Error("expected loading to be true after entering table")
	}
}

func TestBackNavigation(t *testing.T) {
	tables := []dynamodb.Table{
		{Name: "users"},
	}
	m := newTestModel(tables)

	// Enter a table
	m = sendSpecialKey(m, tea.KeyEnter)
	if m.table != "users" {
		t.Fatalf("expected table 'users', got %q", m.table)
	}

	// Go back with esc
	m = sendSpecialKey(m, tea.KeyEsc)
	if m.table != "" {
		t.Errorf("expected table to be empty after back, got %q", m.table)
	}

	// Enter again and go back with h
	m = sendSpecialKey(m, tea.KeyEnter)
	m = sendKey(m, "h")
	if m.table != "" {
		t.Errorf("expected table to be empty after h-back, got %q", m.table)
	}

	// Enter again and go back with backspace
	m = sendSpecialKey(m, tea.KeyEnter)
	m = sendSpecialKey(m, tea.KeyBackspace)
	if m.table != "" {
		t.Errorf("expected table to be empty after backspace-back, got %q", m.table)
	}
}

func TestBackAtRootIsNoop(t *testing.T) {
	m := newTestModel([]dynamodb.Table{{Name: "test"}})

	// Back at root should do nothing
	m = sendSpecialKey(m, tea.KeyEsc)
	if m.table != "" {
		t.Error("expected to stay at root")
	}
}

func TestSearchMode(t *testing.T) {
	tables := []dynamodb.Table{
		{Name: "users"},
		{Name: "orders"},
		{Name: "user-activity"},
	}
	m := newTestModel(tables)

	// Enter search mode
	m = sendKey(m, "/")
	if !m.searching {
		t.Error("expected searching to be true")
	}

	// Exit search with escape
	m = sendSpecialKey(m, tea.KeyEsc)
	if m.searching {
		t.Error("expected searching to be false after escape")
	}
}

func TestFilteredTables(t *testing.T) {
	tables := []dynamodb.Table{
		{Name: "users"},
		{Name: "orders"},
		{Name: "user-activity"},
	}
	m := newTestModel(tables)

	// No filter - should return all
	filtered := m.filteredTables()
	if len(filtered) != 3 {
		t.Errorf("expected 3 tables, got %d", len(filtered))
	}

	// Set search value manually (simulating search input)
	m.search.SetValue("user")
	filtered = m.filteredTables()
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered tables, got %d", len(filtered))
	}
}

func TestFilteredItems(t *testing.T) {
	m := newTestModel(nil)
	m.table = "users"
	m.items = []dynamodb.Item{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
		{"id": "3", "name": "Charlie"},
	}

	// No filter
	filtered := m.filteredItems()
	if len(filtered) != 3 {
		t.Errorf("expected 3 items, got %d", len(filtered))
	}

	// Filter by value
	m.search.SetValue("alice")
	filtered = m.filteredItems()
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered item, got %d", len(filtered))
	}
}

func TestItemsLoadedMsg(t *testing.T) {
	m := newTestModel(nil)
	m.table = "users"
	m.loading = true

	items := []dynamodb.Item{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
	}
	result := &dynamodb.ScanResult{
		Items:        items,
		ScannedCount: 2,
	}
	m = sendMsg(m, itemsLoadedMsg{result: result, tableName: "users"})

	if m.loading {
		t.Error("expected loading to be false")
	}
	if len(m.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.items))
	}
	if m.statusLine != "Loaded 2 items" {
		t.Errorf("unexpected status: %q", m.statusLine)
	}
}

func TestItemsLoadedWithPagination(t *testing.T) {
	m := newTestModel(nil)
	m.table = "users"
	m.loading = true

	result := &dynamodb.ScanResult{
		Items:            []dynamodb.Item{{"id": "1"}},
		LastEvaluatedKey: map[string]interface{}{"id": "1"},
		ScannedCount:     1,
	}
	m = sendMsg(m, itemsLoadedMsg{result: result, tableName: "users"})

	if m.lastEvaluatedKey == nil {
		t.Error("expected lastEvaluatedKey to be set")
	}
	if m.statusLine != "Loaded 1 items (loading more...)" {
		t.Errorf("unexpected status: %q", m.statusLine)
	}
}

func TestItemsIgnoredWhenTableChanged(t *testing.T) {
	m := newTestModel(nil)
	m.table = "orders" // Changed to a different table

	result := &dynamodb.ScanResult{
		Items: []dynamodb.Item{{"id": "1"}},
	}
	m = sendMsg(m, itemsLoadedMsg{result: result, tableName: "users"})

	// Items should not be set because table changed
	if len(m.items) != 0 {
		t.Errorf("expected 0 items (wrong table), got %d", len(m.items))
	}
}

func TestMoreTablesLoaded(t *testing.T) {
	tables := []dynamodb.Table{{Name: "first"}}
	m := newTestModel(tables)

	m = sendMsg(m, moreTablesLoadedMsg{
		tables: []dynamodb.Table{{Name: "second"}},
	})

	if len(m.tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(m.tables))
	}
}

func TestMoreItemsLoaded(t *testing.T) {
	m := newTestModel(nil)
	m.table = "users"
	m.items = []dynamodb.Item{{"id": "1"}}

	result := &dynamodb.ScanResult{
		Items: []dynamodb.Item{{"id": "2"}},
	}
	m = sendMsg(m, moreItemsLoadedMsg{result: result})

	if len(m.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.items))
	}
}

func TestExpandItemInItemsView(t *testing.T) {
	m := newTestModel(nil)
	m.table = "users"
	m.items = []dynamodb.Item{
		{"id": "1", "name": "Alice"},
	}
	m.itemColumns = []string{"id", "name"}

	// Press enter to expand
	m = sendSpecialKey(m, tea.KeyEnter)
	if m.expandedItem != 0 {
		t.Errorf("expected expandedItem to be 0, got %d", m.expandedItem)
	}

	// Press esc to close
	m = sendSpecialKey(m, tea.KeyEsc)
	if m.expandedItem != -1 {
		t.Errorf("expected expandedItem to be -1, got %d", m.expandedItem)
	}
}

func TestExpandItemWithSpace(t *testing.T) {
	m := newTestModel(nil)
	m.table = "users"
	m.items = []dynamodb.Item{
		{"id": "1", "name": "Alice"},
	}
	m.itemColumns = []string{"id", "name"}

	// Press space to expand
	m = sendKey(m, " ")
	if m.expandedItem != 0 {
		t.Errorf("expected expandedItem to be 0, got %d", m.expandedItem)
	}
}

func TestGetARN(t *testing.T) {
	// Table list view
	tables := []dynamodb.Table{
		{Name: "users"},
		{Name: "orders"},
	}
	m := newTestModel(tables)
	m.cursor = 1

	arn := m.getARN()
	if arn != "arn:aws:dynamodb:*:*:table/orders" {
		t.Errorf("unexpected ARN: %q", arn)
	}

	// Items view
	m.table = "users"
	arn = m.getARN()
	if arn != "arn:aws:dynamodb:*:*:table/users" {
		t.Errorf("unexpected ARN: %q", arn)
	}

	// Empty table list
	m2 := newTestModel(nil)
	arn = m2.getARN()
	if arn != "" {
		t.Errorf("expected empty ARN, got %q", arn)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel(nil)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 200, Height: 50})

	if m.width != 200 {
		t.Errorf("expected width 200, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height 50, got %d", m.height)
	}
}

func TestTableDescriptionMsg(t *testing.T) {
	tables := []dynamodb.Table{{Name: "users"}}
	m := newTestModel(tables)

	desc := &dynamodb.TableDescription{
		Name:      "users",
		Status:    "ACTIVE",
		ItemCount: 100,
	}
	m = sendMsg(m, tableDescriptionMsg{desc: desc, tableName: "users"})

	if m.description == nil {
		t.Fatal("expected description to be set")
	}
	if m.description.Name != "users" {
		t.Errorf("expected description name 'users', got %q", m.description.Name)
	}
}

func TestTableDescriptionIgnoredForWrongTable(t *testing.T) {
	tables := []dynamodb.Table{{Name: "users"}}
	m := newTestModel(tables)

	desc := &dynamodb.TableDescription{
		Name:   "orders",
		Status: "ACTIVE",
	}
	m = sendMsg(m, tableDescriptionMsg{desc: desc, tableName: "orders"})

	// Description for wrong table should not be set
	if m.description != nil {
		t.Error("expected description to be nil for non-matching table")
	}
}

func TestBuildColumns(t *testing.T) {
	m := newTestModel(nil)
	m.items = []dynamodb.Item{
		{"id": "1", "name": "Alice", "age": "30"},
		{"id": "2", "email": "bob@example.com"},
	}
	m.description = &dynamodb.TableDescription{
		KeySchema: []dynamodb.KeySchema{
			{AttributeName: "id", KeyType: "HASH"},
		},
	}

	columns := m.buildColumns()

	// "id" should be first (it's a key attribute)
	if len(columns) == 0 {
		t.Fatal("expected columns")
	}
	if columns[0] != "id" {
		t.Errorf("expected first column to be 'id', got %q", columns[0])
	}
}

func TestStatusHelp(t *testing.T) {
	m := newTestModel([]dynamodb.Table{{Name: "test"}})

	// Table list view
	help := m.StatusHelp()
	if help != "↑↓ move, pgup/pgdn page, / search, enter open, y copy ARN, ctrl+r refresh" {
		t.Errorf("unexpected help: %q", help)
	}

	// Items view
	m.table = "test"
	help = m.StatusHelp()
	if help != "↑↓ move, pgup/pgdn page, / search, enter expand, y copy ARN, ctrl+r refresh, esc/h back" {
		t.Errorf("unexpected help: %q", help)
	}

	// Searching
	m.searching = true
	help = m.StatusHelp()
	if help != "enter/esc close search" {
		t.Errorf("unexpected help: %q", help)
	}

	// Expanded item
	m.searching = false
	m.expandedItem = 0
	help = m.StatusHelp()
	if help != "↑↓ scroll, pgup/pgdn page, home/end top/bottom, esc close" {
		t.Errorf("unexpected help: %q", help)
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	// Table list view
	tables := []dynamodb.Table{
		{Name: "users"},
		{Name: "orders"},
	}
	m := newTestModel(tables)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}

	// Width 0 should show loading
	m.width = 0
	view = m.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...', got %q", view)
	}
}

func TestViewRendersItemsWithoutPanic(t *testing.T) {
	m := newTestModel(nil)
	m.table = "users"
	m.items = []dynamodb.Item{
		{"id": "1", "name": "Alice"},
	}
	m.itemColumns = []string{"id", "name"}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestClampCursor(t *testing.T) {
	m := newTestModel([]dynamodb.Table{{Name: "a"}, {Name: "b"}})
	m.cursor = 10 // Way past the end

	m.clampCursor()

	if m.cursor != 1 {
		t.Errorf("expected cursor clamped to 1, got %d", m.cursor)
	}
}

func TestClampCursorEmpty(t *testing.T) {
	m := newTestModel(nil) // No tables
	m.cursor = 5

	m.clampCursor()

	if m.cursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.cursor)
	}
}
