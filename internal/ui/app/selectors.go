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

// viewMode represents the current view in a selector with multiple views.
type viewMode int

const (
	viewCommon viewMode = iota
	viewAll
)

// optionSelector is a lightweight searchable picker used for region/service selection.
type optionSelector struct {
	title     string
	items     []string // all items
	commonSet []string // common items subset (optional)
	filtered  []string
	cursor    int
	active    bool
	input     textinput.Model
	viewMode  viewMode
	hasViews  bool // whether this selector supports view switching
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

func newOptionSelectorWithViews(title string, items, common []string) optionSelector {
	in := textinput.New()
	in.Placeholder = "type to filter"
	return optionSelector{
		title:     title,
		items:     items,
		commonSet: common,
		filtered:  append([]string{}, common...),
		input:     in,
		viewMode:  viewCommon,
		hasViews:  true,
	}
}

func (s *optionSelector) open(items []string, current string) {
	s.items = append([]string{}, items...)
	if s.hasViews {
		s.viewMode = viewCommon
		s.filtered = append([]string{}, s.commonSet...)
	} else {
		s.filtered = append([]string{}, items...)
	}
	s.cursor = 0
	found := false
	for i, v := range s.filtered {
		if v == current {
			s.cursor = i
			found = true
			break
		}
	}
	// If the current value is not in the common view, switch to All
	// so the user sees their current selection highlighted.
	if s.hasViews && !found && current != "" {
		s.viewMode = viewAll
		s.filtered = append(s.filtered[:0], items...)
		for i, v := range s.filtered {
			if v == current {
				s.cursor = i
				break
			}
		}
	}
	s.input.SetValue("")
	s.active = true
}

func (s *optionSelector) currentViewItems() []string {
	if !s.hasViews || s.viewMode == viewAll {
		return s.items
	}
	return s.commonSet
}

func (s *optionSelector) switchView() {
	if !s.hasViews {
		return
	}
	if s.viewMode == viewCommon {
		s.viewMode = viewAll
	} else {
		s.viewMode = viewCommon
	}
	s.applyFilter()
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
	case tea.KeyTab:
		s.switchView()
		return "", nil
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
	source := s.currentViewItems()
	for _, item := range source {
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

	// Render view tabs if this selector has views
	if s.hasViews {
		activeTab := lipgloss.NewStyle().Bold(true).Underline(true)
		inactiveTab := lipgloss.NewStyle().Faint(true)

		commonLabel := "Common"
		allLabel := "All"
		if s.viewMode == viewCommon {
			commonLabel = activeTab.Render(commonLabel)
			allLabel = inactiveTab.Render(allLabel)
		} else {
			commonLabel = inactiveTab.Render(commonLabel)
			allLabel = activeTab.Render(allLabel)
		}
		fmt.Fprintf(&b, "[ %s | %s ]  (Tab to switch)\n\n", commonLabel, allLabel)
	}

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

var commonRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"eu-west-1", "eu-central-1",
	"ap-southeast-1", "ap-northeast-1",
}

func serviceNames(svcs map[string]awsx.Service) []string {
	names := make([]string, 0, len(svcs))
	for name := range svcs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
