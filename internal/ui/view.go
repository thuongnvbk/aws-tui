package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("170")
	secondaryColor = lipgloss.Color("240")
	successColor   = lipgloss.Color("82")
	warningColor   = lipgloss.Color("214")
	errorColor     = lipgloss.Color("196")
	mutedColor     = lipgloss.Color("241")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginLeft(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1).
			MarginBottom(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	normalStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	resourceTabStyle = lipgloss.NewStyle().
				Padding(0, 2).
				MarginRight(1)

	activeTabStyle = resourceTabStyle.
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(primaryColor)

	inactiveTabStyle = resourceTabStyle.
				Foreground(secondaryColor).
				Background(lipgloss.Color("236"))
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Resource tabs
	tabs := m.renderTabs()
	b.WriteString(tabs)
	b.WriteString("\n\n")

	// Main content
	switch m.viewMode {
	case ViewList:
		b.WriteString(m.renderList())
	case ViewDetail:
		b.WriteString(m.renderDetail())
	case ViewHelp:
		b.WriteString(m.renderHelp())
	case ViewProfileSelect:
		b.WriteString(m.renderProfileSelect())
	case ViewRegionSelect:
		b.WriteString(m.renderRegionSelect())
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m Model) renderHeader() string {
	title := "☁️  AWS TUI"
	profile := fmt.Sprintf("Profile: %s", m.client.Profile)
	region := fmt.Sprintf("Region: %s", m.client.Region)

	left := headerStyle.Render(title)
	right := headerStyle.Render(fmt.Sprintf("%s | %s", profile, region))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderTabs() string {
	resources := []ResourceType{ResourceEC2, ResourceEKS, ResourceECS, ResourceS3, ResourceRDS, ResourceLambda}
	var tabs []string

	for _, r := range resources {
		style := inactiveTabStyle
		if r == m.resourceType {
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(r.String()))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m Model) renderList() string {
	if m.loading {
		return boxStyle.Render(fmt.Sprintf("%s Loading %s...", m.spinner.View(), m.resourceType.String()))
	}

	return m.list.View()
}

func (m Model) renderDetail() string {
	title := titleStyle.Render(fmt.Sprintf("📋 %s Details", m.resourceType.String()))
	content := boxStyle.Width(m.width - 4).Render(m.detail.View())
	help := helpStyle.Render("↑/↓: scroll • esc: back")

	return fmt.Sprintf("%s\n%s\n%s", title, content, help)
}

func (m Model) renderHelp() string {
	title := titleStyle.Render("⌨️  Keyboard Shortcuts")

	helpContent := `
Navigation
  ↑/k, ↓/j     Navigate list
  ←/h, →/l     Previous/Next resource type
  Tab/S-Tab    Switch resource type
  Enter        View details
  Esc          Go back

Actions
  r            Refresh current view
  p            Switch AWS profile
  R            Switch AWS region
  c            Copy resource ID
  /            Search/Filter

General
  ?            Toggle help
  q            Quit
`

	content := boxStyle.Width(m.width - 4).Render(helpContent)
	help := helpStyle.Render("Press ? or Esc to close")

	return fmt.Sprintf("%s\n%s\n%s", title, content, help)
}

func (m Model) renderProfileSelect() string {
	title := titleStyle.Render("🔐 Select AWS Profile")

	var items []string
	for i, profile := range m.profiles {
		style := normalStyle
		prefix := "  "
		if i == m.selectedIndex {
			style = selectedStyle
			prefix = "▶ "
		}
		items = append(items, style.Render(prefix+profile))
	}

	content := boxStyle.Width(40).Render(strings.Join(items, "\n"))
	help := helpStyle.Render("↑/↓: navigate • Enter: select • Esc: cancel")

	return fmt.Sprintf("%s\n%s\n%s", title, content, help)
}

func (m Model) renderRegionSelect() string {
	title := titleStyle.Render("🌍 Select AWS Region")

	var items []string
	for i, region := range m.regions {
		style := normalStyle
		prefix := "  "
		if i == m.selectedIndex {
			style = selectedStyle
			prefix = "▶ "
		}
		items = append(items, style.Render(prefix+region))
	}

	content := boxStyle.Width(40).Render(strings.Join(items, "\n"))
	help := helpStyle.Render("↑/↓: navigate • Enter: select • Esc: cancel")

	return fmt.Sprintf("%s\n%s\n%s", title, content, help)
}

func (m Model) renderStatusBar() string {
	status := m.statusMsg
	if m.err != nil {
		status = lipgloss.NewStyle().Foreground(errorColor).Render(fmt.Sprintf("Error: %v", m.err))
	}

	helpText := m.help.View(m.keys)

	statusBar := statusBarStyle.Width(m.width).Render(status)
	return fmt.Sprintf("%s\n%s", statusBar, helpStyle.Render(helpText))
}
