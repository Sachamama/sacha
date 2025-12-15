package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	awsx "github.com/sachamama/sacha/internal/aws"
)

// optionSelector is a lightweight searchable picker used for region/service selection.
type optionSelector struct {
	title    string
	items    []string
	filtered []string
	cursor   int
	active   bool
	input    textinput.Model
}

func newOptionSelector(title string, items []string) optionSelector {
	in := textinput.New()
	in.Placeholder = "type to filter"
	return optionSelector{
		title:    title,
		items:    items,
		filtered: append([]string{}, items...),
		input:    in,
	}
}

func (s *optionSelector) open(items []string, current string) {
	s.items = append([]string{}, items...)
	s.filtered = append([]string{}, items...)
	s.cursor = 0
	for i, v := range s.filtered {
		if v == current {
			s.cursor = i
			break
		}
	}
	s.input.SetValue("")
	s.active = true
}

func (s *optionSelector) update(msg tea.KeyMsg) (string, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		s.active = false
		return "", nil
	case tea.KeyEnter:
		choice := s.current()
		s.active = false
		return choice, nil
	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
	case tea.KeyDown:
		if s.cursor < len(s.filtered)-1 {
			s.cursor++
		}
	}
	var cmd tea.Cmd
	prev := s.input.Value()
	s.input, cmd = s.input.Update(msg)
	if s.input.Value() != prev {
		s.applyFilter()
	}
	return "", cmd
}

func (s *optionSelector) applyFilter() {
	q := strings.ToLower(s.input.Value())
	s.filtered = s.filtered[:0]
	for _, item := range s.items {
		if strings.Contains(strings.ToLower(item), q) {
			s.filtered = append(s.filtered, item)
		}
	}
	if len(s.filtered) == 0 {
		s.cursor = 0
	} else if s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
}

func (s *optionSelector) current() string {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered) {
		return ""
	}
	return s.filtered[s.cursor]
}

func (s optionSelector) View(width int) string {
	box := lipgloss.NewStyle().Width(width).Padding(1).Border(lipgloss.RoundedBorder())
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", s.title)
	fmt.Fprintf(&b, "%s\n\n", s.input.View())
	if len(s.filtered) == 0 {
		fmt.Fprintln(&b, "No matches")
	} else {
		for i, item := range s.filtered {
			cursor := "  "
			if i == s.cursor {
				cursor = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", cursor, item)
		}
	}
	fmt.Fprintln(&b, "\n↑/↓ to move, type to filter, Enter to select, Esc to cancel")
	return box.Render(b.String())
}

func (m Model) overlayView(header, overlay string) string {
	if m.width == 0 || m.height == 0 {
		return overlay
	}
	containerHeight := m.height - 1 // reserve a line for the header already printed
	if containerHeight < 4 {
		containerHeight = m.height
	}
	container := lipgloss.NewStyle().Width(m.width).Height(containerHeight)
	popup := lipgloss.Place(m.width, containerHeight, lipgloss.Center, lipgloss.Center, overlay)
	return fmt.Sprintf("%s\n%s", header, container.Render(popup))
}

func minWidth(actual, limit int) int {
	switch {
	case actual <= 0:
		return limit
	case actual < limit:
		if actual-2 > 10 {
			return actual - 2
		}
		return actual
	}
	return limit
}

var awsRegions = []string{
	"af-south-1", "ap-east-1", "ap-south-1", "ap-south-2",
	"ap-southeast-1", "ap-southeast-2", "ap-southeast-3", "ap-northeast-1",
	"ap-northeast-2", "ap-northeast-3", "ca-central-1", "ca-west-1",
	"eu-central-1", "eu-central-2", "eu-west-1", "eu-west-2",
	"eu-west-3", "eu-north-1", "eu-south-1", "eu-south-2",
	"il-central-1", "me-south-1", "me-central-1",
	"sa-east-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2",
}

func serviceNames(svcs map[string]awsx.Service) []string {
	names := make([]string, 0, len(svcs))
	for name := range svcs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
