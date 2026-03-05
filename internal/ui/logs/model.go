package logs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/logs"
)

const (
	defaultTailWindow   = 15 * time.Minute
	defaultPollInterval = 5 * time.Second
)

type logGroupsLoadedMsg struct {
	groups    []logs.LogGroup
	nextToken *string
	err       error
}

type moreLogGroupsLoadedMsg struct {
	groups    []logs.LogGroup
	nextToken *string
	err       error
}

type tailUpdateMsg struct {
	events    []logs.TailEvent
	nextStart time.Time
	err       error
}

type pollTailMsg struct{}

type logGroupCreatedMsg struct {
	name string
	err  error
}

type logGroupsDeletedMsg struct {
	names []string
	err   error
}

type retentionSetMsg struct {
	names []string
	days  int32
	err   error
}

// retentionOption represents a selectable retention period.
type retentionOption struct {
	label string
	days  int32 // 0 means never expire
}

var retentionOptions = []retentionOption{
	{"1 day", 1},
	{"3 days", 3},
	{"5 days", 5},
	{"7 days", 7},
	{"14 days", 14},
	{"30 days", 30},
	{"60 days", 60},
	{"90 days", 90},
	{"120 days", 120},
	{"180 days", 180},
	{"365 days", 365},
	{"Never expire", 0},
}

type panel int

const (
	panelGroups panel = iota
	panelTail
)

type Model struct {
	client *logs.Client

	width  int
	height int

	logGroups      []logs.LogGroup
	nextGroupToken *string
	cursor         int
	listOffset     int
	selected       map[string]bool
	loading        bool
	loadingMore    bool

	// Cache
	cache    *cache.Cache
	cacheKey cache.Key

	searching  bool
	search     textinput.Model
	statusLine string

	creating    bool
	createInput textinput.Model

	deleting         bool // showing delete confirmation
	deleteTargets    []string
	settingRetention bool // showing retention picker
	retentionCursor  int

	tailing      bool
	tailStart    time.Time
	pollInterval time.Duration
	events       []logs.TailEvent
	view         viewport.Model

	fullscreen    bool           // fullscreen tail mode
	focus         panel          // which panel has focus
	eventCursor   int            // cursor position in tail events
	expandedEvent int            // index of expanded event, -1 if none
	scrollX       int            // horizontal scroll offset for fullscreen
	expandedView  viewport.Model // viewport for expanded event
	autoScroll    bool           // auto-scroll to bottom on new events

	// Highlight/filter: jq-style field paths like ".level", ".message"
	highlightFields []string        // fields to highlight in log output
	highlightInput  textinput.Model // text input for entering highlight expression
	enteringHL      bool            // whether the highlight input is active
	filterByHL      bool            // when true, only show events matching highlight fields

	// Cross-account monitoring
	monitoring       awsx.MonitoringInfo
	accountOptions   []string // "All Accounts" + linked account IDs
	selectedAccount  string   // "" means all accounts, otherwise an account ID
	selectingAccount bool     // whether the account selector is active
	accountCursor    int
}

func NewModel(client *logs.Client, c *cache.Cache, cacheKey cache.Key, monitoring ...awsx.MonitoringInfo) Model {
	ti := textinput.New()
	ti.Placeholder = "filter log groups"
	ti.Prompt = "/ "

	ci := textinput.New()
	ci.Placeholder = "log group name"
	ci.Prompt = "Name: "

	hi := textinput.New()
	hi.Placeholder = ".level .message"
	hi.Prompt = "highlight: "

	var mon awsx.MonitoringInfo
	if len(monitoring) > 0 {
		mon = monitoring[0]
	}

	var accountOpts []string
	if mon.IsMonitoring {
		accountOpts = append(accountOpts, "All Accounts")
		for _, la := range mon.LinkedAccounts {
			accountOpts = append(accountOpts, la.AccountID)
		}
	}

	m := Model{
		client:         client,
		selected:       map[string]bool{},
		loading:        true,
		cache:          c,
		cacheKey:       cacheKey,
		search:         ti,
		createInput:    ci,
		highlightInput: hi,
		pollInterval:   defaultPollInterval,
		expandedEvent:  -1, // no event expanded
		monitoring:     mon,
		accountOptions: accountOpts,
	}
	if c != nil {
		if items, ok := c.Get(cacheKey); ok {
			if groups, ok := items.([]logs.LogGroup); ok && len(groups) > 0 {
				m.logGroups = groups
				m.loading = false
				m.statusLine = fmt.Sprintf("%d log groups (cached)", len(groups))
			}
		}
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if len(m.logGroups) > 0 {
		return nil
	}
	return m.loadLogGroupsCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.setViewportSize(m.bodyHeight())
		m.ensureCursorVisible()
	case logGroupsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.logGroups = msg.groups
		m.nextGroupToken = msg.nextToken
		sortLogGroups(m.logGroups)
		m.updateCache()
		if m.nextGroupToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d log groups (loading more...)", len(msg.groups))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d log groups", len(msg.groups))
		}
		return m, m.loadMoreIfNeeded()
	case moreLogGroupsLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.logGroups = append(m.logGroups, msg.groups...)
		m.nextGroupToken = msg.nextToken
		sortLogGroups(m.logGroups)
		m.updateCache()
		if m.nextGroupToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d log groups (loading more...)", len(m.logGroups))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d log groups", len(m.logGroups))
		}
		return m, m.loadMoreIfNeeded()
	case logGroupCreatedMsg:
		m.creating = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.statusLine = fmt.Sprintf("Created log group: %s", msg.name)
		m.loading = true
		return m, m.loadLogGroupsCmd()
	case logGroupsDeletedMsg:
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		// Remove deleted groups from local state
		for _, name := range msg.names {
			delete(m.selected, name)
		}
		m.statusLine = fmt.Sprintf("Deleted %d log group(s)", len(msg.names))
		m.loading = true
		return m, m.loadLogGroupsCmd()
	case retentionSetMsg:
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		label := fmt.Sprintf("%d days", msg.days)
		if msg.days == 0 {
			label = "never expire"
		}
		m.statusLine = fmt.Sprintf("Set retention to %s on %d group(s)", label, len(msg.names))
		m.loading = true
		return m, m.loadLogGroupsCmd()

	case tea.KeyMsg:
		if m.creating {
			switch msg.Type {
			case tea.KeyEnter:
				name := m.createInput.Value()
				if name != "" {
					m.statusLine = "Creating log group..."
					return m, m.createLogGroupCmd(name)
				}
				m.creating = false
				return m, nil
			case tea.KeyEscape:
				m.creating = false
				return m, nil
			}
			var cmd tea.Cmd
			m.createInput, cmd = m.createInput.Update(msg)
			return m, cmd
		}

		if m.deleting {
			switch msg.String() {
			case "y", "Y":
				m.deleting = false
				targets := m.deleteTargets
				m.deleteTargets = nil
				m.statusLine = fmt.Sprintf("Deleting %d log group(s)...", len(targets))
				return m, m.deleteLogGroupsCmd(targets)
			case "n", "N", "esc":
				m.deleting = false
				m.deleteTargets = nil
				m.statusLine = ""
			}
			return m, nil
		}

		if m.settingRetention {
			switch msg.String() {
			case "up", "k":
				if m.retentionCursor > 0 {
					m.retentionCursor--
				}
			case "down", "j":
				if m.retentionCursor < len(retentionOptions)-1 {
					m.retentionCursor++
				}
			case "enter":
				m.settingRetention = false
				opt := retentionOptions[m.retentionCursor]
				targets := m.selectedGroups()
				label := opt.label
				m.statusLine = fmt.Sprintf("Setting retention to %s on %d group(s)...", label, len(targets))
				return m, m.setRetentionCmd(targets, opt.days)
			case "esc", "q":
				m.settingRetention = false
				m.statusLine = ""
			}
			return m, nil
		}

		if m.selectingAccount {
			switch msg.String() {
			case "up", "k":
				if m.accountCursor > 0 {
					m.accountCursor--
				}
			case "down", "j":
				if m.accountCursor < len(m.accountOptions)-1 {
					m.accountCursor++
				}
			case "enter":
				m.selectingAccount = false
				if m.accountCursor == 0 {
					m.selectedAccount = "" // "All Accounts"
				} else {
					m.selectedAccount = m.accountOptions[m.accountCursor]
				}
				// Reload log groups with the new account filter
				if m.cache != nil {
					m.cache.Delete(m.cacheKey)
				}
				m.logGroups = nil
				m.nextGroupToken = nil
				m.cursor = 0
				m.listOffset = 0
				m.loading = true
				m.statusLine = "Loading log groups..."
				return m, m.loadLogGroupsCmd()
			case "esc", "q":
				m.selectingAccount = false
			}
			return m, nil
		}

		if m.searching {
			switch msg.Type {
			case tea.KeyEnter, tea.KeyEscape:
				m.searching = false
				return m, m.loadMoreIfNeeded()
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			// Reset cursor and offset when filter changes
			m.cursor = 0
			m.listOffset = 0
			return m, cmd
		}

		// Handle highlight input
		if m.enteringHL {
			switch msg.Type {
			case tea.KeyEnter:
				m.enteringHL = false
				val := strings.TrimSpace(m.highlightInput.Value())
				if val == "" {
					m.highlightFields = nil
					m.filterByHL = false
				} else {
					m.highlightFields = strings.Fields(val)
				}
				m.eventCursor = 0
				m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
				m.ensureEventCursorVisible()
				return m, nil
			case tea.KeyEscape:
				m.enteringHL = false
				return m, nil
			}
			var cmd tea.Cmd
			m.highlightInput, cmd = m.highlightInput.Update(msg)
			return m, cmd
		}

		// Handle expanded event popup
		if m.expandedEvent >= 0 {
			switch msg.String() {
			case "esc", "q", "enter":
				m.expandedEvent = -1
			case "up", "k":
				m.expandedView.ScrollUp(1)
			case "down", "j", " ":
				m.expandedView.ScrollDown(1)
			case "pgup":
				m.expandedView.HalfPageUp()
			case "pgdn":
				m.expandedView.HalfPageDown()
			}
			return m, nil
		}

		switch msg.String() {
		case "tab":
			if m.tailing && !m.fullscreen {
				if m.focus == panelTail {
					m.focus = panelGroups
				} else {
					m.focus = panelTail
				}
				m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
			}
		case "left", "h":
			if m.tailing && m.fullscreen {
				// Horizontal scroll left in fullscreen
				if m.scrollX > 0 {
					m.scrollX -= 10
					if m.scrollX < 0 {
						m.scrollX = 0
					}
					m.view.SetContent(m.renderEventsContent(true))
				}
			} else if m.tailing && !m.fullscreen {
				if m.focus == panelTail {
					m.focus = panelGroups
				} else {
					m.focus = panelTail
				}
				m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
			}
		case "right", "l":
			if m.tailing && m.fullscreen {
				// Horizontal scroll right in fullscreen
				m.scrollX += 10
				m.view.SetContent(m.renderEventsContent(true))
			} else if m.tailing && !m.fullscreen {
				if m.focus == panelGroups {
					m.focus = panelTail
				} else {
					m.focus = panelGroups
				}
				m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
			}
		case "up", "k":
			if m.tailing && m.focus == panelTail {
				if m.eventCursor > 0 {
					m.eventCursor--
					m.autoScroll = false
					m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
					m.ensureEventCursorVisible()
				}
			} else {
				if m.cursor > 0 {
					m.cursor--
					m.ensureCursorVisible()
				}
			}
		case "down", "j":
			if m.tailing && m.focus == panelTail {
				if m.eventCursor < len(m.filteredEvents())-1 {
					m.eventCursor++
					m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
					m.ensureEventCursorVisible()
				}
				if m.eventCursor >= len(m.filteredEvents())-1 {
					m.autoScroll = true
				}
			} else {
				if m.cursor < len(m.filteredGroups())-1 {
					m.cursor++
					m.ensureCursorVisible()
				}
				// Lazy load more log groups when near the end
				if !m.tailing && m.nextGroupToken != nil && !m.loadingMore {
					if m.cursor >= len(m.filteredGroups())-5 {
						m.loadingMore = true
						return m, m.loadMoreLogGroupsCmd()
					}
				}
			}
		case "/":
			if !m.tailing || m.focus == panelGroups {
				m.searching = true
				return m, m.search.Focus()
			}
		case " ":
			if m.tailing && m.focus == panelTail && len(m.filteredEvents()) > 0 {
				evts := m.filteredEvents()
				m.expandedEvent = m.eventCursor
				m.expandedView = initExpandedView(evts[m.eventCursor], m.width, m.height)
			} else {
				m.toggleSelection()
				// Refresh logs when selection changes while tailing
				if m.tailing && len(m.selectedGroups()) > 0 {
					m.events = nil
					m.eventCursor = 0
					m.autoScroll = true
					m.tailStart = time.Now().Add(-defaultTailWindow)
					m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
					return m, m.pollTailCmd()
				}
			}
		case "enter":
			if m.tailing && m.focus == panelTail && len(m.filteredEvents()) > 0 {
				evts := m.filteredEvents()
				m.expandedEvent = m.eventCursor
				m.expandedView = initExpandedView(evts[m.eventCursor], m.width, m.height)
			}
		case "a":
			if !m.tailing || m.focus == panelGroups {
				m.toggleAll()
				// Refresh logs when selection changes while tailing
				if m.tailing && len(m.selectedGroups()) > 0 {
					m.events = nil
					m.eventCursor = 0
					m.autoScroll = true
					m.tailStart = time.Now().Add(-defaultTailWindow)
					m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
					return m, m.pollTailCmd()
				}
			}
		case "c":
			if !m.tailing || m.focus == panelGroups {
				m.creating = true
				m.createInput.SetValue("")
				return m, m.createInput.Focus()
			}
		case "d", "D":
			if !m.tailing {
				targets := m.selectedGroups()
				if len(targets) > 0 {
					m.deleting = true
					m.deleteTargets = targets
				}
			}
		case "R":
			if !m.tailing {
				targets := m.selectedGroups()
				if len(targets) > 0 {
					m.settingRetention = true
					m.retentionCursor = 0
				}
			}
		case "m":
			// Open account selector (only in monitoring mode, not while tailing)
			if m.monitoring.IsMonitoring && !m.tailing && len(m.accountOptions) > 0 {
				m.selectingAccount = true
				m.accountCursor = 0
				// Pre-select current account
				for i, opt := range m.accountOptions {
					if (m.selectedAccount == "" && i == 0) || opt == m.selectedAccount {
						m.accountCursor = i
						break
					}
				}
			}
		case "t":
			if !m.tailing && len(m.selectedGroups()) > 0 {
				m.tailing = true
				m.fullscreen = false
				m.focus = panelTail
				m.events = nil
				m.eventCursor = 0
				m.autoScroll = true
				m.tailStart = time.Now().Add(-defaultTailWindow)
				m.view = viewport.Model{}
				m.setViewportSize(m.bodyHeight())
				return m, m.pollTailCmd()
			}
		case "f":
			if m.tailing {
				m.fullscreen = !m.fullscreen
				if m.fullscreen {
					m.focus = panelTail
				} else {
					m.scrollX = 0 // Reset horizontal scroll when exiting fullscreen
				}
				m.setViewportSize(m.bodyHeight())
				m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
				m.ensureEventCursorVisible()
			}
		case "q", "esc":
			if m.tailing {
				m.tailing = false
				m.fullscreen = false
				m.focus = panelGroups
				m.scrollX = 0
				m.autoScroll = false
				m.highlightFields = nil
				m.filterByHL = false
			}
		case "H":
			// Open highlight input (jq-style field paths)
			if m.tailing && m.focus == panelTail {
				m.enteringHL = true
				// Pre-fill with current highlight expression
				m.highlightInput.SetValue(strings.Join(m.highlightFields, " "))
				return m, m.highlightInput.Focus()
			}
		case "F":
			// Toggle filter-by-highlight mode
			if m.tailing && m.focus == panelTail && len(m.highlightFields) > 0 {
				m.filterByHL = !m.filterByHL
				m.eventCursor = 0
				m.view.SetContent(m.renderEventsContent(true))
				m.ensureEventCursorVisible()
			}
		case "x":
			// Stop watching and reset Log panel
			if m.tailing {
				m.tailing = false
				m.fullscreen = false
				m.focus = panelGroups
				m.events = nil
				m.eventCursor = 0
				m.scrollX = 0
				m.autoScroll = false
				m.highlightFields = nil
				m.filterByHL = false
			}
		case "y":
			text := m.getCopyText()
			if text != "" {
				_ = clipboard.WriteAll(text)
				m.statusLine = "Copied: " + text
			}
		case "ctrl+r":
			if !m.tailing {
				if m.cache != nil {
					m.cache.Delete(m.cacheKey)
				}
				m.logGroups = nil
				m.nextGroupToken = nil
				m.cursor = 0
				m.listOffset = 0
				m.loading = true
				m.statusLine = "Refreshing..."
				return m, m.loadLogGroupsCmd()
			}
		}
	case pollTailMsg:
		if !m.tailing {
			return m, nil
		}
		return m, m.pollTailCmd()
	case tailUpdateMsg:
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		if len(msg.events) > 0 {
			m.tailStart = msg.nextStart
			m.events = append(m.events, msg.events...)
			if len(m.events) > 1000 {
				trimmed := len(m.events) - 1000
				m.events = m.events[trimmed:]
				if !m.autoScroll {
					m.eventCursor -= trimmed
					if m.eventCursor < 0 {
						m.eventCursor = 0
					}
				}
			}
			// Auto-scroll to latest events when autoScroll is enabled
			evts := m.filteredEvents()
			if m.autoScroll {
				m.eventCursor = len(evts) - 1
			} else if m.eventCursor >= len(evts) {
				m.eventCursor = len(evts) - 1
			}
			if m.eventCursor < 0 {
				m.eventCursor = 0
			}
			m.view.SetContent(m.renderEventsContent(m.focus == panelTail))
			m.ensureEventCursorVisible()
		}
		if m.tailing {
			return m, tea.Tick(m.pollInterval, func(time.Time) tea.Msg { return pollTailMsg{} })
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	bodyHeight := m.bodyHeight()

	if m.tailing {
		m.setViewportSize(bodyHeight)
	}

	var view string

	if m.tailing && m.fullscreen {
		// Fullscreen tail mode
		view = panelStyle.Width(m.width - 2).Height(bodyHeight).Render(m.renderTail())
	} else {
		// Split width for two panels (40/60)
		leftWidth := m.width * 2 / 5
		rightWidth := m.width - leftWidth

		// Highlight focused panel
		leftStyle := panelStyle
		rightStyle := panelStyle
		if m.tailing {
			if m.focus == panelGroups {
				leftStyle = leftStyle.BorderForeground(lipgloss.Color("212"))
			} else {
				rightStyle = rightStyle.BorderForeground(lipgloss.Color("212"))
			}
		}

		left := leftStyle.Width(leftWidth - 2).Height(bodyHeight).Render(m.renderGroups())
		right := rightStyle.Width(rightWidth - 2).Height(bodyHeight).Render(m.renderTail())

		view = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	if m.creating {
		popup := m.renderCreatePopup()
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	if m.deleting {
		popup := m.renderDeleteConfirm()
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	if m.settingRetention {
		popup := m.renderRetentionPicker()
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	if m.enteringHL {
		popup := m.renderHighlightPopup()
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	if m.selectingAccount {
		popup := m.renderAccountPicker()
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	if evts := m.filteredEvents(); m.expandedEvent >= 0 && m.expandedEvent < len(evts) {
		popup := m.renderExpandedEvent(evts[m.expandedEvent])
		view = lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
	}

	return view
}

func (m Model) listOpts() []logs.ListLogGroupsOptions {
	if !m.monitoring.IsMonitoring {
		return nil
	}
	opts := logs.ListLogGroupsOptions{IncludeLinkedAccounts: true}
	if m.selectedAccount != "" {
		opts.AccountIdentifiers = []string{m.selectedAccount}
	}
	return []logs.ListLogGroupsOptions{opts}
}

func (m Model) loadLogGroupsCmd() tea.Cmd {
	opts := m.listOpts()
	return func() tea.Msg {
		ctx := context.Background()
		groups, nextToken, err := m.client.ListLogGroups(ctx, nil, opts...)
		if err != nil {
			return logGroupsLoadedMsg{err: err}
		}
		return logGroupsLoadedMsg{groups: groups, nextToken: nextToken}
	}
}

func (m Model) loadMoreLogGroupsCmd() tea.Cmd {
	token := m.nextGroupToken
	opts := m.listOpts()
	return func() tea.Msg {
		ctx := context.Background()
		groups, nextToken, err := m.client.ListLogGroups(ctx, token, opts...)
		return moreLogGroupsLoadedMsg{groups: groups, nextToken: nextToken, err: err}
	}
}

func (m *Model) loadMoreIfNeeded() tea.Cmd {
	if m.tailing {
		return nil
	}
	if m.nextGroupToken == nil || m.loadingMore {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreLogGroupsCmd()
}

// updateCache stores the current log groups in the cache.
func (m *Model) updateCache() {
	if m.cache != nil {
		m.cache.Set(m.cacheKey, m.logGroups)
	}
}

// sortLogGroups sorts log groups by Name (case-insensitive).
func sortLogGroups(groups []logs.LogGroup) {
	sort.Slice(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
}

func (m Model) pollTailCmd() tea.Cmd {
	groups := m.selectedGroups()
	start := m.tailStart
	return func() tea.Msg {
		ctx := context.Background()
		events, next, err := m.client.FetchEvents(ctx, groups, start)
		return tailUpdateMsg{events: events, nextStart: next, err: err}
	}
}

func (m Model) createLogGroupCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.client.CreateLogGroup(ctx, name)
		return logGroupCreatedMsg{name: name, err: err}
	}
}

func (m Model) deleteLogGroupsCmd(names []string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, name := range names {
			if err := m.client.DeleteLogGroup(ctx, name); err != nil {
				return logGroupsDeletedMsg{err: err}
			}
		}
		return logGroupsDeletedMsg{names: names}
	}
}

func (m Model) setRetentionCmd(names []string, days int32) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, name := range names {
			if err := m.client.SetRetentionPolicy(ctx, name, days); err != nil {
				return retentionSetMsg{err: err}
			}
		}
		return retentionSetMsg{names: names, days: days}
	}
}

func (m Model) filteredGroups() []logs.LogGroup {
	if !m.searching && m.search.Value() == "" {
		return m.logGroups
	}
	q := strings.ToLower(m.search.Value())
	out := make([]logs.LogGroup, 0, len(m.logGroups))
	for _, g := range m.logGroups {
		if strings.Contains(strings.ToLower(g.Name), q) {
			out = append(out, g)
		}
	}
	return out
}

func (m *Model) toggleSelection() {
	groups := m.filteredGroups()
	if len(groups) == 0 || m.cursor >= len(groups) {
		return
	}
	name := groups[m.cursor].Name
	if m.selected[name] {
		delete(m.selected, name)
	} else {
		m.selected[name] = true
	}
}

func (m *Model) toggleAll() {
	if len(m.selected) == len(m.logGroups) {
		m.selected = map[string]bool{}
		return
	}
	for _, g := range m.logGroups {
		m.selected[g.Name] = true
	}
}

func (m Model) selectedGroups() []string {
	out := make([]string, 0, len(m.selected))
	for name, ok := range m.selected {
		if ok {
			out = append(out, name)
		}
	}
	return out
}

func (m Model) selectedCount() int {
	count := 0
	for _, ok := range m.selected {
		if ok {
			count++
		}
	}
	return count
}

func (m Model) getCopyText() string {
	// When tailing and focused on tail panel, copy the selected event message
	if m.tailing && m.focus == panelTail && len(m.events) > 0 && m.eventCursor < len(m.events) {
		return m.events[m.eventCursor].Message
	}
	// Otherwise copy the log group name under cursor
	groups := m.filteredGroups()
	if len(groups) == 0 || m.cursor >= len(groups) {
		return ""
	}
	return groups[m.cursor].Name
}

// Tailing reports whether the model is actively tailing logs.
func (m Model) Tailing() bool {
	return m.tailing
}

// Searching reports whether the model has an active text input or overlay.
func (m Model) Searching() bool {
	return m.searching || m.creating || m.deleting || m.settingRetention || m.enteringHL || m.selectingAccount
}

// SelectedAccount returns the selected account ID for cross-account filtering.
// Returns empty string when showing all accounts.
func (m Model) SelectedAccount() string {
	return m.selectedAccount
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.expandedEvent >= 0 {
		return "↑↓ scroll, pgup/pgdn page, esc close"
	}
	if m.deleting {
		return "y confirm delete, n/esc cancel"
	}
	if m.settingRetention {
		return "↑↓ move, enter select, esc cancel"
	}
	if m.creating {
		return "enter create, esc cancel"
	}
	if m.searching {
		return "enter/esc close search"
	}
	if m.enteringHL {
		return "enter apply, esc cancel — use jq syntax: .level .message"
	}
	if m.selectingAccount {
		return "↑↓ move, enter select, esc cancel"
	}
	if m.tailing {
		hlInfo := ""
		if len(m.highlightFields) > 0 {
			filterState := "off"
			if m.filterByHL {
				filterState = "on"
			}
			hlInfo = fmt.Sprintf(", H highlight, F filter[%s]", filterState)
		} else {
			hlInfo = ", H highlight"
		}
		if m.fullscreen {
			return fmt.Sprintf("↑↓ move, ←→ scroll, enter expand, y copy%s, f exit fullscreen, x/q stop", hlInfo)
		}
		if m.focus == panelGroups {
			return "↑↓ move, / search, space select, a all, y copy, tab/→ switch, f fullscreen, x/q stop"
		}
		return fmt.Sprintf("↑↓ move, enter expand, y copy%s, tab/← switch, f fullscreen, x/q stop", hlInfo)
	}
	help := "↑↓ move, / search, space select, a all, c create, d delete, R retention, t tail, y copy, ctrl+r refresh"
	if m.monitoring.IsMonitoring {
		help += ", m account"
	}
	return help
}

func (m *Model) setViewportSize(bodyHeight int) {
	if !m.tailing {
		return
	}
	var innerWidth int
	if m.fullscreen {
		innerWidth = m.width - 4 // account for border/padding
	} else {
		rightWidth := m.width - m.width*2/5
		innerWidth = rightWidth - 4 // account for border/padding
	}
	if innerWidth < 20 {
		innerWidth = m.width - 4
	}
	contentHeight := bodyHeight - 2  // panel borders
	innerHeight := contentHeight - 1 // header inside panel
	if innerHeight < 1 {
		innerHeight = 1
	}
	m.view.Width = innerWidth
	m.view.Height = innerHeight
}

func (m Model) bodyHeight() int {
	h := m.height - 4 // account for header/footer lines in app view
	if h < 4 {
		return m.height
	}
	return h
}

func (m Model) listHeight() int {
	// bodyHeight minus: header(1) + search hint(1) + blank(1) + footer(1) + status(1) + borders(2)
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

	// When scrolled past the top, the "↑ more above" indicator takes one
	// line from the visible area, so reduce the effective height.
	effective := visibleHeight
	if m.listOffset > 0 {
		effective--
	}
	if effective < 1 {
		effective = 1
	}

	if m.cursor >= m.listOffset+effective {
		m.listOffset = m.cursor - effective + 1
	}

	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

// eventContentOffset returns the number of header lines rendered before event
// rows in the viewport content. Plain view starts immediately with event rows.
func (m *Model) eventContentOffset() int {
	return 0
}

// filteredEvents returns events filtered by highlight fields when filterByHL is active.
func (m *Model) filteredEvents() []logs.TailEvent {
	if !m.filterByHL || len(m.highlightFields) == 0 {
		return m.events
	}
	var out []logs.TailEvent
	for _, e := range m.events {
		if eventMatchesHighlight(e, m.highlightFields) {
			out = append(out, e)
		}
	}
	return out
}

// eventMatchesHighlight checks if an event's JSON message contains any of the highlight fields.
func eventMatchesHighlight(e logs.TailEvent, fields []string) bool {
	parsed := parseJSONLog(e.Message)
	if parsed == nil {
		return false
	}
	for _, field := range fields {
		key := strings.TrimPrefix(field, ".")
		if key == "" {
			continue
		}
		if _, ok := parsed[key]; ok {
			return true
		}
	}
	return false
}

// renderEventsContent builds the viewport content for the tail panel.
func (m *Model) renderEventsContent(showCursor bool) string {
	return renderEvents(m.filteredEvents(), m.eventCursor, m.view.Width, showCursor, m.scrollX, m.highlightFields)
}

// ensureEventCursorVisible adjusts the viewport's YOffset so the line
// corresponding to eventCursor is within the visible area.
func (m *Model) ensureEventCursorVisible() {
	evts := m.filteredEvents()
	if len(evts) == 0 || m.view.Height <= 0 {
		return
	}
	cursorLine := m.eventCursor + m.eventContentOffset()
	if cursorLine < m.view.YOffset {
		m.view.SetYOffset(cursorLine)
	} else if cursorLine >= m.view.YOffset+m.view.Height {
		m.view.SetYOffset(cursorLine - m.view.Height + 1)
	}
}
