package logs

import (
	"encoding/json"
	"fmt"
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

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("44"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true)
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
	} else if m.search.Value() != "" {
		fmt.Fprintln(b, statusStyle.Render(fmt.Sprintf("Filter: %s", m.search.Value())))
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

	// Calculate max name width based on left panel content area.
	// Left panel: Width(leftWidth-2) with padding(0,1), so content = leftWidth - 4.
	// Each line: "[x] name" = 4 chars prefix + name.
	leftWidth := m.width * 2 / 5
	maxNameLen := leftWidth - 4 - 4 // content width minus checkbox prefix
	if maxNameLen < 10 {
		maxNameLen = 10
	}

	for i := m.listOffset; i < endIdx; i++ {
		g := groups[i]
		line := fmt.Sprintf("[%s] %s", checkbox(m.selected[g.Name]), truncate(g.Name, maxNameLen))
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
		return m.renderGroupDetails()
	}
	header := titleStyle.Render("Logs")
	if m.autoScroll {
		header += "  " + statusStyle.Render("[follow]")
	}
	if evts := m.filteredEvents(); len(evts) > 0 {
		minTime := evts[0].Timestamp
		for _, e := range evts[1:] {
			if e.Timestamp.Before(minTime) {
				minTime = e.Timestamp
			}
		}
		header += "  " + dimText.Render(fmt.Sprintf("since %s", minTime.Format("15:04:05")))
	}
	if len(m.highlightFields) > 0 {
		hlText := strings.Join(m.highlightFields, " ")
		if m.filterByHL {
			header += "  " + statusStyle.Render(fmt.Sprintf("[HL: %s] [filter: on]", hlText))
		} else {
			header += "  " + statusStyle.Render(fmt.Sprintf("[HL: %s]", hlText))
		}
	}
	return fmt.Sprintf("%s\n%s", header, m.view.View())
}

func (m Model) renderGroupDetails() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Log Group Details"))

	groups := m.filteredGroups()
	if len(groups) == 0 || m.cursor >= len(groups) {
		fmt.Fprintln(b)
		fmt.Fprintln(b, dimText.Render("No log group selected"))
		return b.String()
	}

	g := groups[m.cursor]
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Name:"), g.Name)

	if g.AccountID != "" {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Account:"), g.AccountID)
	}

	if g.RetentionDays > 0 {
		fmt.Fprintf(b, "%s  %d days\n", labelStyle.Render("Retention:"), g.RetentionDays)
	} else {
		fmt.Fprintf(b, "%s  Never expire\n", labelStyle.Render("Retention:"))
	}

	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Stored:"), formatBytes(g.StoredBytes))

	if !g.CreationTime.IsZero() {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Created:"), g.CreationTime.Format("2006-01-02 15:04:05"))
	}

	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("Press t to start tailing selected groups"))

	return b.String()
}

func renderEvents(events []logs.TailEvent, cursor, width int, showCursor bool, scrollX int, highlightFields []string) string {
	if len(events) == 0 {
		return ""
	}
	return renderEventsPlain(events, cursor, width, showCursor, scrollX, highlightFields)
}

// shortGroupName returns the last path segment of a log group name.
// e.g. "/aws/lambda/my-func" -> "my-func"
func shortGroupName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 && i < len(name)-1 {
		return name[i+1:]
	}
	return name
}

// relativeTimestamp formats a timestamp relative to a base time.
// The first event shows the full HH:MM:SS, subsequent events show +Xs offset.
func relativeTimestamp(ts, base time.Time) string {
	if ts.Equal(base) {
		return ts.Format("15:04:05")
	}
	diff := ts.Sub(base)
	secs := diff.Seconds()
	if secs < 0 {
		return ts.Format("15:04:05")
	}
	if secs < 60 {
		return fmt.Sprintf("+%.1fs", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("+%.0fm%.0fs", secs/60, float64(int(secs)%60))
	}
	return fmt.Sprintf("+%.0fh%.0fm", secs/3600, float64(int(secs)%3600)/60)
}

// highlightMessage applies highlighting to a message for the given jq-style field paths.
// Fields are specified as ".fieldName" and matching values are highlighted in the output.
func highlightMessage(msg string, fields []string) string {
	if len(fields) == 0 {
		return msg
	}
	parsed := parseJSONLog(msg)
	if parsed == nil {
		return msg
	}
	for _, field := range fields {
		key := strings.TrimPrefix(field, ".")
		if key == "" {
			continue
		}
		val, ok := parsed[key]
		if !ok {
			continue
		}
		valStr := formatValue(val)
		if valStr == "" {
			continue
		}
		// Highlight occurrences of the value in the message
		msg = strings.ReplaceAll(msg, valStr, highlightStyle.Render(valStr))
	}
	return msg
}

func renderEventsPlain(events []logs.TailEvent, cursor, width int, showCursor bool, scrollX int, highlightFields []string) string {
	var b strings.Builder

	// Find the min (earliest) timestamp as the base for relative display
	var baseTime time.Time
	if len(events) > 0 {
		baseTime = events[0].Timestamp
		for _, e := range events[1:] {
			if e.Timestamp.Before(baseTime) {
				baseTime = e.Timestamp
			}
		}
	}

	// Calculate column widths
	timeWidth := 8 // enough for "HH:MM:SS" or "+XXmXXs"
	groupWidth := 20
	separators := 6 // " │ " twice
	pointer := 1    // "▶" or " "
	msgWidth := width - timeWidth - groupWidth - separators - pointer
	if msgWidth < 20 {
		msgWidth = 20
	}

	for i, e := range events {
		ts := padRight(relativeTimestamp(e.Timestamp, baseTime), timeWidth)
		group := padRight(truncate(shortGroupName(e.LogGroup), groupWidth), groupWidth)
		msg := strings.TrimSpace(e.Message)
		msg = highlightMessage(msg, highlightFields)
		msg = truncate(msg, msgWidth+scrollX)
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

func checkbox(selected bool) string {
	if selected {
		return "x"
	}
	return " "
}

func (m Model) renderHighlightPopup() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Highlight Fields"))
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("Enter jq-style field paths separated by spaces:"))
	fmt.Fprintln(b, dimText.Render("  .level .message .statusCode"))
	fmt.Fprintln(b)
	fmt.Fprintln(b, m.highlightInput.View())
	fmt.Fprintln(b)
	if len(m.highlightFields) > 0 {
		fmt.Fprintf(b, "%s %s\n", dimText.Render("Active:"), strings.Join(m.highlightFields, " "))
	}
	fmt.Fprintln(b, dimText.Render("Enter to apply, Esc to cancel, empty to clear"))
	return popupStyle.Render(b.String())
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
	fmt.Fprintln(b, dimText.Render("↑↓/j/k scroll, pgup/pgdn page, g/G top/bottom, y copy, esc close"))

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

func (m Model) renderDeleteConfirm() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, warnStyle.Render("Delete Log Groups"))
	fmt.Fprintln(b)
	fmt.Fprintf(b, "Delete %d log group(s)?\n", len(m.deleteTargets))
	fmt.Fprintln(b)
	for i, name := range m.deleteTargets {
		if i >= 10 {
			fmt.Fprintf(b, "  ... and %d more\n", len(m.deleteTargets)-10)
			break
		}
		fmt.Fprintf(b, "  %s\n", name)
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("y to confirm, n/esc to cancel"))
	return popupStyle.Render(b.String())
}

func (m Model) renderRetentionPicker() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Set Retention Policy"))
	fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Apply to %d selected group(s)", len(m.selectedGroups()))))
	fmt.Fprintln(b)
	for i, opt := range retentionOptions {
		if i == m.retentionCursor {
			fmt.Fprintf(b, "  %s %s\n", hoverPointer, cursorStyle.Render(opt.label))
		} else {
			fmt.Fprintf(b, "    %s\n", opt.label)
		}
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("↑↓ move, enter select, esc cancel"))
	return popupStyle.Render(b.String())
}

func (m Model) renderAccountPicker() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Select Account"))
	fmt.Fprintln(b, dimText.Render("Filter log groups by source account"))
	fmt.Fprintln(b)
	for i, opt := range m.accountOptions {
		label := opt
		if i == 0 {
			// "All Accounts" option
			label = opt
		}
		// Show current selection
		if (m.selectedAccount == "" && i == 0) || opt == m.selectedAccount {
			label += " ●"
		}
		if i == m.accountCursor {
			fmt.Fprintf(b, "  %s %s\n", hoverPointer, cursorStyle.Render(label))
		} else {
			fmt.Fprintf(b, "    %s\n", label)
		}
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("↑↓ move, enter select, esc cancel"))
	return popupStyle.Render(b.String())
}
