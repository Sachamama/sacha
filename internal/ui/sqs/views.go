package sqs

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/sqs"
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

	fifoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("156"))

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

	if m.viewingMessages {
		return m.renderMessageList(b)
	}

	// Header
	fmt.Fprintln(b, titleStyle.Render("SQS Queues"))

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
	m.renderQueueList(b, visibleHeight)

	// Status footer
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Total: %d queues", len(m.filteredQueues()))))

	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}

	return b.String()
}

func (m Model) renderQueueList(b *strings.Builder, visibleHeight int) {
	items := m.filteredQueues()
	if len(items) == 0 {
		fmt.Fprintln(b, dimText.Render("No queues found"))
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
		line := fmt.Sprintf("  %s", item.Name)

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

func (m Model) renderMessageList(b *strings.Builder) string {
	fmt.Fprintln(b, titleStyle.Render("Messages"))

	if len(m.messages) == 0 {
		fmt.Fprintln(b, dimText.Render("No messages"))
		return b.String()
	}

	visibleHeight := m.listHeight()

	// Show scroll indicator if needed
	if m.messageOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.messageOffset+visibleHeight, len(m.messages))

	for i := m.messageOffset; i < endIdx; i++ {
		msg := m.messages[i]
		// Show a truncated preview of the body
		preview := truncate(msg.Body, 50)
		line := fmt.Sprintf("  %s  %s", dimText.Render(msg.ID[:minInt(8, len(msg.ID))]), preview)

		if i == m.messageCursor {
			line = cursorStyle.Render(line)
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(m.messages) {
		fmt.Fprintln(b, dimText.Render("  ↓ more below"))
	}

	// Status
	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("%d messages", len(m.messages))))

	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}

	return b.String()
}

func (m Model) renderRight() string {
	b := &strings.Builder{}

	if m.viewingMessages {
		return m.renderMessageDetails(b)
	}

	fmt.Fprintln(b, titleStyle.Render("Queue Details"))

	items := m.filteredQueues()
	if len(items) == 0 || m.cursor >= len(items) {
		fmt.Fprintln(b, dimText.Render("No queue selected"))
		return b.String()
	}

	item := items[m.cursor]

	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Name:"), item.Name)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("URL:"), dimText.Render(item.URL))

	if m.attrs != nil {
		fmt.Fprintln(b)
		// Message counts
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Messages:"), countStyle.Render(fmt.Sprintf("%d", m.attrs.ApproximateMessages)))
		fmt.Fprintf(b, "%s  %d\n", labelStyle.Render("In-Flight:"), m.attrs.ApproximateNotVisible)
		fmt.Fprintf(b, "%s  %d\n", labelStyle.Render("Delayed:"), m.attrs.ApproximateDelayed)

		fmt.Fprintln(b)
		// Queue type
		if m.attrs.IsFIFO {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Type:"), fifoStyle.Render("FIFO"))
		} else {
			fmt.Fprintf(b, "%s  Standard\n", labelStyle.Render("Type:"))
		}

		fmt.Fprintf(b, "%s  %ss\n", labelStyle.Render("Visibility:"), m.attrs.VisibilityTimeout)
		fmt.Fprintf(b, "%s  %s bytes\n", labelStyle.Render("Max Size:"), m.attrs.MaxMessageSize)
		fmt.Fprintf(b, "%s  %ss\n", labelStyle.Render("Retention:"), m.attrs.MessageRetentionPeriod)
		fmt.Fprintf(b, "%s  %ss\n", labelStyle.Render("Delay:"), m.attrs.DelaySeconds)

		if m.attrs.RedrivePolicy != "" {
			fmt.Fprintln(b)
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Redrive:"), m.attrs.RedrivePolicy)
		}

		if m.attrs.ARN != "" {
			fmt.Fprintln(b)
			fmt.Fprintf(b, "%s\n", labelStyle.Render("ARN:"))
			fmt.Fprintf(b, "%s\n", dimText.Render(m.attrs.ARN))
		}
	} else {
		fmt.Fprintln(b, dimText.Render("Loading attributes..."))
	}

	return b.String()
}

func (m Model) renderMessageDetails(b *strings.Builder) string {
	fmt.Fprintln(b, titleStyle.Render("Message Details"))

	if len(m.messages) == 0 || m.messageCursor >= len(m.messages) {
		fmt.Fprintln(b, dimText.Render("No message selected"))
		return b.String()
	}

	msg := m.messages[m.messageCursor]

	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("ID:"), msg.ID)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("MD5:"), dimText.Render(msg.MD5))

	if !msg.SentTimestamp.IsZero() {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Sent:"), msg.SentTimestamp.Format("2006-01-02 15:04:05"))
	}

	if count, ok := msg.Attributes["ApproximateReceiveCount"]; ok {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Receives:"), count)
	}

	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s\n", labelStyle.Render("Body:"))
	// Try to pretty-print JSON
	body := msg.Body
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(body), &jsonObj); err == nil {
		if pretty, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
			body = string(pretty)
		}
	}
	fmt.Fprintln(b, body)

	return b.String()
}

func (m Model) renderExpandedQueueContent() string {
	b := &strings.Builder{}

	items := m.filteredQueues()
	if m.expandedQueue >= len(items) {
		return ""
	}

	item := items[m.expandedQueue]

	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Name:"), item.Name)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("URL:"), item.URL)

	if m.attrs != nil {
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("ARN:"), m.attrs.ARN)
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s  %d\n", labelStyle.Render("Messages:"), m.attrs.ApproximateMessages)
		fmt.Fprintf(b, "%s  %d\n", labelStyle.Render("In-Flight:"), m.attrs.ApproximateNotVisible)
		fmt.Fprintf(b, "%s  %d\n", labelStyle.Render("Delayed:"), m.attrs.ApproximateDelayed)
		fmt.Fprintln(b)
		if m.attrs.IsFIFO {
			fmt.Fprintf(b, "%s  FIFO\n", labelStyle.Render("Type:"))
			fmt.Fprintf(b, "%s  %v\n", labelStyle.Render("Content Dedup:"), m.attrs.ContentDeduplication)
		} else {
			fmt.Fprintf(b, "%s  Standard\n", labelStyle.Render("Type:"))
		}
		fmt.Fprintf(b, "%s  %ss\n", labelStyle.Render("Visibility Timeout:"), m.attrs.VisibilityTimeout)
		fmt.Fprintf(b, "%s  %s bytes\n", labelStyle.Render("Max Message Size:"), m.attrs.MaxMessageSize)
		fmt.Fprintf(b, "%s  %ss\n", labelStyle.Render("Retention Period:"), m.attrs.MessageRetentionPeriod)
		fmt.Fprintf(b, "%s  %ss\n", labelStyle.Render("Delay Seconds:"), m.attrs.DelaySeconds)
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Encryption:"), m.attrs.EncryptionType)

		if !m.attrs.CreatedTimestamp.IsZero() {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Created:"), m.attrs.CreatedTimestamp.Format("2006-01-02 15:04:05"))
		}
		if !m.attrs.LastModifiedTimestamp.IsZero() {
			fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Modified:"), m.attrs.LastModifiedTimestamp.Format("2006-01-02 15:04:05"))
		}

		if m.attrs.RedrivePolicy != "" {
			fmt.Fprintln(b)
			fmt.Fprintf(b, "%s\n", labelStyle.Render("Redrive Policy:"))
			// Pretty-print JSON redrive policy
			var jsonObj interface{}
			if err := json.Unmarshal([]byte(m.attrs.RedrivePolicy), &jsonObj); err == nil {
				if pretty, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
					fmt.Fprintln(b, string(pretty))
				} else {
					fmt.Fprintln(b, m.attrs.RedrivePolicy)
				}
			} else {
				fmt.Fprintln(b, m.attrs.RedrivePolicy)
			}
		}
	} else {
		fmt.Fprintln(b)
		fmt.Fprintln(b, dimText.Render("Loading attributes..."))
	}

	return b.String()
}

func (m Model) renderExpandedMessageContent(msg sqs.Message) string {
	b := &strings.Builder{}

	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Message ID:"), msg.ID)
	fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("MD5:"), msg.MD5)

	if !msg.SentTimestamp.IsZero() {
		fmt.Fprintf(b, "%s  %s\n", labelStyle.Render("Sent:"), msg.SentTimestamp.Format("2006-01-02 15:04:05"))
	}

	// Show all attributes
	if len(msg.Attributes) > 0 {
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%s\n", labelStyle.Render("Attributes:"))
		for k, v := range msg.Attributes {
			fmt.Fprintf(b, "  %s: %s\n", k, v)
		}
	}

	fmt.Fprintln(b)
	fmt.Fprintf(b, "%s\n", labelStyle.Render("Body:"))
	// Try to pretty-print JSON
	body := msg.Body
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(body), &jsonObj); err == nil {
		if pretty, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
			body = string(pretty)
		}
	}
	fmt.Fprintln(b, body)

	return b.String()
}

func (m Model) renderExpandedPopup(background, title string) string {
	if title == "" {
		return background
	}

	header := titleStyle.Render(title)
	body := m.expandedView.View()
	footer := dimText.Render("↑↓/j/k scroll • pgup/pgdn page • esc close")

	content := header + "\n" + body + "\n" + footer

	popup := popupBorderStyle.
		Width(m.expandedWidth()).
		Height(m.expandedHeight() + 4).
		Render(content)

	return lipgloss.Place(
		m.width, m.height-4,
		lipgloss.Center, lipgloss.Center,
		popup,
	)
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
