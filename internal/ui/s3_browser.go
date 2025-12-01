package ui

import (
	"fmt"
	"strings"

	"aws-tui/internal/aws"

	"github.com/charmbracelet/lipgloss"
)

// renderS3Browser renders the S3 object browser view
func (m Model) renderS3Browser() string {
	if m.loading {
		return boxStyle.Render(fmt.Sprintf("%s Loading objects...", m.spinner.View()))
	}

	var b strings.Builder

	// Breadcrumb
	breadcrumb := m.renderBreadcrumb()
	b.WriteString(breadcrumb)
	b.WriteString("\n\n")

	// Search input (if in search mode)
	headerLines := 3 // breadcrumb + blank line + help
	if m.s3SearchMode {
		b.WriteString(m.renderSearchInput())
		b.WriteString("\n\n")
		headerLines += 3 // search input + error (if any) + blank line
	}

	// Search status (if filtered)
	if m.s3ObjectsFiltered != nil && !m.s3SearchMode {
		searchInfo := lipgloss.NewStyle().
			Foreground(primaryColor).
			Render(fmt.Sprintf("🔍 Filtered: %d of %d objects (press / to search again, Esc to clear)",
				len(m.s3ObjectsFiltered), len(m.s3Objects)))
		b.WriteString(searchInfo)
		b.WriteString("\n\n")
		headerLines += 2
	}

	// Object list
	objectsToShow := m.s3Objects
	if m.s3ObjectsFiltered != nil {
		objectsToShow = m.s3ObjectsFiltered
	}

	// Calculate available height for object list
	availableHeight := m.height - headerLines
	if availableHeight < 5 {
		availableHeight = 5
	}

	if len(objectsToShow) == 0 {
		if m.s3ObjectsFiltered != nil {
			b.WriteString(boxStyle.Render("No objects match your search"))
		} else {
			b.WriteString(boxStyle.Render("No objects found in this location"))
		}
	} else {
		b.WriteString(m.renderS3ObjectListWithViewport(objectsToShow, availableHeight))
	}

	b.WriteString("\n\n")

	// Help text
	var helpText string
	if m.s3SearchMode {
		helpText = "Type regex pattern • Enter: search • Esc: cancel"
	} else if m.s3ObjectsFiltered != nil {
		helpText = "↑/↓: navigate • Enter: open • /: new search • Esc: clear filter • v: view • d: download"
	} else {
		helpText = "↑/↓: navigate • Enter: open • Backspace: up • /: search • v: view • d: download • Esc: back"
	}
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

// renderSearchInput renders the search input field
func (m Model) renderSearchInput() string {
	var b strings.Builder

	// Search prompt
	prompt := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Render("🔍 Search (regex): ")

	// Search query
	query := m.s3SearchQuery
	if query == "" {
		query = lipgloss.NewStyle().
			Foreground(mutedColor).
			Render("(type pattern...)")
	} else {
		query = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Render(query + "█") // Cursor
	}

	b.WriteString(prompt)
	b.WriteString(query)

	// Error message if any
	if m.s3SearchError != "" {
		b.WriteString("\n")
		errorMsg := lipgloss.NewStyle().
			Foreground(errorColor).
			Render("Error: " + m.s3SearchError)
		b.WriteString(errorMsg)
	}

	return b.String()
}

// renderBreadcrumb renders the breadcrumb navigation
func (m Model) renderBreadcrumb() string {
	if len(m.s3BreadcrumbPath) == 0 {
		return titleStyle.Render("📦 S3 Browser")
	}

	var parts []string
	for i, part := range m.s3BreadcrumbPath {
		if i == 0 {
			parts = append(parts, lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				Render("📦 "+part))
		} else if i == len(m.s3BreadcrumbPath)-1 {
			// Current location - highlighted
			parts = append(parts, lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				Render(part))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(secondaryColor).
				Render(part))
		}
	}

	return strings.Join(parts, lipgloss.NewStyle().
		Foreground(mutedColor).
		Render(" > "))
}

// renderS3ObjectListWithViewport renders the list of S3 objects with viewport scrolling
func (m Model) renderS3ObjectListWithViewport(objects []aws.S3Object, maxHeight int) string {
	if len(objects) == 0 {
		return ""
	}

	selectedIdx := m.list.Index()

	// Calculate viewport window
	// Keep selected item in the middle when possible
	startIdx := 0
	endIdx := len(objects)

	if len(objects) > maxHeight {
		// Calculate scroll position to keep selected item visible
		halfHeight := maxHeight / 2

		if selectedIdx < halfHeight {
			// Near the top
			startIdx = 0
			endIdx = maxHeight
		} else if selectedIdx >= len(objects)-halfHeight {
			// Near the bottom
			startIdx = len(objects) - maxHeight
			endIdx = len(objects)
		} else {
			// In the middle
			startIdx = selectedIdx - halfHeight
			endIdx = selectedIdx + halfHeight
			if endIdx > len(objects) {
				endIdx = len(objects)
			}
		}
	}

	var items []string

	// Show scroll indicator at top if not at start
	if startIdx > 0 {
		scrollInfo := lipgloss.NewStyle().
			Foreground(mutedColor).
			Render(fmt.Sprintf("▲ %d more above", startIdx))
		items = append(items, scrollInfo)
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		obj := objects[i]
		var icon, name, details string

		if obj.IsPrefix {
			// Folder/prefix
			icon = "📁"
			name = aws.GetObjectName(obj.Key)
			details = ""
		} else {
			// File/object
			icon = "📄"
			name = aws.GetObjectName(obj.Key)
			sizeStr := aws.FormatSize(obj.Size)
			details = fmt.Sprintf("%10s  %s", sizeStr, obj.LastModified)
		}

		// Highlight selected item
		style := normalStyle
		prefix := "  "
		if i == selectedIdx {
			style = selectedStyle
			prefix = "▶ "
		}

		line := fmt.Sprintf("%s%s %-50s %s", prefix, icon, truncate(name, 50), details)
		items = append(items, style.Render(line))
	}

	// Show scroll indicator at bottom if not at end
	if endIdx < len(objects) {
		scrollInfo := lipgloss.NewStyle().
			Foreground(mutedColor).
			Render(fmt.Sprintf("▼ %d more below", len(objects)-endIdx))
		items = append(items, scrollInfo)
	}

	content := strings.Join(items, "\n")
	return boxStyle.Width(m.width - 4).Render(content)
}

// renderS3ObjectDetail renders detailed information about an S3 object
func (m Model) renderS3ObjectDetail() string {
	if m.s3ObjectDetail == nil {
		return boxStyle.Render("No object selected")
	}

	title := titleStyle.Render(fmt.Sprintf("📄 Object Details: %s", aws.GetObjectName(m.s3ObjectDetail.Key)))

	lines := m.s3ObjectDetail.DetailLines()
	content := boxStyle.Width(m.width - 4).Render(strings.Join(lines, "\n"))

	var helpText string
	if m.s3ObjectDetail.IsPreviewable {
		helpText = "v: view content • d: download • c: copy URL • Esc: back"
	} else {
		helpText = "d: download • c: copy URL • Esc: back"
	}
	help := helpStyle.Render(helpText)

	return fmt.Sprintf("%s\n%s\n%s", title, content, help)
}

// renderS3ObjectContent renders the content of an S3 object
func (m Model) renderS3ObjectContent() string {
	if m.s3ObjectDetail == nil {
		return boxStyle.Render("No object selected")
	}

	objectName := aws.GetObjectName(m.s3ObjectDetail.Key)
	sizeInfo := ""
	if m.s3ObjectDetail.Size > m.cfg.Resources.S3.ContentPreviewSize {
		sizeInfo = fmt.Sprintf(" (showing first %s)", aws.FormatSize(m.cfg.Resources.S3.ContentPreviewSize))
	}

	title := titleStyle.Render(fmt.Sprintf("📄 Content: %s%s", objectName, sizeInfo))

	content := boxStyle.Width(m.width - 4).Height(m.height - 10).Render(m.s3ContentViewer.View())

	helpText := "↑/↓: scroll • d: download full • Esc: back"
	help := helpStyle.Render(helpText)

	return fmt.Sprintf("%s\n%s\n%s", title, content, help)
}

// Helper function to truncate strings (if not already defined)
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	if length <= 3 {
		return s[:length]
	}
	return s[:length-3] + "..."
}

// Made with Bob
