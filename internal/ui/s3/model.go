package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/cache"
	"github.com/sachamama/sacha/internal/s3"
)

const maxPreviewBytes = 10 * 1024 // 10KB

// scrollPosition stores cursor state for scroll memory.
type scrollPosition struct {
	cursor     int
	listOffset int
}

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

	// Pagination
	nextToken   *string // continuation token for lazy loading
	loadingMore bool    // loading next page

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

	// Cache
	cache    *cache.Cache
	cacheKey cache.Key

	// UI
	width      int
	height     int
	searching  bool
	search     textinput.Model
	loading    bool
	statusLine string

	// Scroll memory: stack of saved positions for hierarchical navigation
	scrollStack    []scrollPosition
	pendingRestore *scrollPosition // restore after next objectsLoadedMsg

	// Download progress
	downloading     bool
	downloadChan    <-chan tea.Msg
	downloadFile    string
	downloadIndex   int
	downloadTotal   int
	downloadBytes   int64
	downloadSize    int64
	downloadedBytes int64
	downloadGrand   int64
}

// NewModel creates a new S3 browser model.
func NewModel(client *s3.Client, c *cache.Cache, cacheKey cache.Key) Model {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Prompt = "/ "
	m := Model{
		client:   client,
		selected: make(map[string]bool),
		search:   ti,
		loading:  true,
		cache:    c,
		cacheKey: cacheKey,
	}
	if c != nil {
		if items, ok := c.Get(cacheKey); ok {
			if buckets, ok := items.([]s3.Bucket); ok && len(buckets) > 0 {
				m.buckets = buckets
				m.loading = false
				m.statusLine = fmt.Sprintf("%d buckets (cached)", len(buckets))
			}
		}
	}
	return m
}

// Init initializes the model by loading buckets.
func (m Model) Init() tea.Cmd {
	if len(m.buckets) > 0 {
		return nil
	}
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
		sortBuckets(m.buckets)
		m.updateCache()
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
		m.nextToken = msg.nextToken
		m.cursor = 0
		m.selected = make(map[string]bool)
		m.details = nil
		m.preview = nil
		m.showPreview = false
		// Restore scroll position if navigating back
		if m.pendingRestore != nil {
			m.cursor = m.pendingRestore.cursor
			m.listOffset = m.pendingRestore.listOffset
			m.pendingRestore = nil
			m.clampCursor()
		}
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d items (loading more...)", len(msg.objects))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d items", len(msg.objects))
		}
		return m, tea.Batch(m.fetchDetailsCmd(), m.loadMoreIfNeeded())

	case moreObjectsLoadedMsg:
		m.loadingMore = false
		if msg.err != nil {
			m.statusLine = msg.err.Error()
			return m, nil
		}
		m.objects = append(m.objects, msg.objects...)
		m.nextToken = msg.nextToken
		if m.nextToken != nil {
			m.statusLine = fmt.Sprintf("Loaded %d items (loading more...)", len(m.objects))
		} else {
			m.statusLine = fmt.Sprintf("Loaded %d items", len(m.objects))
		}
		return m, m.loadMoreIfNeeded()

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

	case downloadStartedMsg:
		m.downloading = true
		m.downloadChan = msg.ch
		return m, listenForProgress(msg.ch)

	case downloadProgressMsg:
		m.downloading = true
		m.downloadFile = msg.currentFile
		m.downloadIndex = msg.fileIndex
		m.downloadTotal = msg.totalFiles
		m.downloadBytes = msg.bytesWritten
		m.downloadSize = msg.totalBytes
		m.downloadedBytes = msg.totalDownloaded
		m.downloadGrand = msg.grandTotal
		return m, listenForProgress(m.downloadChan)

	case downloadCompleteMsg:
		m.downloading = false
		m.downloadChan = nil
		if msg.err != nil {
			m.statusLine = "Download failed: " + msg.err.Error()
		} else {
			m.statusLine = "Downloaded: " + msg.path
		}

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
		case "pgdown", "pgdn":
			m.previewView.PageDown()
			return m, nil
		case "home":
			m.previewView.GotoTop()
			return m, nil
		case "end":
			m.previewView.GotoBottom()
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
			cmd := m.onCursorMove()
			// Lazy load more when near the end
			if m.bucket != "" && m.nextToken != nil && !m.loadingMore {
				if m.cursor >= len(m.filteredObjects())-5 {
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
	case "y":
		uri := m.getS3URI()
		if uri != "" {
			_ = clipboard.WriteAll(uri)
			m.statusLine = "Copied: " + uri
		}
	case "ctrl+r":
		if m.bucket == "" {
			if m.cache != nil {
				m.cache.Delete(m.cacheKey)
			}
			m.buckets = nil
			m.cursor = 0
			m.listOffset = 0
			m.loading = true
			m.statusLine = "Refreshing..."
			return m, m.loadBucketsCmd()
		}
		m.objects = nil
		m.nextToken = nil
		m.selected = make(map[string]bool)
		m.cursor = 0
		m.listOffset = 0
		m.details = nil
		m.loading = true
		m.statusLine = "Refreshing..."
		return m, m.loadObjectsCmd()
	}

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

	view := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

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

	// If cursor is above visible area, scroll up
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

	// If cursor is below visible area, scroll down
	if m.cursor >= m.listOffset+effective {
		m.listOffset = m.cursor - effective + 1
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
	rightWidth := availableWidth - availableWidth*2/5
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

// jumpCursor moves the cursor to idx (clamped to the list bounds), updates
// scroll visibility, and returns the command to refresh the right pane,
// lazy-loading the next page of objects when the cursor lands near the end.
// Used by page and home/end navigation.
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
	if m.bucket != "" && m.nextToken != nil && !m.loadingMore && m.cursor >= len(m.filteredObjects())-5 {
		m.loadingMore = true
		return tea.Batch(cmd, m.loadMoreCmd())
	}
	return cmd
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
		// Save scroll position before entering bucket
		m.scrollStack = append(m.scrollStack, scrollPosition{cursor: m.cursor, listOffset: m.listOffset})
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

	// Save scroll position before entering folder
	m.scrollStack = append(m.scrollStack, scrollPosition{cursor: m.cursor, listOffset: m.listOffset})
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

	// Restore saved scroll position
	var saved scrollPosition
	if len(m.scrollStack) > 0 {
		saved = m.scrollStack[len(m.scrollStack)-1]
		m.scrollStack = m.scrollStack[:len(m.scrollStack)-1]
	}

	if m.prefix == "" {
		// Go back to bucket list
		m.bucket = ""
		m.path = nil
		m.cursor = saved.cursor
		m.listOffset = saved.listOffset
		m.objects = nil
		m.selected = make(map[string]bool)
		m.details = nil
		m.search.SetValue("")
		m.clampCursor()
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
	// Defer restore until objects are loaded
	m.pendingRestore = &saved
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
	// Allow selecting both files and folders
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
	// If all selected, deselect all
	if len(m.selected) == len(objects) && len(objects) > 0 {
		m.selected = make(map[string]bool)
		return
	}
	// Select all objects (including folders for recursive download)
	for _, obj := range objects {
		m.selected[obj.Key] = true
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

func (m *Model) loadMoreIfNeeded() tea.Cmd {
	if m.bucket == "" {
		return nil // buckets are not paginated
	}
	if m.nextToken == nil || m.loadingMore {
		return nil
	}
	m.loadingMore = true
	return m.loadMoreCmd()
}

// updateCache stores the current buckets in the cache.
func (m *Model) updateCache() {
	if m.cache != nil {
		m.cache.Set(m.cacheKey, m.buckets)
	}
}

// sortBuckets sorts buckets by Name (case-insensitive).
func sortBuckets(buckets []s3.Bucket) {
	sort.Slice(buckets, func(i, j int) bool {
		return strings.ToLower(buckets[i].Name) < strings.ToLower(buckets[j].Name)
	})
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
		objects, nextToken, err := m.client.ListObjects(ctx, bucket, prefix, nil)
		if err != nil {
			return objectsLoadedMsg{err: err}
		}
		return objectsLoadedMsg{objects: objects, nextToken: nextToken}
	}
}

func (m Model) loadMoreCmd() tea.Cmd {
	bucket := m.bucket
	prefix := m.prefix
	token := m.nextToken
	return func() tea.Msg {
		ctx := context.Background()
		objects, nextToken, err := m.client.ListObjects(ctx, bucket, prefix, token)
		if err != nil {
			return moreObjectsLoadedMsg{err: err}
		}
		return moreObjectsLoadedMsg{objects: objects, nextToken: nextToken}
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
		// Download current item (file or folder)
		objects := m.filteredObjects()
		if len(objects) > 0 && m.cursor < len(objects) {
			keys = []string{objects[m.cursor].Key}
		} else if m.details != nil {
			keys = []string{m.details.Key}
		} else {
			return nil
		}
	}
	bucket := m.bucket
	client := m.client

	// Separate folders (prefixes) from files
	var folders []string
	var files []string
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			folders = append(folders, key)
		} else {
			files = append(files, key)
		}
	}

	// Create a channel for progress updates
	progressChan := make(chan tea.Msg, 100)

	// Start download in background
	go func() {
		defer close(progressChan)

		ctx := context.Background()
		cwd, err := os.Getwd()
		if err != nil {
			progressChan <- downloadCompleteMsg{err: err}
			return
		}
		downloadDir := filepath.Join(cwd, "sacha-downloads")
		if err := os.MkdirAll(downloadDir, 0o750); err != nil {
			progressChan <- downloadCompleteMsg{err: err}
			return
		}

		// Build list of all files to download
		type downloadItem struct {
			key      string
			destPath string
			size     int64
		}
		var items []downloadItem

		// Add individual files
		for _, key := range files {
			name := extractDisplayName(key)
			destPath := filepath.Join(downloadDir, name)
			items = append(items, downloadItem{key: key, destPath: destPath})
		}

		// Add folder contents
		for _, folderKey := range folders {
			folderObjects, err := client.ListAllObjects(ctx, bucket, folderKey)
			if err != nil {
				progressChan <- downloadCompleteMsg{err: err}
				return
			}
			folderName := strings.TrimSuffix(extractDisplayName(folderKey), "/")
			for _, obj := range folderObjects {
				relativePath := strings.TrimPrefix(obj.Key, folderKey)
				destPath := filepath.Join(downloadDir, folderName, relativePath)
				items = append(items, downloadItem{key: obj.Key, destPath: destPath, size: obj.Size})
			}
		}

		if len(items) == 0 {
			progressChan <- downloadCompleteMsg{path: "No files to download"}
			return
		}

		// Calculate total size (approximate for files without size info)
		var grandTotal int64
		for _, item := range items {
			grandTotal += item.size
		}

		var totalDownloaded int64
		for i, item := range items {
			fileName := extractDisplayName(item.key)

			// Send initial progress for this file
			progressChan <- downloadProgressMsg{
				currentFile:     fileName,
				fileIndex:       i + 1,
				totalFiles:      len(items),
				bytesWritten:    0,
				totalBytes:      item.size,
				totalDownloaded: totalDownloaded,
				grandTotal:      grandTotal,
			}

			var fileBytes int64
			err := client.DownloadObjectWithProgress(ctx, bucket, item.key, item.destPath, func(written, total int64) {
				fileBytes = written
				progressChan <- downloadProgressMsg{
					currentFile:     fileName,
					fileIndex:       i + 1,
					totalFiles:      len(items),
					bytesWritten:    written,
					totalBytes:      total,
					totalDownloaded: totalDownloaded + written,
					grandTotal:      grandTotal,
				}
			})
			if err != nil {
				progressChan <- downloadCompleteMsg{err: err}
				return
			}
			totalDownloaded += fileBytes
		}

		if len(items) == 1 {
			progressChan <- downloadCompleteMsg{path: "1 file to sacha-downloads/"}
		} else {
			progressChan <- downloadCompleteMsg{path: fmt.Sprintf("%d files to sacha-downloads/", len(items))}
		}
	}()

	// Return command that sends the channel to the model, which starts listening
	return func() tea.Msg {
		return downloadStartedMsg{ch: progressChan}
	}
}

// listenForProgress returns a command that listens for the next progress message.
func listenForProgress(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
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
	if m.showPreview {
		return "↑↓ scroll, pgup/pgdn page, home/end top/bottom, p/esc close preview"
	}
	if m.bucket == "" {
		// Bucket list view
		return "↑↓ move, / search, enter open, y copy, ctrl+r refresh"
	}
	// Inside bucket
	return "↑↓ move, / search, space select, a all, d download, p preview, y copy, ctrl+r refresh, esc back"
}
