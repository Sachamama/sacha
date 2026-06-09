package dynamodb

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/dynamodb"
)

var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("213")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	dimText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("44"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)
)

func lipglossJoinHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderLeft() string {
	b := &strings.Builder{}

	if m.table == "" {
		fmt.Fprintln(b, titleStyle.Render("DynamoDB Tables"))
	} else {
		fmt.Fprintln(b, titleStyle.Render(fmt.Sprintf("Table: %s", m.table)))
	}

	if m.loading {
		fmt.Fprintln(b, dimText.Render("Loading..."))
		return b.String()
	}

	// Search input
	if m.searching {
		fmt.Fprintln(b, m.search.View())
	} else if m.search.Value() != "" {
		fmt.Fprintln(b, statusStyle.Render(fmt.Sprintf("Filter: %s", m.search.Value())))
	} else {
		fmt.Fprintln(b, dimText.Render("Press / to filter"))
	}

	visibleHeight := m.listHeight()

	if m.table == "" {
		m.renderTableList(b, visibleHeight)
	} else {
		m.renderItemList(b, visibleHeight)
	}

	// Status footer
	fmt.Fprintln(b)
	if m.table == "" {
		fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Total: %d tables", len(m.tables))))
	} else {
		fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Total: %d items", len(m.filteredItems()))))
	}

	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}

	return b.String()
}

func (m Model) renderTableList(b *strings.Builder, visibleHeight int) {
	tables := m.filteredTables()
	if len(tables) == 0 {
		fmt.Fprintln(b, dimText.Render("No tables found"))
		return
	}

	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.listOffset+visibleHeight, len(tables))

	for i := m.listOffset; i < endIdx; i++ {
		t := tables[i]
		line := fmt.Sprintf("  %s", t.Name)
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(tables) {
		if m.loadingMore {
			fmt.Fprintln(b, dimText.Render("  ⟳ loading more..."))
		} else {
			fmt.Fprintln(b, dimText.Render("  ↓ more below"))
		}
	}
}

func (m Model) renderItemList(b *strings.Builder, visibleHeight int) {
	items := m.filteredItems()
	if len(items) == 0 {
		fmt.Fprintln(b, dimText.Render("No items found"))
		return
	}

	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.listOffset+visibleHeight, len(items))

	for i := m.listOffset; i < endIdx; i++ {
		item := items[i]
		line := m.formatItemLine(item)
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(items) {
		if m.loadingMore {
			fmt.Fprintln(b, dimText.Render("  ⟳ loading more..."))
		} else {
			fmt.Fprintln(b, dimText.Render("  ↓ more below"))
		}
	}
}

func (m Model) formatItemLine(item dynamodb.Item) string {
	if len(m.itemColumns) == 0 {
		return "  (empty)"
	}

	// Show key columns first, truncated to fit
	parts := make([]string, 0, len(m.itemColumns))
	for _, col := range m.itemColumns {
		v, ok := item[col]
		if !ok {
			v = "-"
		}
		if len(v) > 30 {
			v = v[:27] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", col, v))
	}

	line := "  " + strings.Join(parts, " | ")

	// Truncate if too wide for panel
	maxWidth := m.width*2/5 - 6
	if maxWidth > 0 && len(line) > maxWidth {
		line = line[:maxWidth-3] + "..."
	}

	return line
}

func (m Model) renderRight() string {
	b := &strings.Builder{}

	if m.table == "" {
		fmt.Fprintln(b, titleStyle.Render("Table Details"))
	} else {
		fmt.Fprintln(b, titleStyle.Render("Item Details"))
	}

	fmt.Fprint(b, m.detailViewport.View())

	// Show truncation hint when content overflows the detail pane.
	if m.detailViewport.TotalLineCount() > m.detailViewport.VisibleLineCount() {
		fmt.Fprintf(b, "\n%s", dimText.Render("  ↓ more details below"))
	}

	return b.String()
}

func (m Model) buildTableDetailsContent() string {
	var b strings.Builder

	tables := m.filteredTables()
	if len(tables) == 0 || m.cursor >= len(tables) {
		fmt.Fprintln(&b, dimText.Render("No table selected"))
		return b.String()
	}

	if m.description == nil {
		fmt.Fprintln(&b, dimText.Render("Loading details..."))
		return b.String()
	}

	d := m.description
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Name:"), d.Name)
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Status:"), d.Status)
	fmt.Fprintf(&b, "%s  %d\n", labelStyle.Render("Items:"), d.ItemCount)
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Size:"), formatBytes(d.TableSizeBytes))
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Created:"), d.CreationDateTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render("Billing:"), d.BillingMode)

	if d.BillingMode == "PROVISIONED" {
		fmt.Fprintf(&b, "%s  %d RCU / %d WCU\n", labelStyle.Render("Capacity:"), d.ReadCapacity, d.WriteCapacity)
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, titleStyle.Render("Key Schema"))
	for _, ks := range d.KeySchema {
		attrType := attributeType(d.AttributeDefinitions, ks.AttributeName)
		fmt.Fprintf(&b, "  %s (%s, %s)\n", ks.AttributeName, ks.KeyType, attrType)
	}

	if len(d.GlobalSecondaryIndexes) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, titleStyle.Render("Global Secondary Indexes"))
		for _, gsi := range d.GlobalSecondaryIndexes {
			fmt.Fprintf(&b, "  %s [%s]\n", gsi.Name, gsi.Status)
			for _, ks := range gsi.KeySchema {
				attrType := attributeType(d.AttributeDefinitions, ks.AttributeName)
				fmt.Fprintf(&b, "    %s (%s, %s)\n", ks.AttributeName, ks.KeyType, attrType)
			}
		}
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, dimText.Render("ARN"))
	fmt.Fprintf(&b, "arn:aws:dynamodb:*:*:table/%s\n", d.Name)

	return b.String()
}

func (m Model) buildItemDetailsContent() string {
	var b strings.Builder

	items := m.filteredItems()
	if len(items) == 0 || m.cursor >= len(items) {
		fmt.Fprintln(&b, dimText.Render("No item selected"))
		return b.String()
	}

	item := items[m.cursor]
	fmt.Fprintln(&b)

	keys := dynamodb.ItemKeys(item)
	for _, k := range keys {
		v := item[k]
		fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render(k+":"), v)
	}

	return b.String()
}

// detailViewportSize returns the width and height for the detail panel viewport.
func (m Model) detailViewportSize() (int, int) {
	rightWidth := m.width - m.width*2/5
	// panelStyle: border(2h+2v) + padding(0v+2h) => content = width-4 h, height-2 v
	contentWidth := rightWidth - 6
	if contentWidth < 10 {
		contentWidth = 10
	}
	// bodyHeight is total panel height; subtract border(2), title(1), scroll hint(1)
	vpHeight := m.bodyHeight() - 4
	if vpHeight < 3 {
		vpHeight = 3
	}
	return contentWidth, vpHeight
}

// updateDetailViewport rebuilds the detail panel viewport content and resets scroll.
func (m *Model) updateDetailViewport() {
	w, h := m.detailViewportSize()
	m.detailViewport.Width = w
	m.detailViewport.Height = h

	var content string
	if m.table == "" {
		content = m.buildTableDetailsContent()
	} else {
		content = m.buildItemDetailsContent()
	}
	m.detailViewport.SetContent(content)
	m.detailViewport.GotoTop()
}

func attributeType(defs []dynamodb.AttributeDefinition, name string) string {
	for _, d := range defs {
		if d.Name == name {
			return d.Type
		}
	}
	return "?"
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (m Model) renderExpandedItem() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Item Details"))
	fmt.Fprintf(b, "%s %s\n", dimText.Render("Table:"), m.table)
	fmt.Fprintln(b)
	fmt.Fprintln(b, m.expandedView.View())
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("↑↓/j/k scroll, pgup/pgdn page, home/end top/bottom, esc close"))

	maxWidth := m.width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	return popupStyle.Width(maxWidth).Render(b.String())
}

// initExpandedItemView creates a viewport for displaying an expanded DynamoDB item.
func initExpandedItemView(item dynamodb.Item, columns []string, width, height int) viewport.Model {
	maxWidth := width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	viewportHeight := height - 14
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	// Build content: show all attributes with full values
	var b strings.Builder
	keys := columns
	if len(keys) == 0 {
		keys = dynamodb.ItemKeys(item)
	}
	for _, k := range keys {
		v, ok := item[k]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", k, v)
	}

	vp := viewport.New(maxWidth-4, viewportHeight)
	vp.SetContent(b.String())
	return vp
}
