package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"aws-tui/internal/aws"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Regex cache to avoid recompiling the same patterns
var (
	regexCache      = make(map[string]*regexp.Regexp)
	regexCacheMutex sync.RWMutex
	maxCacheSize    = 50 // Limit cache size to prevent memory issues
)

// getCompiledRegex returns a compiled regex from cache or compiles and caches it
func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	// Check cache first (read lock)
	regexCacheMutex.RLock()
	if re, exists := regexCache[pattern]; exists {
		regexCacheMutex.RUnlock()
		return re, nil
	}
	regexCacheMutex.RUnlock()

	// Compile regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	// Store in cache (write lock)
	regexCacheMutex.Lock()
	defer regexCacheMutex.Unlock()

	// Limit cache size - simple FIFO eviction
	if len(regexCache) >= maxCacheSize {
		// Clear half the cache
		count := 0
		for k := range regexCache {
			delete(regexCache, k)
			count++
			if count >= maxCacheSize/2 {
				break
			}
		}
	}

	regexCache[pattern] = re
	return re, nil
}

// filterS3ObjectsByRegex filters objects using regex pattern (optimized for performance)
func filterS3ObjectsByRegex(objects []aws.S3Object, pattern string) ([]aws.S3Object, error) {
	if pattern == "" {
		return objects, nil
	}

	// Get compiled regex (from cache if possible)
	re, err := getCompiledRegex(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	// Pre-allocate slice with estimated capacity
	filtered := make([]aws.S3Object, 0, len(objects)/10)

	// Filter objects - optimized loop
	for i := range objects {
		// Search in object name (not full key for better UX)
		name := aws.GetObjectName(objects[i].Key)
		if re.MatchString(name) {
			filtered = append(filtered, objects[i])
		}
	}

	return filtered, nil
}

// fetchS3Objects fetches objects from an S3 bucket
func (m Model) fetchS3Objects() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		delimiter := "/"
		maxKeys := int32(m.cfg.Resources.S3.MaxObjectsPerPage)

		objects, err := m.client.ListObjects(
			ctx,
			m.currentBucket,
			m.currentPrefix,
			delimiter,
			maxKeys,
		)

		return s3ObjectsFetchedMsg{
			objects: objects,
			err:     err,
		}
	}
}

// fetchS3ObjectDetail fetches detailed information about an S3 object
func (m Model) fetchS3ObjectDetail(bucket, key string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		detail, err := m.client.GetObjectMetadata(ctx, bucket, key)
		if err != nil {
			return s3ObjectDetailFetchedMsg{err: err}
		}

		// Check if content is previewable
		detail.IsPreviewable = aws.IsPreviewableContent(
			detail.ContentType,
			m.cfg.Resources.S3.SupportedPreviewTypes,
		)

		return s3ObjectDetailFetchedMsg{
			detail: detail,
			err:    nil,
		}
	}
}

// fetchS3ObjectContent fetches the content of an S3 object
func (m Model) fetchS3ObjectContent(bucket, key string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		maxSize := m.cfg.Resources.S3.ContentPreviewSize
		content, err := m.client.GetObjectContent(ctx, bucket, key, maxSize)
		if err != nil {
			return s3ObjectContentFetchedMsg{err: err}
		}

		return s3ObjectContentFetchedMsg{
			content: string(content),
			err:     nil,
		}
	}
}

// downloadS3Object downloads an S3 object to local filesystem
func (m Model) downloadS3Object(bucket, key string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// Expand download directory
		downloadDir := m.cfg.Resources.S3.DownloadDirectory
		if strings.HasPrefix(downloadDir, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				downloadDir = filepath.Join(home, downloadDir[1:])
			}
		}

		// Create local path
		objectName := aws.GetObjectName(key)
		localPath := filepath.Join(downloadDir, objectName)

		// Download
		err := m.client.DownloadObject(ctx, bucket, key, localPath)

		return s3DownloadCompleteMsg{
			path: localPath,
			err:  err,
		}
	}
}

// handleS3BrowserKeys handles key presses in S3 browser view
func (m *Model) handleS3BrowserKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle search mode
	if m.s3SearchMode {
		return m.handleS3SearchKeys(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Search):
		// Enter search mode
		m.s3SearchMode = true
		m.s3SearchQuery = ""
		m.s3SearchError = ""
		return m, nil

	case key.Matches(msg, m.keys.Back):
		// Clear filter if active, otherwise go back
		if m.s3ObjectsFiltered != nil {
			m.s3ObjectsFiltered = nil
			m.s3SearchQuery = ""
			m.updateS3ObjectsList()
			m.statusMsg = "Filter cleared"
		} else {
			// Go back to bucket list
			m.viewMode = ViewList
			m.currentBucket = ""
			m.currentPrefix = ""
			m.s3Objects = nil
			m.s3BreadcrumbPath = nil
		}

	case key.Matches(msg, m.keys.GoUp):
		// Go up one level
		if m.currentPrefix != "" {
			m.currentPrefix = aws.GetParentPrefix(m.currentPrefix)
			m.s3BreadcrumbPath = aws.BuildBreadcrumb(m.currentBucket, m.currentPrefix)
			m.s3ObjectsFiltered = nil // Clear filter when navigating
			m.s3SearchQuery = ""
			m.loading = true
			cmds = append(cmds, m.fetchS3Objects(), m.spinner.Tick)
		} else {
			// At root, go back to bucket list
			m.viewMode = ViewList
			m.currentBucket = ""
			m.s3Objects = nil
			m.s3ObjectsFiltered = nil
			m.s3BreadcrumbPath = nil
		}

	case key.Matches(msg, m.keys.Refresh):
		m.s3ObjectsFiltered = nil // Clear filter when refreshing
		m.s3SearchQuery = ""
		m.loading = true
		cmds = append(cmds, m.fetchS3Objects(), m.spinner.Tick)

	case key.Matches(msg, m.keys.Enter):
		// Navigate into folder or view object details
		if m.list.SelectedItem() != nil && len(m.s3Objects) > 0 {
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.s3Objects) {
				obj := m.s3Objects[idx]

				if obj.IsPrefix {
					// Navigate into folder
					m.currentPrefix = obj.Key
					m.s3BreadcrumbPath = aws.BuildBreadcrumb(m.currentBucket, m.currentPrefix)
					m.loading = true
					cmds = append(cmds, m.fetchS3Objects(), m.spinner.Tick)
				} else {
					// View object details
					m.loading = true
					cmds = append(cmds, m.fetchS3ObjectDetail(m.currentBucket, obj.Key), m.spinner.Tick)
				}
			}
		}

	case key.Matches(msg, m.keys.View):
		// View object content
		if m.list.SelectedItem() != nil && len(m.s3Objects) > 0 {
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.s3Objects) {
				obj := m.s3Objects[idx]
				if !obj.IsPrefix {
					m.loading = true
					cmds = append(cmds, m.fetchS3ObjectDetail(m.currentBucket, obj.Key), m.spinner.Tick)
				}
			}
		}

	case key.Matches(msg, m.keys.Download):
		// Download object
		if m.list.SelectedItem() != nil && len(m.s3Objects) > 0 {
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.s3Objects) {
				obj := m.s3Objects[idx]
				if !obj.IsPrefix {
					m.loading = true
					m.statusMsg = fmt.Sprintf("Downloading %s...", aws.GetObjectName(obj.Key))
					cmds = append(cmds, m.downloadS3Object(m.currentBucket, obj.Key), m.spinner.Tick)
				}
			}
		}

	case key.Matches(msg, m.keys.Copy):
		// Copy object key
		if m.list.SelectedItem() != nil && len(m.s3Objects) > 0 {
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.s3Objects) {
				obj := m.s3Objects[idx]
				if err := clipboard.WriteAll(obj.Key); err != nil {
					m.statusMsg = fmt.Sprintf("Copy failed: %v", err)
				} else {
					m.statusMsg = fmt.Sprintf("Copied: %s", obj.Key)
				}
			}
		}

	default:
		// Handle list navigation
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleS3SearchKeys handles key presses in search mode
func (m *Model) handleS3SearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Exit search mode and restore full list
		m.s3SearchMode = false
		m.s3SearchQuery = ""
		m.s3SearchError = ""
		m.s3ObjectsFiltered = nil
		m.updateS3ObjectsList()
		m.statusMsg = "Search cancelled"
		return m, nil

	case "enter":
		// Apply search
		if m.s3SearchQuery == "" {
			// Empty search - show all
			m.s3SearchMode = false
			m.s3ObjectsFiltered = nil
			m.updateS3ObjectsList()
			m.statusMsg = "Showing all objects"
		} else {
			// Perform regex search
			filtered, err := filterS3ObjectsByRegex(m.s3Objects, m.s3SearchQuery)
			if err != nil {
				m.s3SearchError = err.Error()
				return m, nil
			}
			m.s3ObjectsFiltered = filtered
			m.s3SearchMode = false
			m.s3SearchError = ""
			m.updateS3ObjectsList()
			m.statusMsg = fmt.Sprintf("Found %d matches", len(filtered))
		}
		return m, nil

	case "backspace", "delete":
		// Remove last character
		if len(m.s3SearchQuery) > 0 {
			m.s3SearchQuery = m.s3SearchQuery[:len(m.s3SearchQuery)-1]
			m.s3SearchError = "" // Clear error when typing
		}
		return m, nil

	default:
		// Add character to search query (handle all other keys as input)
		if len(msg.Runes) > 0 {
			m.s3SearchQuery += string(msg.Runes)
			m.s3SearchError = "" // Clear error when typing
		}
		return m, nil
	}
}

// handleS3ObjectDetailKeys handles key presses in S3 object detail view
func (m *Model) handleS3ObjectDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, m.keys.Back):
		// Go back to browser
		m.viewMode = ViewS3Browser
		m.s3ObjectDetail = nil

	case key.Matches(msg, m.keys.View):
		// View content if previewable
		if m.s3ObjectDetail != nil && m.s3ObjectDetail.IsPreviewable {
			m.loading = true
			cmds = append(cmds, m.fetchS3ObjectContent(m.currentBucket, m.s3ObjectDetail.Key), m.spinner.Tick)
		} else {
			m.statusMsg = "Content preview not available for this file type"
		}

	case key.Matches(msg, m.keys.Download):
		// Download object
		if m.s3ObjectDetail != nil {
			m.loading = true
			m.statusMsg = fmt.Sprintf("Downloading %s...", aws.GetObjectName(m.s3ObjectDetail.Key))
			cmds = append(cmds, m.downloadS3Object(m.currentBucket, m.s3ObjectDetail.Key), m.spinner.Tick)
		}

	case key.Matches(msg, m.keys.Copy):
		// Generate and copy presigned URL
		if m.s3ObjectDetail != nil {
			url, err := m.client.GeneratePresignedURL(
				context.Background(),
				m.currentBucket,
				m.s3ObjectDetail.Key,
				15*time.Minute,
			)
			if err != nil {
				m.statusMsg = fmt.Sprintf("Failed to generate URL: %v", err)
			} else {
				if err := clipboard.WriteAll(url); err != nil {
					m.statusMsg = fmt.Sprintf("Copy failed: %v", err)
				} else {
					m.statusMsg = "Presigned URL copied to clipboard (valid for 15 minutes)"
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// handleS3ObjectContentKeys handles key presses in S3 object content view
func (m *Model) handleS3ObjectContentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, m.keys.Back):
		// Go back to object detail
		m.viewMode = ViewS3ObjectDetail
		m.s3ContentViewer = viewport.Model{}

	case key.Matches(msg, m.keys.Download):
		// Download full object
		if m.s3ObjectDetail != nil {
			m.loading = true
			m.statusMsg = fmt.Sprintf("Downloading %s...", aws.GetObjectName(m.s3ObjectDetail.Key))
			cmds = append(cmds, m.downloadS3Object(m.currentBucket, m.s3ObjectDetail.Key), m.spinner.Tick)
		}

	case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down):
		// Scroll content
		var cmd tea.Cmd
		m.s3ContentViewer, cmd = m.s3ContentViewer.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateS3ObjectsList updates the list view with S3 objects
func (m *Model) updateS3ObjectsList() {
	var items []list.Item

	// Use filtered results if search is active, otherwise use all objects
	objectsToShow := m.s3Objects
	if m.s3ObjectsFiltered != nil {
		objectsToShow = m.s3ObjectsFiltered
	}

	for _, obj := range objectsToShow {
		var title, desc string

		if obj.IsPrefix {
			title = "📁 " + aws.GetObjectName(obj.Key)
			desc = "Folder"
		} else {
			title = "📄 " + aws.GetObjectName(obj.Key)
			desc = fmt.Sprintf("%s | %s", aws.FormatSize(obj.Size), obj.LastModified)
		}

		items = append(items, listItem{
			title:    title,
			desc:     desc,
			resource: obj,
		})
	}

	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("📦 %s (%d items)", m.currentBucket, len(items))
}

// Made with Bob
