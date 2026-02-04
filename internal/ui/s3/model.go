package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/s3"
)

const maxPreviewBytes = 10 * 1024 // 10KB

// Model is the Bubble Tea model for the S3 browser.
type Model struct {
	client *s3.Client

	// Location
	bucket string   // empty = bucket list view
	prefix string   // current path prefix
	path   []string // breadcrumb segments

	// List state
	buckets    []s3.Bucket
	objects    []s3.Object
	cursor     int
	listOffset int // scroll offset for the list
	selected   map[string]bool

	// Right pane - bucket details (when in bucket list)
	hoveredBucket string
	bucketRegion  string

	// Right pane - object details (when in bucket)
	details     *s3.ObjectDetails
	preview     []byte
	previewType string
	previewView viewport.Model
	previewErr  string
	showPreview bool

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string

	// Confirmation
	confirmDelete bool
	deleteKeys    []string
}

// NewModel creates a new S3 browser model.
func NewModel(client *s3.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	return Model{
		client:   client,
		selected: make(map[string]bool),
		search:   ti,
		loading:  true,
	}
}

// Init initializes the model by loading buckets.
func (m Model) Init() tea.Cmd {
	return m.loadBucketsCmd()
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportSize()

	case bucketsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.buckets = msg.buckets
		m.statusLine = fmt.Sprintf("Loaded %d buckets", len(msg.buckets))
		return m, m.fetchBucketRegionCmd()

	case bucketRegionMsg:
		if msg.err != nil {
			m.bucketRegion = ""
			return m, nil
		}
		if m.currentBucketName() == msg.bucket {
			m.hoveredBucket = msg.bucket
			m.bucketRegion = msg.region
		}
		return m, nil

	case objectsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.objects = msg.objects
		m.cursor = 0
		m.selected = make(map[string]bool)
		m.details = nil
		m.preview = nil
		m.showPreview = false
		m.statusLine = fmt.Sprintf("Loaded %d items", len(msg.objects))
		return m, m.fetchDetailsCmd()

	case detailsLoadedMsg:
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		// Only update if still on the same object
		if m.currentObjectKey() == msg.key {
			m.details = msg.details
		}

	case previewLoadedMsg:
		if msg.err != nil {
			m.previewErr = msg.err.Error()
			return m, nil
		}
		if m.currentObjectKey() == msg.key {
			m.preview = msg.content
			m.previewType = msg.contentType
			m.previewView.SetContent(string(msg.content))
		}

	case downloadCompleteMsg:
		if msg.err != nil {
			m.statusLine = "Download failed: " + msg.err.Error()
		} else {
			m.statusLine = "Downloaded: " + msg.path
		}

	case deleteCompleteMsg:
		m.confirmDelete = false
		m.deleteKeys = nil
		if msg.err != nil {
			m.statusLine = "Delete failed: " + msg.err.Error()
		} else {
			m.statusLine = fmt.Sprintf("Deleted %d objects", msg.count)
			m.selected = make(map[string]bool)
			return m, m.loadObjectsCmd()
		}

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle confirmation dialog
	if m.confirmDelete {
		switch msg.String() {
		case "y", "Y":
			return m, m.deleteObjectsCmd()
		case "n", "N", "esc":
			m.confirmDelete = false
			m.deleteKeys = nil
		}
		return m, nil
	}

	// Handle search input
	if m.searching {
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEscape:
			m.searching = false
			m.clampCursor()
			return m, nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.clampCursor()
		return m, cmd
	}

	// Handle preview scrolling
	if m.showPreview {
		switch msg.String() {
		case "up", "k":
			m.previewView.ScrollUp(1)
			return m, nil
		case "down", "j":
			m.previewView.ScrollDown(1)
			return m, nil
		case "pgup":
			m.previewView.PageUp()
			return m, nil
		case "pgdn":
			m.previewView.PageDown()
			return m, nil
		case "p", "esc":
			m.showPreview = false
			return m, nil
		}
	}

	// Normal navigation
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
			return m, m.onCursorMove()
		}
	case "enter":
		return m.handleEnter()
	case "backspace", "h", "esc":
		return m.handleBack()
	case " ":
		m.toggleSelection()
	case "a":
		m.toggleAll()
	case "/":
		m.searching = true
		m.search.SetValue("")
		return m, m.search.Focus()
	case "p":
		if m.details != nil && isTextContent(m.details.ContentType) {
			m.showPreview = true
			m.previewErr = ""
			return m, m.fetchPreviewCmd()
		}
	case "d":
		return m, m.downloadSelectedCmd()
	case "D":
		keys := m.selectedKeys()
		if len(keys) > 0 {
			m.confirmDelete = true
			m.deleteKeys = keys
		}
	case "y":
		uri := m.getS3URI()
		if uri != "" {
			_ = clipboard.WriteAll(uri)
			m.statusLine = "Copied: " + uri
		}
	}

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

	view := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	// Overlay confirmation dialog
	if m.confirmDelete {
		dialog := m.renderConfirmDialog()
		view = m.overlayDialog(view, dialog)
	}

	return view
}

func (m Model) overlayDialog(_, dialog string) string {
	return lipgloss.Place(m.width, m.height-4, lipgloss.Center, lipgloss.Center, dialog,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("0")))
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

	// If cursor is above visible area, scroll up
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}

	// If cursor is below visible area, scroll down
	if m.cursor >= m.listOffset+visibleHeight {
		m.listOffset = m.cursor - visibleHeight + 1
	}

	// Clamp offset
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

func (m *Model) updateViewportSize() {
	// Account for border (2) + padding (2) per panel
	panelChrome := 4
	availableWidth := m.width - (panelChrome * 2)
	rightWidth := availableWidth - availableWidth/2
	innerWidth := max(rightWidth, 20)
	contentHeight := m.bodyHeight() - 2
	innerHeight := max(contentHeight-10, 1) // space for details above
	m.previewView.Width = innerWidth
	m.previewView.Height = innerHeight
}

func (m Model) maxCursorIndex() int {
	if m.bucket == "" {
		return len(m.filteredBuckets()) - 1
	}
	return len(m.filteredObjects()) - 1
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

func (m Model) currentObjectKey() string {
	if m.bucket == "" {
		return ""
	}
	objects := m.filteredObjects()
	if len(objects) == 0 || m.cursor >= len(objects) {
		return ""
	}
	return objects[m.cursor].Key
}

func (m Model) currentBucketName() string {
	if m.bucket != "" {
		return ""
	}
	buckets := m.filteredBuckets()
	if len(buckets) == 0 || m.cursor >= len(buckets) {
		return ""
	}
	return buckets[m.cursor].Name
}

func (m Model) onCursorMove() tea.Cmd {
	if m.bucket == "" {
		// In bucket list - fetch bucket region
		m.bucketRegion = ""
		return m.fetchBucketRegionCmd()
	}
	// In object list - fetch object details
	m.details = nil
	m.preview = nil
	m.showPreview = false
	m.previewErr = ""
	return m.fetchDetailsCmd()
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.bucket == "" {
		// Enter a bucket
		buckets := m.filteredBuckets()
		if len(buckets) == 0 || m.cursor >= len(buckets) {
			return m, nil
		}
		m.bucket = buckets[m.cursor].Name
		m.prefix = ""
		m.path = nil
		m.cursor = 0
		m.listOffset = 0
		m.search.SetValue("")
		m.loading = true
		return m, m.loadObjectsCmd()
	}

	// Enter a prefix (folder)
	objects := m.filteredObjects()
	if len(objects) == 0 || m.cursor >= len(objects) {
		return m, nil
	}
	obj := objects[m.cursor]
	if !obj.IsPrefix {
		return m, nil // Can't enter a file
	}

	m.prefix = obj.Key
	m.path = append(m.path, obj.Name)
	m.cursor = 0
	m.listOffset = 0
	m.search.SetValue("")
	m.loading = true
	return m, m.loadObjectsCmd()
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	if m.bucket == "" {
		return m, nil // Already at root
	}

	if m.prefix == "" {
		// Go back to bucket list
		m.bucket = ""
		m.path = nil
		m.cursor = 0
		m.listOffset = 0
		m.objects = nil
		m.selected = make(map[string]bool)
		m.details = nil
		m.search.SetValue("")
		return m, nil
	}

	// Go up one level
	if len(m.path) > 0 {
		m.path = m.path[:len(m.path)-1]
	}
	// Recalculate prefix
	if len(m.path) == 0 {
		m.prefix = ""
	} else {
		m.prefix = strings.Join(m.path, "/") + "/"
	}
	m.cursor = 0
	m.listOffset = 0
	m.search.SetValue("")
	m.loading = true
	return m, m.loadObjectsCmd()
}

func (m *Model) toggleSelection() {
	if m.bucket == "" {
		return // No selection in bucket view
	}
	objects := m.filteredObjects()
	if len(objects) == 0 || m.cursor >= len(objects) {
		return
	}
	obj := objects[m.cursor]
	if obj.IsPrefix {
		return // Don't select folders
	}
	if m.selected[obj.Key] {
		delete(m.selected, obj.Key)
	} else {
		m.selected[obj.Key] = true
	}
}

func (m *Model) toggleAll() {
	if m.bucket == "" {
		return
	}
	objects := m.filteredObjects()
	// Count non-prefix objects
	nonPrefixCount := 0
	for _, obj := range objects {
		if !obj.IsPrefix {
			nonPrefixCount++
		}
	}
	// If all selected, deselect all
	if len(m.selected) == nonPrefixCount && nonPrefixCount > 0 {
		m.selected = make(map[string]bool)
		return
	}
	// Select all non-prefix objects
	for _, obj := range objects {
		if !obj.IsPrefix {
			m.selected[obj.Key] = true
		}
	}
}

func (m Model) selectedCount() int {
	return len(m.selected)
}

func (m Model) selectedKeys() []string {
	keys := make([]string, 0, len(m.selected))
	for k := range m.selected {
		keys = append(keys, k)
	}
	return keys
}

func (m Model) getS3URI() string {
	if m.bucket == "" {
		return ""
	}
	if m.details != nil {
		return fmt.Sprintf("s3://%s/%s", m.bucket, m.details.Key)
	}
	objects := m.filteredObjects()
	if len(objects) == 0 || m.cursor >= len(objects) {
		return ""
	}
	return fmt.Sprintf("s3://%s/%s", m.bucket, objects[m.cursor].Key)
}

// Commands

func (m Model) loadBucketsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		buckets, err := m.client.ListBuckets(ctx)
		return bucketsLoadedMsg{buckets: buckets, err: err}
	}
}

func (m Model) fetchBucketRegionCmd() tea.Cmd {
	bucketName := m.currentBucketName()
	if bucketName == "" {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		region, err := m.client.GetBucketRegion(ctx, bucketName)
		return bucketRegionMsg{bucket: bucketName, region: region, err: err}
	}
}

func (m Model) loadObjectsCmd() tea.Cmd {
	bucket := m.bucket
	prefix := m.prefix
	return func() tea.Msg {
		ctx := context.Background()
		var all []s3.Object
		var token *string
		for {
			objects, next, err := m.client.ListObjects(ctx, bucket, prefix, token)
			if err != nil {
				return objectsLoadedMsg{err: err}
			}
			all = append(all, objects...)
			if next == nil {
				break
			}
			token = next
		}
		return objectsLoadedMsg{objects: all}
	}
}

func (m Model) fetchDetailsCmd() tea.Cmd {
	if m.bucket == "" {
		return nil
	}
	objects := m.filteredObjects()
	if len(objects) == 0 || m.cursor >= len(objects) {
		return nil
	}
	obj := objects[m.cursor]
	if obj.IsPrefix {
		return nil
	}
	bucket := m.bucket
	key := obj.Key
	return func() tea.Msg {
		ctx := context.Background()
		details, err := m.client.GetObjectDetails(ctx, bucket, key)
		return detailsLoadedMsg{details: details, key: key, err: err}
	}
}

func (m Model) fetchPreviewCmd() tea.Cmd {
	if m.bucket == "" || m.details == nil {
		return nil
	}
	bucket := m.bucket
	key := m.details.Key
	return func() tea.Msg {
		ctx := context.Background()
		content, contentType, err := m.client.GetObjectPreview(ctx, bucket, key, maxPreviewBytes)
		return previewLoadedMsg{content: content, contentType: contentType, key: key, err: err}
	}
}

func (m Model) downloadSelectedCmd() tea.Cmd {
	keys := m.selectedKeys()
	if len(keys) == 0 {
		// Download current item
		if m.details != nil {
			keys = []string{m.details.Key}
		} else {
			return nil
		}
	}
	bucket := m.bucket
	return func() tea.Msg {
		ctx := context.Background()
		cwd, err := os.Getwd()
		if err != nil {
			return downloadCompleteMsg{err: err}
		}
		downloadDir := filepath.Join(cwd, "sacha-downloads")
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			return downloadCompleteMsg{err: err}
		}
		var lastPath string
		for _, key := range keys {
			name := extractDisplayName(key)
			destPath := filepath.Join(downloadDir, name)
			if err := m.client.DownloadObject(ctx, bucket, key, destPath); err != nil {
				return downloadCompleteMsg{err: err}
			}
			lastPath = destPath
		}
		if len(keys) == 1 {
			return downloadCompleteMsg{path: lastPath}
		}
		return downloadCompleteMsg{path: fmt.Sprintf("%d files to sacha-downloads/", len(keys))}
	}
}

func (m Model) deleteObjectsCmd() tea.Cmd {
	bucket := m.bucket
	keys := m.deleteKeys
	return func() tea.Msg {
		ctx := context.Background()
		err := m.client.DeleteObjects(ctx, bucket, keys)
		return deleteCompleteMsg{count: len(keys), err: err}
	}
}

// StatusHelp returns context-aware help text for the status bar.
func (m Model) StatusHelp() string {
	if m.confirmDelete {
		return "y confirm, n/esc cancel"
	}
	if m.searching {
		return "enter/esc close search"
	}
	if m.showPreview {
		return "↑↓ scroll, p/esc close preview"
	}
	if m.bucket == "" {
		// Bucket list view
		return "↑↓ move, / search, enter open, y copy"
	}
	// Inside bucket
	return "↑↓ move, / search, space select, a all, d download, D delete, p preview, y copy, esc back"
}
