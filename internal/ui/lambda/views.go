package lambda

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/lambda"
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

func (m Model) renderLeft() string {
	b := &strings.Builder{}

	fmt.Fprintln(b, titleStyle.Render("Lambda Functions"))

	if m.loading {
		fmt.Fprintln(b, dimText.Render("Loading..."))
		return b.String()
	}

	// Search input
	if m.searching {
		fmt.Fprintln(b, m.search.View())
	} else {
		fmt.Fprintln(b, dimText.Render("Press / to filter"))
	}

	visibleHeight := m.listHeight()
	m.renderFunctionList(b, visibleHeight)

	// Status footer
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Total: %d functions", len(m.functions))))

	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}

	return b.String()
}

func (m Model) renderFunctionList(b *strings.Builder, visibleHeight int) {
	functions := m.filteredFunctions()
	if len(functions) == 0 {
		fmt.Fprintln(b, dimText.Render("No functions found"))
		return
	}

	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.listOffset+visibleHeight, len(functions))

	for i := m.listOffset; i < endIdx; i++ {
		f := functions[i]
		runtime := f.Runtime
		if runtime == "" {
			runtime = "n/a"
		}
		// Build plain text first, truncate, then apply styling
		// to avoid breaking ANSI escape codes during truncation.
		line := fmt.Sprintf("  %s  %s", f.Name, runtime)

		maxWidth := m.width/2 - 6
		truncated := false
		if maxWidth > 0 && len(line) > maxWidth {
			line = line[:maxWidth-3] + "..."
			truncated = true
		}

		if i == m.cursor {
			line = cursorStyle.Render(line)
		} else if !truncated {
			// Apply dim styling to runtime portion only when not truncated
			line = fmt.Sprintf("  %s  %s", f.Name, dimText.Render(runtime))
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(functions) {
		if m.loadingMore {
			fmt.Fprintln(b, dimText.Render("  ⟳ loading more..."))
		} else {
			fmt.Fprintln(b, dimText.Render("  ↓ more below"))
		}
	}
}

func (m Model) renderRight() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Function Details"))

	functions := m.filteredFunctions()
	if len(functions) == 0 || m.cursor >= len(functions) {
		fmt.Fprintln(b, dimText.Render("No function selected"))
		return b.String()
	}

	if m.details == nil {
		fmt.Fprintln(b, dimText.Render("Loading details..."))
		return b.String()
	}

	d := m.details
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Name:"), d.Name)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("State:"), d.State)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Runtime:"), d.Runtime)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Handler:"), d.Handler)
	fmt.Fprintf(b, "%s  %d MB\n", labelStyle.Render("Memory:"), d.Memory)
	fmt.Fprintf(b, "%s  %d s\n", labelStyle.Render("Timeout:"), d.Timeout)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Code Size:"), formatBytes(d.CodeSize))
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Package:"), d.PackageType)

	if !d.LastModified.IsZero() {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Modified:"), d.LastModified.Format("2006-01-02 15:04:05"))
	}

	if d.EphemeralStorage > 0 {
		fmt.Fprintf(b, "%s  %d MB\n", labelStyle.Render("Storage:"), d.EphemeralStorage)
	}

	if len(d.Architectures) > 0 {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Arch:"), strings.Join(d.Architectures, ", "))
	}

	if d.Description != "" {
		fmt.Fprintln(b)
		fmt.Fprintln(b, titleStyle.Render("Description"))
		fmt.Fprintf(b, "  %s\n", d.Description)
	}

	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("Role"))
	fmt.Fprintf(b, "  %s\n", d.Role)

	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("ARN"))
	fmt.Fprintf(b, "  %s\n", d.ARN)

	if len(d.Layers) > 0 {
		fmt.Fprintln(b)
		fmt.Fprintln(b, titleStyle.Render("Layers"))
		for _, l := range d.Layers {
			fmt.Fprintf(b, "  %s\n", l)
		}
	}

	return b.String()
}

func (m Model) renderExpandedFunction() string {
	b := &strings.Builder{}
	fmt.Fprintln(b, titleStyle.Render("Function Details"))

	functions := m.filteredFunctions()
	if m.expandedFunc < len(functions) {
		fmt.Fprintf(b, "%s %s\n", dimText.Render("Function:"), functions[m.expandedFunc].Name)
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, m.expandedView.View())
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("↑↓/j/k scroll, pgup/pgdn page, esc close"))

	maxWidth := m.width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	return popupStyle.Width(maxWidth).Render(b.String())
}

// initExpandedFunctionView creates a viewport for displaying expanded function details.
func initExpandedFunctionView(details *lambda.FunctionDetails, name string, width, height int) viewport.Model {
	maxWidth := width - 10
	if maxWidth > 100 {
		maxWidth = 100
	}
	viewportHeight := height - 14
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	var b strings.Builder
	if details == nil {
		fmt.Fprintf(&b, "Name: %s\n", name)
		fmt.Fprintln(&b, "(Details not loaded)")
	} else {
		fmt.Fprintf(&b, "Name: %s\n", details.Name)
		fmt.Fprintf(&b, "ARN: %s\n", details.ARN)
		fmt.Fprintf(&b, "State: %s\n", details.State)
		fmt.Fprintf(&b, "Runtime: %s\n", details.Runtime)
		fmt.Fprintf(&b, "Handler: %s\n", details.Handler)
		fmt.Fprintf(&b, "Memory: %d MB\n", details.Memory)
		fmt.Fprintf(&b, "Timeout: %d s\n", details.Timeout)
		fmt.Fprintf(&b, "Code Size: %s\n", formatBytes(details.CodeSize))
		fmt.Fprintf(&b, "Package: %s\n", details.PackageType)

		if !details.LastModified.IsZero() {
			fmt.Fprintf(&b, "Last Modified: %s\n", details.LastModified.Format("2006-01-02 15:04:05"))
		}
		if details.EphemeralStorage > 0 {
			fmt.Fprintf(&b, "Ephemeral Storage: %d MB\n", details.EphemeralStorage)
		}
		if len(details.Architectures) > 0 {
			fmt.Fprintf(&b, "Architectures: %s\n", strings.Join(details.Architectures, ", "))
		}
		if details.Description != "" {
			fmt.Fprintf(&b, "Description: %s\n", details.Description)
		}
		fmt.Fprintf(&b, "Role: %s\n", details.Role)

		if len(details.Layers) > 0 {
			fmt.Fprintln(&b, "\nLayers:")
			for _, l := range details.Layers {
				fmt.Fprintf(&b, "  %s\n", l)
			}
		}

		if len(details.Environment) > 0 {
			fmt.Fprintln(&b, "\nEnvironment Variables:")
			keys := make([]string, 0, len(details.Environment))
			for k := range details.Environment {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&b, "  %s=%s\n", k, details.Environment[k])
			}
		}
	}

	vp := viewport.New(maxWidth-4, viewportHeight)
	vp.SetContent(b.String())
	return vp
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
