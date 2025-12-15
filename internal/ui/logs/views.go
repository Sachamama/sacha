package logs

import (
	"fmt"
	"strings"
	"time"

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

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("51"))

	dimText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("44"))
)

func (m Model) renderGroups() string {
	b := &strings.Builder{}
	header := titleStyle.Render("Log Groups")
	if m.loading {
		header += " " + dimText.Render("(loading...)")
	}
	fmt.Fprintln(b, header)
	if m.searching {
		fmt.Fprintln(b, m.search.View())
	} else {
		fmt.Fprintln(b, "Press / to search")
	}
	groups := m.filteredGroups()
	if len(groups) == 0 {
		fmt.Fprintln(b, "no log groups")
		return b.String()
	}

	for i, g := range groups {
		line := fmt.Sprintf("[%s] %s", checkbox(m.selected[g.Name]), g.Name)
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		if m.selected[g.Name] {
			line = selectedStyle.Render(line)
		}
		fmt.Fprintln(b, line)
	}

	fmt.Fprintf(b, "\n%s\n", dimText.Render(fmt.Sprintf("Selected: %d | Total: %d", m.selectedCount(), len(m.logGroups))))
	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}
	return b.String()
}

func (m Model) renderTail() string {
	if !m.tailing {
		return fmt.Sprintf("%s\n%s", titleStyle.Render("Tail"), dimText.Render("Press t to start tailing selected groups"))
	}
	header := fmt.Sprintf("%s %s", titleStyle.Render("Tail"), dimText.Render("(pgup/pgdn scroll, q/esc stop)"))
	return fmt.Sprintf("%s\n%s", header, m.view.View())
}

func renderEvents(events []logs.TailEvent) string {
	var b strings.Builder
	for _, e := range events {
		fmt.Fprintf(&b, "%s | %s | %s\n", e.Timestamp.Format(time.RFC3339), e.LogGroup, strings.TrimSpace(e.Message))
	}
	return b.String()
}

func checkbox(selected bool) string {
	if selected {
		return "x"
	}
	return " "
}
