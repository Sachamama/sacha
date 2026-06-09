package sqs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/sqs"
)

// Model is the Bubble Tea model for the SQS Queue Browser.
type Model struct {
	client *sqs.Client

	// List state
	queues     []sqs.Queue
	cursor     int
	listOffset int

	// Pagination
	nextToken   *string
	loadingMore bool

	// Cache
	cache    *cache.Cache
	cacheKey cache.Key

	// Right pane - queue attributes
	attrs *sqs.QueueAttributes

	// Message peek
	messages        []sqs.Message
	viewingMessages bool
	messageCursor   int
	messageOffset   int

	// Expanded popup
	expandedQueue   int            // index of expanded queue, -1 if none
	expandedView    viewport.Model // viewport for scrolling expanded content
	expandedMessage int            // index of expanded message, -1 if none

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string
}

// NewModel creates a new SQS browser model.
func NewModel(client *sqs.Client, c *cache.Cache, cacheKey cache.Key) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	m := Model{
		client:          client,
		search:          ti,
		loading:         true,
		expandedQueue:   -1,
		expandedMessage: -1,
		cache:           c,
		cacheKey:        cacheKey,
	}
	if c != nil {
		if items, ok := c.Get(cacheKey); ok {
			if queues, ok := items.([]sqs.Queue); ok && len(queues) > 0 {
				m.queues = queues
				m.loading = false
				m.statusLine = fmt.Sprintf("%d queues (cached)", len(queues))
			}
		}
	}
	return m
}

// Init initializes the model by loading queues.
func (m Model) Init() tea.Cmd {
	if len(m.queues) > 0 {
		return nil
	}
	return m.loadQueuesCmd()
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case queuesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.queues = msg.queues
		m.nextToken = msg.nextToken
		m.cursor = 0
		m.attrs = nil
		sortQueues(m.queues)
		m.updateCache()
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d queues (loading more...)", len(msg.queues))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d queues", len(msg.queues))
		}
		return m, tea.Batch(m.fetchAttributesCmd(), m.loadMoreIfNeeded())

	case moreQueuesLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.queues = append(m.queues, msg.queues...)
		m.nextToken = msg.nextToken
		sortQueues(m.queues)
		m.updateCache()
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d queues (loading more...)", len(m.queues))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d queues", len(m.queues))
		}
		return m, m.loadMoreIfNeeded()

	case queueAttributesMsg:
		if msg.err != nil {
			return m, nil
		}
		if m.currentQueueURL() == msg.url {
			m.attrs = msg.attrs
		}
		return m, nil

	case messagesLoadedMsg:
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.messages = msg.messages
		m.viewingMessages = true
		m.messageCursor = 0
		m.messageOffset = 0
		if len(msg.messages) == 0 {
			m.statusLine = "No messages in queue"
		} else {
			m.statusLine = fmt.Sprintf("Peeked %d messages", len(msg.messages))
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input
	if m.searching {
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEscape:
			m.searching = false
			m.clampCursor()
			return m, tea.Batch(m.onCursorMove(), m.loadMoreIfNeeded())
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.clampCursor()
		return m, cmd
	}

	// Handle expanded message popup
	if m.expandedMessage >= 0 {
		switch msg.String() {
		case "up", "k":
			m.expandedView.ScrollUp(1)
			return m, nil
		case "down", "j":
			m.expandedView.ScrollDown(1)
			return m, nil
		case "pgup":
			m.expandedView.PageUp()
			return m, nil
		case "pgdown", "pgdn":
			m.expandedView.PageDown()
			return m, nil
		case "home":
			m.expandedView.GotoTop()
			return m, nil
		case "end":
			m.expandedView.GotoBottom()
			return m, nil
		case "esc", "q":
			m.expandedMessage = -1
			return m, nil
		}
		return m, nil
	}

	// Handle expanded queue popup
	if m.expandedQueue >= 0 {
		switch msg.String() {
		case "up", "k":
			m.expandedView.ScrollUp(1)
			return m, nil
		case "down", "j":
			m.expandedView.ScrollDown(1)
			return m, nil
		case "pgup":
			m.expandedView.PageUp()
			return m, nil
		case "pgdown", "pgdn":
			m.expandedView.PageDown()
			return m, nil
		case "home":
			m.expandedView.GotoTop()
			return m, nil
		case "end":
			m.expandedView.GotoBottom()
			return m, nil
		case "esc", "q":
			m.expandedQueue = -1
			return m, nil
		}
		return m, nil
	}

	// Handle message view navigation
	if m.viewingMessages {
		switch msg.String() {
		case "up", "k":
			if m.messageCursor > 0 {
				m.messageCursor--
				m.ensureMessageCursorVisible()
			}
			return m, nil
		case "down", "j":
			if m.messageCursor < len(m.messages)-1 {
				m.messageCursor++
				m.ensureMessageCursorVisible()
			}
			return m, nil
		case "pgup":
			m.setMessageCursor(m.messageCursor - m.listHeight())
			return m, nil
		case "pgdown", "pgdn":
			m.setMessageCursor(m.messageCursor + m.listHeight())
			return m, nil
		case "home":
			m.setMessageCursor(0)
			return m, nil
		case "end":
			m.setMessageCursor(len(m.messages) - 1)
			return m, nil
		case "enter", " ":
			if len(m.messages) > 0 && m.messageCursor < len(m.messages) {
				m.expandedMessage = m.messageCursor
				content := m.renderExpandedMessageContent(m.messages[m.messageCursor])
				m.expandedView = viewport.New(m.expandedWidth(), m.expandedHeight())
				m.expandedView.SetContent(content)
			}
			return m, nil
		case "esc", "backspace", "h":
			m.viewingMessages = false
			m.messages = nil
			m.messageCursor = 0
			m.messageOffset = 0
			m.statusLine = ""
			return m, nil
		case "y":
			if len(m.messages) > 0 && m.messageCursor < len(m.messages) {
				text := m.messages[m.messageCursor].Body
				_ = clipboard.WriteAll(text)
				m.statusLine = "Copied message body"
			}
			return m, nil
		}
		return m, nil
	}

	// Normal queue list navigation
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
			return m, m.onCursorMove()
		}
	case "down", "j":
		maxIdx := m.maxCursorIndex()
		if m.cursor < maxIdx {
			m.cursor++
			m.ensureCursorVisible()
			cmd := m.onCursorMove()
			// Lazy load more when near the end
			if m.nextToken != nil && !m.loadingMore {
				if m.cursor >= len(m.filteredQueues())-5 {
					m.loadingMore = true
					return m, tea.Batch(cmd, m.loadMoreCmd())
				}
			}
			return m, cmd
		}
	case "pgup":
		return m, m.jumpCursor(m.cursor - m.listHeight())
	case "pgdown", "pgdn":
		return m, m.jumpCursor(m.cursor + m.listHeight())
	case "home":
		return m, m.jumpCursor(0)
	case "end":
		return m, m.jumpCursor(m.maxCursorIndex())
	case "enter":
		return m.handlePeek()
	case " ":
		return m.handleExpand()
	case "/":
		m.searching = true
		m.search.SetValue("")
		return m, m.search.Focus()
	case "y":
		text := m.getCopyText()
		if text != "" {
			_ = clipboard.WriteAll(text)
			m.statusLine = "Copied: " + text
		}
	case "ctrl+r":
		if m.viewingMessages {
			// Refresh messages
			items := m.filteredQueues()
			if len(items) > 0 && m.cursor < len(items) {
				m.messages = nil
				m.messageCursor = 0
				m.messageOffset = 0
				m.statusLine = "Refreshing..."
				return m, m.peekMessagesCmd(items[m.cursor].URL)
			}
		} else {
			if m.cache != nil {
				m.cache.Delete(m.cacheKey)
			}
			m.queues = nil
			m.nextToken = nil
			m.cursor = 0
			m.listOffset = 0
			m.attrs = nil
			m.loading = true
			m.statusLine = "Refreshing..."
			return m, m.loadQueuesCmd()
		}
	}

	return m, nil
}

func (m Model) handlePeek() (tea.Model, tea.Cmd) {
	items := m.filteredQueues()
	if len(items) == 0 || m.cursor >= len(items) {
		return m, nil
	}
	queue := items[m.cursor]
	m.statusLine = "Peeking messages..."
	return m, m.peekMessagesCmd(queue.URL)
}

func (m Model) handleExpand() (tea.Model, tea.Cmd) {
	items := m.filteredQueues()
	if len(items) == 0 || m.cursor >= len(items) {
		return m, nil
	}
	m.expandedQueue = m.cursor
	content := m.renderExpandedQueueContent()
	m.expandedView = viewport.New(m.expandedWidth(), m.expandedHeight())
	m.expandedView.SetContent(content)
	return m, nil
}

// View renders the model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Split width for two panels (40/60)
	leftWidth := m.width * 2 / 5
	rightWidth := m.width - leftWidth
	bodyHeight := m.bodyHeight()

	left := panelStyle.Width(leftWidth - 2).Height(bodyHeight).Render(m.renderLeft())
	right := panelStyle.Width(rightWidth - 2).Height(bodyHeight).Render(m.renderRight())

	view := joinHorizontal(left, right)

	// Overlay expanded popups if active
	if m.expandedMessage >= 0 {
		view = m.renderExpandedPopup(view, "Message: "+m.messages[m.expandedMessage].ID)
	} else if m.expandedQueue >= 0 {
		items := m.filteredQueues()
		if m.expandedQueue < len(items) {
			view = m.renderExpandedPopup(view, "Queue: "+items[m.expandedQueue].Name)
		}
	}

	return view
}

func (m Model) bodyHeight() int {
	h := m.height - 4
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

func (m *Model) ensureMessageCursorVisible() {
	visibleHeight := m.listHeight()
	if visibleHeight <= 0 {
		return
	}

	if m.messageCursor < m.messageOffset {
		m.messageOffset = m.messageCursor
	}

	effective := visibleHeight
	if m.messageOffset > 0 {
		effective--
	}
	if effective < 1 {
		effective = 1
	}

	if m.messageCursor >= m.messageOffset+effective {
		m.messageOffset = m.messageCursor - effective + 1
	}

	if m.messageOffset < 0 {
		m.messageOffset = 0
	}
}

func (m Model) maxCursorIndex() int {
	return len(m.filteredQueues()) - 1
}

func (m *Model) clampCursor() {
	maxIdx := m.maxCursorIndex()
	if m.cursor > maxIdx {
		m.cursor = maxIdx
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

// jumpCursor moves the queue cursor to idx (clamped to the list bounds),
// updates scroll visibility, and returns the command to refresh attributes,
// lazy-loading the next page when the cursor lands near the end. Used by page
// and home/end navigation.
func (m *Model) jumpCursor(idx int) tea.Cmd {
	maxIdx := m.maxCursorIndex()
	if idx > maxIdx {
		idx = maxIdx
	}
	if idx < 0 {
		idx = 0
	}
	if idx == m.cursor {
		return nil
	}
	m.cursor = idx
	m.ensureCursorVisible()
	cmd := m.onCursorMove()
	if m.nextToken != nil && !m.loadingMore && m.cursor >= len(m.filteredQueues())-5 {
		m.loadingMore = true
		return tea.Batch(cmd, m.loadMoreCmd())
	}
	return cmd
}

// setMessageCursor moves the peeked-message cursor to idx (clamped) and updates
// scroll visibility. Used by page and home/end navigation in the message view.
func (m *Model) setMessageCursor(idx int) {
	last := len(m.messages) - 1
	if idx > last {
		idx = last
	}
	if idx < 0 {
		idx = 0
	}
	m.messageCursor = idx
	m.ensureMessageCursorVisible()
}

func (m Model) currentQueueURL() string {
	items := m.filteredQueues()
	if len(items) == 0 || m.cursor >= len(items) {
		return ""
	}
	return items[m.cursor].URL
}

func (m Model) onCursorMove() tea.Cmd {
	m.attrs = nil
	return m.fetchAttributesCmd()
}

func (m Model) getCopyText() string {
	items := m.filteredQueues()
	if len(items) == 0 || m.cursor >= len(items) {
		return ""
	}
	return items[m.cursor].URL
}

func (m Model) filteredQueues() []sqs.Queue {
	if m.search.Value() == "" {
		return m.queues
	}
	q := strings.ToLower(m.search.Value())
	out := make([]sqs.Queue, 0, len(m.queues))
	for _, queue := range m.queues {
		if strings.Contains(strings.ToLower(queue.Name), q) {
			out = append(out, queue)
		}
	}
	return out
}

func (m *Model) loadMoreIfNeeded() tea.Cmd {
	if m.nextToken == nil || m.loadingMore {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreCmd()
}

// updateCache stores the current queues in the cache.
func (m *Model) updateCache() {
	if m.cache != nil {
		m.cache.Set(m.cacheKey, m.queues)
	}
}

// sortQueues sorts queues by Name (case-insensitive).
func sortQueues(queues []sqs.Queue) {
	sort.Slice(queues, func(i, j int) bool {
		return strings.ToLower(queues[i].Name) < strings.ToLower(queues[j].Name)
	})
}

// Commands

func (m Model) loadQueuesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		queues, nextToken, err := m.client.ListQueues(ctx, nil)
		return queuesLoadedMsg{queues: queues, nextToken: nextToken, err: err}
	}
}

func (m Model) loadMoreCmd() tea.Cmd {
	token := m.nextToken
	return func() tea.Msg {
		ctx := context.Background()
		queues, nextToken, err := m.client.ListQueues(ctx, token)
		return moreQueuesLoadedMsg{queues: queues, nextToken: nextToken, err: err}
	}
}

func (m Model) fetchAttributesCmd() tea.Cmd {
	items := m.filteredQueues()
	if len(items) == 0 || m.cursor >= len(items) {
		return nil
	}
	url := items[m.cursor].URL
	return func() tea.Msg {
		ctx := context.Background()
		attrs, err := m.client.GetQueueAttributes(ctx, url)
		return queueAttributesMsg{attrs: attrs, url: url, err: err}
	}
}

func (m Model) peekMessagesCmd(queueURL string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		messages, err := m.client.PeekMessages(ctx, queueURL, 10)
		return messagesLoadedMsg{messages: messages, err: err}
	}
}

func (m Model) expandedWidth() int {
	w := m.width - 8
	if w < 40 {
		return 40
	}
	return w
}

func (m Model) expandedHeight() int {
	h := m.height - 10
	if h < 10 {
		return 10
	}
	return h
}

// Searching reports whether the model has an active text input.
func (m Model) Searching() bool {
	return m.searching
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.searching {
		return "enter/esc close search"
	}
	if m.expandedMessage >= 0 || m.expandedQueue >= 0 {
		return "↑↓ scroll, pgup/pgdn page, home/end top/bottom, esc close"
	}
	if m.viewingMessages {
		return "↑↓ move, enter expand, y copy body, ctrl+r refresh, esc back"
	}
	return "↑↓ move, / search, enter peek, space expand, y copy URL, ctrl+r refresh"
}
