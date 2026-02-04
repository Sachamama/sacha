package s3

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sachamama/sacha/internal/s3"
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

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("51"))

	dimText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("44"))

	folderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	progressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("213"))

	progressFillStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("213")).
				Background(lipgloss.Color("213"))

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238")).
				Background(lipgloss.Color("238"))
)

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
	} else {
		fmt.Fprintln(b, dimText.Render("Press / to filter"))
	}

	// Calculate visible height for list (total - header - search - footer - status - borders)
	visibleHeight := m.listHeight()

	// List items
	if m.bucket == "" {
		m.renderBucketList(b, visibleHeight)
	} else {
		m.renderObjectList(b, visibleHeight)
	}

	// Status footer
	fmt.Fprintln(b)
	if m.bucket == "" {
		fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Total: %d buckets", len(m.buckets))))
	} else {
		fmt.Fprintf(b, "%s\n", dimText.Render(fmt.Sprintf("Selected: %d | Total: %d", m.selectedCount(), len(m.filteredObjects()))))
	}

	if m.statusLine != "" {
		fmt.Fprintf(b, "%s\n", statusStyle.Render(m.statusLine))
	}

	// Download progress bar
	if m.downloading {
		fmt.Fprintln(b)
		fmt.Fprintln(b, m.renderProgressBar())
	}

	return b.String()
}

func (m Model) renderBreadcrumb() string {
	if m.bucket == "" {
		return "s3://"
	}
	if m.prefix == "" {
		return fmt.Sprintf("s3://%s/", m.bucket)
	}
	return fmt.Sprintf("s3://%s/%s", m.bucket, m.prefix)
}

func (m Model) renderBucketList(b *strings.Builder, visibleHeight int) {
	buckets := m.filteredBuckets()
	if len(buckets) == 0 {
		fmt.Fprintln(b, dimText.Render("No buckets found"))
		return
	}

	// Show scroll indicator if needed
	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.listOffset+visibleHeight, len(buckets))

	for i := m.listOffset; i < endIdx; i++ {
		bucket := buckets[i]
		line := fmt.Sprintf("  %s", bucket.Name)
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(buckets) {
		fmt.Fprintln(b, dimText.Render("  ↓ more below"))
	}
}

func (m Model) renderObjectList(b *strings.Builder, visibleHeight int) {
	objects := m.filteredObjects()
	if len(objects) == 0 {
		fmt.Fprintln(b, dimText.Render("No objects found"))
		return
	}

	// Show scroll indicator if needed
	if m.listOffset > 0 {
		fmt.Fprintln(b, dimText.Render("  ↑ more above"))
		visibleHeight--
	}

	endIdx := min(m.listOffset+visibleHeight, len(objects))

	for i := m.listOffset; i < endIdx; i++ {
		obj := objects[i]
		var line string
		checkbox := " "
		if m.selected[obj.Key] {
			checkbox = "x"
		}

		if obj.IsPrefix {
			line = fmt.Sprintf("[%s] %s/", checkbox, folderStyle.Render(obj.Name))
		} else {
			line = fmt.Sprintf("[%s] %s  %s", checkbox, obj.Name, dimText.Render(formatSize(obj.Size)))
		}

		if i == m.cursor {
			line = cursorStyle.Render(line)
		} else if m.selected[obj.Key] {
			line = selectedStyle.Render(line)
		}
		fmt.Fprintln(b, line)
	}

	if endIdx < len(objects) {
		fmt.Fprintln(b, dimText.Render("  ↓ more below"))
	}
}

func (m Model) renderRight() string {
	b := &strings.Builder{}

	if m.bucket == "" {
		fmt.Fprintln(b, titleStyle.Render("Bucket Details"))
		buckets := m.filteredBuckets()
		if len(buckets) == 0 || m.cursor >= len(buckets) {
			fmt.Fprintln(b, dimText.Render("No bucket selected"))
			return b.String()
		}
		bucket := buckets[m.cursor]
		region := m.bucketRegion
		if region == "" || m.hoveredBucket != bucket.Name {
			region = "loading..."
		}
		fmt.Fprintln(b)
		fmt.Fprintf(b, "Name:     %s\n", bucket.Name)
		fmt.Fprintf(b, "Created:  %s\n", bucket.CreationDate.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(b, "Region:   %s\n", region)
		fmt.Fprintln(b)
		fmt.Fprintln(b, dimText.Render("URIs"))
		fmt.Fprintf(b, "S3:       s3://%s/\n", bucket.Name)
		fmt.Fprintf(b, "ARN:      arn:aws:s3:::%s\n", bucket.Name)
		if region != "" && region != "loading..." {
			fmt.Fprintf(b, "HTTPS:    https://%s.s3.%s.amazonaws.com/\n", bucket.Name, region)
			fmt.Fprintf(b, "Console:  https://%s.console.aws.amazon.com/s3/buckets/%s\n", region, bucket.Name)
		}
		return b.String()
	}

	// Show object details
	fmt.Fprintln(b, titleStyle.Render("Object Details"))

	if m.details == nil {
		objects := m.filteredObjects()
		if len(objects) == 0 || m.cursor >= len(objects) {
			fmt.Fprintln(b, dimText.Render("No object selected"))
			return b.String()
		}
		obj := objects[m.cursor]
		if obj.IsPrefix {
			fmt.Fprintln(b)
			fmt.Fprintf(b, "Folder:   %s\n", obj.Name)
			fmt.Fprintln(b)
			fmt.Fprintln(b, dimText.Render("URIs"))
			fmt.Fprintf(b, "S3:       s3://%s/%s\n", m.bucket, obj.Key)
			fmt.Fprintf(b, "ARN:      arn:aws:s3:::%s/%s\n", m.bucket, obj.Key)
			if m.bucketRegion != "" {
				fmt.Fprintf(b, "HTTPS:    https://%s.s3.%s.amazonaws.com/%s\n", m.bucket, m.bucketRegion, obj.Key)
			}
		} else {
			fmt.Fprintln(b, dimText.Render("Loading details..."))
		}
		return b.String()
	}

	fmt.Fprintln(b)
	fmt.Fprintf(b, "Key:           %s\n", m.details.Key)
	fmt.Fprintf(b, "Size:          %s (%d bytes)\n", formatSize(m.details.Size), m.details.Size)
	fmt.Fprintf(b, "Last Modified: %s\n", m.details.LastModified.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(b, "Storage Class: %s\n", m.details.StorageClass)
	fmt.Fprintf(b, "Content-Type:  %s\n", m.details.ContentType)
	fmt.Fprintf(b, "ETag:          %s\n", m.details.ETag)
	fmt.Fprintln(b)
	fmt.Fprintln(b, dimText.Render("URIs"))
	fmt.Fprintf(b, "S3:            s3://%s/%s\n", m.bucket, m.details.Key)
	fmt.Fprintf(b, "ARN:           arn:aws:s3:::%s/%s\n", m.bucket, m.details.Key)
	if m.bucketRegion != "" {
		fmt.Fprintf(b, "HTTPS:         https://%s.s3.%s.amazonaws.com/%s\n", m.bucket, m.bucketRegion, m.details.Key)
	}

	// Preview section
	fmt.Fprintln(b)
	if m.showPreview {
		fmt.Fprintln(b, titleStyle.Render("Preview")+" "+dimText.Render("(press p to hide)"))
		if m.preview != nil {
			fmt.Fprintln(b, m.previewView.View())
		} else if m.previewErr != "" {
			fmt.Fprintln(b, errorStyle.Render(m.previewErr))
		} else {
			fmt.Fprintln(b, dimText.Render("Loading preview..."))
		}
	} else {
		if isTextContent(m.details.ContentType) {
			fmt.Fprintln(b, dimText.Render("Press p to preview"))
		}
	}

	return b.String()
}

func (m Model) renderProgressBar() string {
	b := &strings.Builder{}

	// File progress
	fileInfo := fmt.Sprintf("Downloading %d/%d: %s", m.downloadIndex, m.downloadTotal, m.downloadFile)
	fmt.Fprintln(b, progressBarStyle.Render(fileInfo))

	// Calculate percentage
	var percent float64
	if m.downloadGrand > 0 {
		percent = float64(m.downloadedBytes) / float64(m.downloadGrand) * 100
	}

	// Progress bar width (leave room for percentage text and borders)
	barWidth := 30
	filled := min(int(percent/100*float64(barWidth)), barWidth)

	// Build the bar
	bar := progressFillStyle.Render(strings.Repeat("█", filled)) +
		progressEmptyStyle.Render(strings.Repeat("░", barWidth-filled))

	// Format progress info
	progressInfo := fmt.Sprintf(" %s / %s (%.0f%%)",
		formatSize(m.downloadedBytes),
		formatSize(m.downloadGrand),
		percent)

	fmt.Fprintln(b, bar+progressInfo)

	return b.String()
}

func formatSize(bytes int64) string {
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

func extractDisplayName(key string) string {
	parts := strings.Split(strings.TrimSuffix(key, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return key
}

func isTextContent(contentType string) bool {
	if contentType == "" {
		return false
	}
	textTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-yaml",
		"application/yaml",
	}
	for _, t := range textTypes {
		if strings.HasPrefix(contentType, t) {
			return true
		}
	}
	return false
}

func (m Model) filteredBuckets() []s3.Bucket {
	if m.search.Value() == "" {
		return m.buckets
	}
	q := strings.ToLower(m.search.Value())
	out := make([]s3.Bucket, 0, len(m.buckets))
	for _, b := range m.buckets {
		if strings.Contains(strings.ToLower(b.Name), q) {
			out = append(out, b)
		}
	}
	return out
}

func (m Model) filteredObjects() []s3.Object {
	if m.search.Value() == "" {
		return m.objects
	}
	q := strings.ToLower(m.search.Value())
	out := make([]s3.Object, 0, len(m.objects))
	for _, o := range m.objects {
		if strings.Contains(strings.ToLower(o.Name), q) {
			out = append(out, o)
		}
	}
	return out
}
