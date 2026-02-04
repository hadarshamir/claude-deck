package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hadar/claude-deck/internal/session"
)

// PreviewLoadedMsg is sent when preview data is loaded
type PreviewLoadedMsg struct {
	SessionID string
	Messages  []session.PreviewMessage
	Err       error
}

// PreviewModel manages the preview pane
type PreviewModel struct {
	session       *session.Session
	messages      []session.PreviewMessage
	width         int
	height        int
	offset        int
	loading       bool
	err           error
	lastSessionID string
	searchSnippet string // If set, show "Found" section with this snippet
}

// NewPreviewModel creates a new preview model
func NewPreviewModel() *PreviewModel {
	return &PreviewModel{}
}

// SetSession updates the preview for a new session
// Returns a command to load the preview async
func (m *PreviewModel) SetSession(s *session.Session) tea.Cmd {
	if s == nil {
		m.session = nil
		m.messages = nil
		m.err = nil
		m.lastSessionID = ""
		m.loading = false
		return nil
	}

	// Skip if same session
	if m.lastSessionID == s.ID {
		return nil
	}

	m.session = s
	m.lastSessionID = s.ID
	m.offset = 0
	m.loading = true
	m.messages = nil

	// Return command to load async
	sessionID := s.ID
	sess := s
	return func() tea.Msg {
		messages, err := session.GetPreview(sess)

		// Filter out tool-related messages
		var filtered []session.PreviewMessage
		for _, msg := range messages {
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			if strings.HasPrefix(msg.Content, "[Tool:") {
				continue
			}
			filtered = append(filtered, msg)
		}

		return PreviewLoadedMsg{
			SessionID: sessionID,
			Messages:  filtered,
			Err:       err,
		}
	}
}

// SetSessionDirect updates the session pointer without reloading messages
// Used when session data is updated (e.g., git info) but messages haven't changed
func (m *PreviewModel) SetSessionDirect(s *session.Session) {
	if s == nil {
		return
	}
	// Update pointer if same session (by ID)
	if m.lastSessionID == s.ID {
		m.session = s
	}
}

// HandleLoaded processes the loaded preview data
func (m *PreviewModel) HandleLoaded(msg PreviewLoadedMsg) {
	// Ignore if session changed while loading
	if m.lastSessionID != msg.SessionID {
		return
	}

	m.loading = false
	m.messages = msg.Messages
	m.err = msg.Err
}

// Refresh reloads the preview content
func (m *PreviewModel) Refresh() tea.Cmd {
	if m.session == nil {
		return nil
	}
	m.loading = true
	sess := m.session
	sessionID := m.lastSessionID
	return func() tea.Msg {
		messages, err := session.GetPreview(sess)
		var filtered []session.PreviewMessage
		for _, msg := range messages {
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			if strings.HasPrefix(msg.Content, "[Tool:") {
				continue
			}
			filtered = append(filtered, msg)
		}
		return PreviewLoadedMsg{
			SessionID: sessionID,
			Messages:  filtered,
			Err:       err,
		}
	}
}

// SetSize updates the preview dimensions
func (m *PreviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetSearchSnippet sets the search snippet to display in "Found" section
func (m *PreviewModel) SetSearchSnippet(snippet string) {
	m.searchSnippet = snippet
}

// View renders the preview pane with fixed header and scrollable messages
func (m *PreviewModel) View() string {
	height := m.height
	if height <= 0 {
		height = 20
	}

	// Handle special states
	if m.session == nil {
		return m.padToHeight(helpStyle.Render("Select a session to preview"), height)
	} else if m.loading {
		return m.padToHeight(helpStyle.Render("Loading..."), height)
	} else if m.err != nil {
		return m.padToHeight(helpStyle.Render(fmt.Sprintf("Error: %v", m.err)), height)
	}

	// Render header (always visible)
	header := m.renderHeader()
	headerLines := strings.Split(header, "\n")

	// Add separator
	sepWidth := m.width - 2
	if sepWidth < 10 {
		sepWidth = 10
	}
	separator := helpStyle.Render(strings.Repeat("─", sepWidth))
	headerLines = append(headerLines, "", separator, "")

	// Calculate remaining height for messages
	headerHeight := len(headerLines)
	messagesHeight := height - headerHeight
	if messagesHeight < 3 {
		messagesHeight = 3
	}

	// Render messages section
	messagesContent := m.renderMessages()
	messageLines := strings.Split(messagesContent, "\n")

	// Show TAIL of messages (most recent)
	startLine := 0
	if len(messageLines) > messagesHeight {
		startLine = len(messageLines) - messagesHeight
	}

	// Build result: header + messages tail
	result := make([]string, height)
	lineIdx := 0

	// Add header lines
	for i := 0; i < headerHeight && lineIdx < height; i++ {
		line := headerLines[i]
		if lipgloss.Width(line) > m.width {
			line = truncateString(line, m.width-3) + "..."
		}
		result[lineIdx] = m.padLine(line)
		lineIdx++
	}

	// Add message lines (tail)
	for i := 0; i < messagesHeight && lineIdx < height; i++ {
		msgIdx := startLine + i
		if msgIdx < len(messageLines) {
			line := messageLines[msgIdx]
			if lipgloss.Width(line) > m.width {
				line = truncateString(line, m.width-3) + "..."
			}
			result[lineIdx] = m.padLine(line)
		} else {
			result[lineIdx] = m.padLine("")
		}
		lineIdx++
	}

	return strings.Join(result, "\n")
}

// padLine pads a line to the full width
func (m *PreviewModel) padLine(line string) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth < m.width {
		return line + strings.Repeat(" ", m.width-lineWidth)
	}
	return line
}

// padToHeight pads content to fill the height
func (m *PreviewModel) padToHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	result := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			result[i] = m.padLine(lines[i])
		} else {
			result[i] = m.padLine("")
		}
	}
	return strings.Join(result, "\n")
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// renderMessages renders just the messages section (for scrolling)
func (m *PreviewModel) renderMessages() string {
	var lines []string

	// Show "Found" section if we have a search snippet
	if m.searchSnippet != "" {
		lines = append(lines, previewTitleStyle.Render("Found:"))
		wrapped := wrapText(m.searchSnippet, m.width-4)
		lines = append(lines, userMessageStyle.Render(wrapped))
		lines = append(lines, "") // blank line
		lines = append(lines, previewTitleStyle.Render("Recent:"))
	}

	if len(m.messages) == 0 {
		lines = append(lines, helpStyle.Render("No messages yet"))
	} else {
		for i, msg := range m.messages {
			if i > 0 {
				lines = append(lines, "") // single blank line between messages
			}
			lines = append(lines, m.renderMessage(msg))
		}
	}

	return strings.Join(lines, "\n")
}

func (m *PreviewModel) renderHeader() string {
	var lines []string

	// Title + status on same line
	status := m.session.Status
	titleLine := previewTitleStyle.Render(m.session.Name) + "  " +
		StatusStyle(status.String()).Render(status.Symbol()) + " " +
		helpStyle.Render(status.String())
	lines = append(lines, titleLine)

	// Directory
	lines = append(lines, previewMetaStyle.Render("Directory: ")+helpStyle.Render(m.session.ProjectPath))

	// Git branch (from Claude's JSONL - branch at time of session)
	if m.session.GitBranch != "" {
		lines = append(lines, previewMetaStyle.Render("Branch: ")+helpStyle.Render(m.session.GitBranch))
	}

	// Timestamps
	if !m.session.CreatedAt.IsZero() {
		lines = append(lines, previewMetaStyle.Render("Created: ")+helpStyle.Render(m.session.CreatedAt.Format("Jan 2 15:04")))
	}
	if !m.session.LastAccessedAt.IsZero() {
		lines = append(lines, previewMetaStyle.Render("Modified: ")+helpStyle.Render(m.session.LastAccessedAt.Format("Jan 2 15:04")))
	}

	return strings.Join(lines, "\n")
}

func (m *PreviewModel) renderMessage(msg session.PreviewMessage) string {
	var roleName string
	var roleStyle, contentStyle lipgloss.Style

	if msg.Role == "user" {
		roleName = "You"
		roleStyle = userRoleStyle
		contentStyle = userMessageStyle
	} else {
		roleName = "Claude"
		roleStyle = assistantRoleStyle
		contentStyle = assistantMessageStyle
	}

	var timeStr string
	if !msg.Timestamp.IsZero() {
		timeStr = " " + msg.Timestamp.Format("15:04")
	}

	header := roleStyle.Render(roleName) + previewMetaStyle.Render(timeStr)

	// Wrap text to fit width
	content := wrapText(msg.Content, m.width-2)

	return header + "\n" + contentStyle.Render(content)
}

// wrapText wraps text to fit within maxWidth
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		var currentLine string
		for _, word := range words {
			if currentLine == "" {
				currentLine = word
			} else if len(currentLine)+1+len(word) <= maxWidth {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return strings.Join(lines, "\n")
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2")
	}
}
