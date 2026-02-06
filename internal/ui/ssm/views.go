package ssm

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/ssm"
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

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("44"))

	folderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	secureStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	popupBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("213")).
				Padding(1, 2)
)

func joinHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderLeft() string {
	b := &strings.Builder{}

	// Breadcrumb header
	breadcrumb := m.renderBreadcrumb()
	fmt.Fprintln(b, titleStyle.Render(breadcrumb))

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

	// List items
	m.renderParamList(b, visibleHeight)

	// Status footer
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Total: %d items", len(m.filteredParams()))))

	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}

	return b.String()
}

func (m Model) renderBreadcrumb() string {
	if len(m.path) == 0 {
		return "ssm://"
	}
	return "ssm:/" + m.path[len(m.path)-1]
}

func (m Model) renderParamList(b *strings.Builder, visibleHeight int) {
	items := m.filteredParams()
	if len(items) == 0 {
		fmt.Fprintln(b, dimText.Render("No parameters found"))
		return
	}

	// Show scroll indicator if needed
	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.listOffset+visibleHeight, len(items))

	for i := m.listOffset; i < endIdx; i++ {
		item := items[i]
		var line string

		if item.IsPrefix {
			line = fmt.Sprintf("  %s/", folderStyle.Render(item.Value))
		} else {
			displayName := item.DisplayName()
			typeHint := dimText.Render(fmt.Sprintf("[%s]", item.Type))
			line = fmt.Sprintf("  %s  %s", displayName, typeHint)
		}

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

func (m Model) renderRight() string {
	b := &strings.Builder{}

	fmt.Fprintln(b, titleStyle.Render("Parameter Details"))

	items := m.filteredParams()
	if len(items) == 0 || m.cursor >= len(items) {
		fmt.Fprintln(b, dimText.Render("No parameter selected"))
		return b.String()
	}

	item := items[m.cursor]

	if item.IsPrefix {
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Path:"), item.Name)
		fmt.Fprintln(b)
		fmt.Fprintln(b, dimText.Render("Press enter to browse"))
		return b.String()
	}

	// Show basic info from list
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Name:"), item.Name)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Type:"), renderType(item.Type))

	if m.details != nil {
		// Show full details (fetched with decryption)
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s\n", labelStyle.Render("Value:"))
		if m.details.Type == "SecureString" {
			fmt.Fprintf(b, "%s\n", m.details.Value)
		} else {
			fmt.Fprintf(b, "%s\n", m.details.Value)
		}

		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s  %d\n", labelStyle.Render("Version:"), m.details.Version)
		if !m.details.LastModifiedDate.IsZero() {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Modified:"), m.details.LastModifiedDate.Format("2006-01-02 15:04:05"))
		}
		if m.details.DataType != "" {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("DataType:"), m.details.DataType)
		}
		if m.details.ARN != "" {
			fmt.Fprintln(b)
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("ARN:"), m.details.ARN)
		}
	} else {
		fmt.Fprintln(b, dimText.Render("Loading details..."))
	}

	return b.String()
}

func (m Model) renderExpandedContent(item ssm.Parameter) string {
	b := &strings.Builder{}

	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Name:"), item.Name)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Type:"), renderType(item.Type))

	if m.details != nil {
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s\n", labelStyle.Render("Value:"))
		fmt.Fprintln(b, m.details.Value)

		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s  %d\n", labelStyle.Render("Version:"), m.details.Version)
		if !m.details.LastModifiedDate.IsZero() {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Modified:"), m.details.LastModifiedDate.Format("2006-01-02 15:04:05"))
		}
		if m.details.DataType != "" {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("DataType:"), m.details.DataType)
		}
		if m.details.ARN != "" {
			fmt.Fprintln(b)
			fmt.Fprintf(b, "%s\n", labelStyle.Render("ARN:"))
			fmt.Fprintln(b, m.details.ARN)
		}
	} else {
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Version:"), fmt.Sprintf("%d", item.Version))
		if !item.LastModifiedDate.IsZero() {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Modified:"), item.LastModifiedDate.Format("2006-01-02 15:04:05"))
		}
		if item.ARN != "" {
			fmt.Fprintln(b)
			fmt.Fprintf(b, "%s\n", labelStyle.Render("ARN:"))
			fmt.Fprintln(b, item.ARN)
		}
	}

	return b.String()
}

func (m Model) renderExpandedPopup(background string) string {
	items := m.filteredParams()
	if m.expandedParam >= len(items) {
		return background
	}

	item := items[m.expandedParam]

	// Build popup content
	header := titleStyle.Render("Parameter: " + item.DisplayName())
	body := m.expandedView.View()
	footer := dimText.Render("↑↓/j/k scroll • pgup/pgdn page • esc close")

	content := header + "\n" + body + "\n" + footer

	popup := popupBorderStyle.
		Width(m.expandedWidth()).
		Height(m.expandedHeight() + 4).
		Render(content)

	// Center the popup
	return lipgloss.Place(
		m.width, m.height-4,
		lipgloss.Center, lipgloss.Center,
		popup,
	)
}

func renderType(t string) string {
	if t == "SecureString" {
		return secureStyle.Render(t)
	}
	return t
}
