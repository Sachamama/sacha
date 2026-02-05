package logs

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/logs"
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

	// hoverStyle for table row selection - subtle background highlight
	hoverStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255"))

	// hoverPointer shown at the start of selected row
	hoverPointer = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			Render("▶")

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("51"))

	dimText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	// tableHeaderStyle for table column headers
	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("44"))

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)
)

func (m Model) renderGroups() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Log Groups"))

	if m.loading {
		fmt.Fprintln(b, dimText.Render("Loading..."))
		return b.String()
	}

	if m.searching {
		fmt.Fprintln(b, m.search.View())
	} else {
		fmt.Fprintln(b, dimText.Render("Press / to filter"))
	}
	groups := m.filteredGroups()
	if len(groups) == 0 {
		fmt.Fprintln(b, dimText.Render("No log groups found"))
		return b.String()
	}

	visibleHeight := m.listHeight()

	showCursor := !m.tailing || m.focus == panelGroups

	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := m.listOffset + visibleHeight
	if endIdx > len(groups) {
		endIdx = len(groups)
	}

	for i := m.listOffset; i < endIdx; i++ {
		g := groups[i]
		line := fmt.Sprintf("[%s] %s", checkbox(m.selected[g.Name]), g.Name)
		if showCursor && i == m.cursor {
			line = cursorStyle.Render(line)
		} else if m.selected[g.Name] {
			line = selectedStyle.Render(line)
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(groups) {
		if m.loadingMore {
			fmt.Fprintln(b, dimText.Render("  ⟳ loading more..."))
		} else {
			fmt.Fprintln(b, dimText.Render("  ↓ more below"))
		}
	}

	fmt.Fprintf(b, "\n%s\n", dimText.Render(fmt.Sprintf("Selected: %d | Total: %d", m.selectedCount(), len(m.logGroups))))
	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}
	return b.String()
}

func (m Model) renderTail() string {
	if !m.tailing {
		return fmt.Sprintf("%s\n%s", titleStyle.Render("Logs"), dimText.Render("Press t to start tailing selected groups"))
	}
	return fmt.Sprintf("%s\n%s", titleStyle.Render("Logs"), m.view.View())
}

func renderEvents(events []logs.TailEvent, jsonView bool, cursor, width int, showCursor bool, scrollX int) string {
	if len(events) == 0 {
		return ""
	}

	// If JSON view is disabled, always use plain rendering
	if !jsonView {
		return renderEventsPlain(events, cursor, width, showCursor, scrollX)
	}

	// Check if events contain JSON - sample first few
	hasJSON := false
	var allKeys []string
	keySet := make(map[string]bool)

	for i, e := range events {
		if i > 10 {
			break // Sample first 10 to detect pattern
		}
		if parsed := parseJSONLog(e.Message); parsed != nil {
			hasJSON = true
			for k := range parsed {
				if !keySet[k] {
					keySet[k] = true
					allKeys = append(allKeys, k)
				}
			}
		}
	}

	if !hasJSON {
		// Fall back to original rendering
		return renderEventsPlain(events, cursor, width, showCursor, scrollX)
	}

	// Render as table
	return renderEventsTable(events, allKeys, cursor, width, showCursor, scrollX)
}

func renderEventsPlain(events []logs.TailEvent, cursor, width int, showCursor bool, scrollX int) string {
	var b strings.Builder

	// Calculate column widths
	timeWidth := 25 // RFC3339 format
	groupWidth := 20
	separators := 6 // " | " twice
	pointer := 1    // "▶" or " "
	msgWidth := width - timeWidth - groupWidth - separators - pointer
	if msgWidth < 20 {
		msgWidth = 20
	}

	for i, e := range events {
		ts := padRight(e.Timestamp.Format(time.RFC3339), timeWidth)
		group := padRight(truncate(e.LogGroup, groupWidth), groupWidth)
		msg := truncate(strings.TrimSpace(e.Message), msgWidth+scrollX)
		line := fmt.Sprintf("%s │ %s │ %s", ts, group, msg)
		line = scrollLine(line, scrollX)
		if showCursor && i == cursor {
			fmt.Fprintln(&b, hoverPointer+hoverStyle.Render(line))
		} else {
			fmt.Fprintln(&b, " "+line)
		}
	}
	return b.String()
}

func renderEventsTable(events []logs.TailEvent, keys []string, cursor, width int, showCursor bool, scrollX int) string {
	// Sort keys for consistent column order (timestamp-like fields first)
	sort.Slice(keys, func(i, j int) bool {
		priority := map[string]int{
			"timestamp": 0, "time": 1, "level": 2,
			"message": 3, "msg": 4,
		}
		pi, oki := priority[strings.ToLower(keys[i])]
		pj, okj := priority[strings.ToLower(keys[j])]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return keys[i] < keys[j]
	})

	// Build columns: TIME + GROUP + JSON keys
	columns := append([]string{"TIME", "GROUP"}, keys...)
	numCols := len(columns)

	// Pre-compute all cell values and track column widths
	type row struct {
		cells []string
	}
	rows := make([]row, len(events))
	colWidths := make([]int, numCols)

	// Initialize widths from headers
	for i, col := range columns {
		colWidths[i] = len(col)
	}

	// Compute cell values and update widths
	for i, e := range events {
		cells := make([]string, numCols)
		cells[0] = e.Timestamp.Format("15:04:05")
		cells[1] = e.LogGroup

		parsed := parseJSONLog(e.Message)
		if parsed == nil {
			// Non-JSON row - show raw message in third column
			if numCols > 2 {
				cells[2] = strings.TrimSpace(e.Message)
			}
		} else {
			for j, k := range keys {
				if v, ok := parsed[k]; ok {
					cells[j+2] = formatValue(v)
				}
			}
		}

		rows[i] = row{cells: cells}

		// Update column widths
		for j, cell := range cells {
			if len(cell) > colWidths[j] {
				colWidths[j] = len(cell)
			}
		}
	}

	// Calculate available width (subtract pointer, separators, padding)
	pointer := 2                                       // "▶ " or "  "
	separators := (numCols - 1) * 3                    // " │ " between columns
	availableWidth := width - pointer - separators - 2 // 2 for safety margin

	// Distribute width: fixed columns first, then expand flexible ones
	totalCurrentWidth := 0
	for _, w := range colWidths {
		totalCurrentWidth += w
	}

	if totalCurrentWidth < availableWidth {
		// Expand last column (usually message) to fill space
		extra := availableWidth - totalCurrentWidth
		colWidths[numCols-1] += extra
	} else {
		// Cap columns proportionally
		const maxColWidth = 40
		for i := range colWidths {
			if colWidths[i] > maxColWidth && i < numCols-1 {
				colWidths[i] = maxColWidth
			}
		}
		// Recalculate and give remaining to last column
		usedWidth := 0
		for i := 0; i < numCols-1; i++ {
			usedWidth += colWidths[i]
		}
		lastColWidth := availableWidth - usedWidth
		if lastColWidth < 10 {
			lastColWidth = 10
		}
		colWidths[numCols-1] = lastColWidth
	}

	// Render table
	var b strings.Builder

	// Top padding
	fmt.Fprintln(&b)

	// Header row
	headerParts := make([]string, 0, len(columns))
	for i, col := range columns {
		headerParts = append(headerParts, padRight(col, colWidths[i]))
	}
	headerLine := scrollLine(strings.Join(headerParts, "   "), scrollX)
	fmt.Fprintln(&b, " "+tableHeaderStyle.Render(headerLine))

	// Data rows
	for i, r := range rows {
		rowParts := make([]string, 0, len(r.cells))
		for j, cell := range r.cells {
			rowParts = append(rowParts, padRight(truncate(cell, colWidths[j]), colWidths[j]))
		}
		line := scrollLine(strings.Join(rowParts, "   "), scrollX)
		if showCursor && i == cursor {
			fmt.Fprintln(&b, hoverPointer+hoverStyle.Render(line))
		} else {
			fmt.Fprintln(&b, " "+line)
		}
	}

	return b.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// scrollLine removes scrollX characters from the beginning of a line
func scrollLine(s string, scrollX int) string {
	if scrollX <= 0 {
		return s
	}
	runes := []rune(s)
	if scrollX >= len(runes) {
		return ""
	}
	return string(runes[scrollX:])
}

// isJSON checks if a string is valid JSON object
func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) > 0 && s[0] == '{'
}

// parseJSONLog attempts to parse a log message as JSON
// Returns nil if not valid JSON object
func parseJSONLog(msg string) map[string]interface{} {
	msg = strings.TrimSpace(msg)
	if !isJSON(msg) {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &result); err != nil {
		return nil
	}
	return result
}

// formatValue converts a value to string, stringifying objects/arrays
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case map[string]interface{}, []interface{}:
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func checkbox(selected bool) string {
	if selected {
		return "x"
	}
	return " "
}

func (m Model) renderCreatePopup() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Create Log Group"))
	fmt.Fprintln(b)
	fmt.Fprintln(b, m.createInput.View())
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("Enter to create, Esc to cancel"))
	return popupStyle.Render(b.String())
}

func (m Model) renderExpandedEvent(e logs.TailEvent) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Log Event"))
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s %s\n", dimText.Render("Time:"), e.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(b, "%s %s\n", dimText.Render("Group:"), e.LogGroup)
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("Message:"))
	fmt.Fprintln(b, m.expandedView.View())
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("↑↓/j/k scroll, pgup/pgdn page, esc close"))

	// Size popup based on content
	maxWidth := m.width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	return popupStyle.Width(maxWidth).Render(b.String())
}

// initExpandedView creates a viewport for displaying an expanded log event.
func initExpandedView(e logs.TailEvent, width, height int) viewport.Model {
	// Calculate popup dimensions
	maxWidth := width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	// Height for viewport (leave room for header, metadata, and footer)
	viewportHeight := height - 14
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	// Format message - pretty print if JSON
	msg := strings.TrimSpace(e.Message)
	if parsed := parseJSONLog(msg); parsed != nil {
		pretty, err := json.MarshalIndent(parsed, "", "  ")
		if err == nil {
			msg = string(pretty)
		}
	}

	vp := viewport.New(maxWidth-4, viewportHeight)
	vp.SetContent(msg)
	return vp
}
