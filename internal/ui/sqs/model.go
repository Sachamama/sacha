package sqs

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
func NewModel(client *sqs.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	return Model{
		client:          client,
		search:          ti,
		loading:         true,
		expandedQueue:   -1,
		expandedMessage: -1,
	}
}

// Init initializes the model by loading queues.
func (m Model) Init() tea.Cmd {
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
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d queues (more available)", len(msg.queues))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d queues", len(msg.queues))
		}
		return m, m.fetchAttributesCmd()

	case moreQueuesLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.queues = append(m.queues, msg.queues...)
		m.nextToken = msg.nextToken
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d queues (more available)", len(m.queues))
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
		case "pgdn":
			m.expandedView.PageDown()
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
		case "pgdn":
			m.expandedView.PageDown()
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

	// Split width for two panels
	leftWidth := m.width / 2
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

// loadMoreIfNeeded loads the next page if a filter is active and the filtered
// results don't fill the visible list height.
func (m *Model) loadMoreIfNeeded() tea.Cmd {
	if m.search.Value() == "" {
		return nil
	}
	if m.nextToken == nil || m.loadingMore {
		return nil
	}
	if len(m.filteredQueues()) >= m.listHeight() {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreCmd()
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
		return "↑↓ scroll, esc close"
	}
	if m.viewingMessages {
		return "↑↓ move, enter expand, y copy body, esc back"
	}
	return "↑↓ move, / search, enter peek, space expand, y copy URL"
}
