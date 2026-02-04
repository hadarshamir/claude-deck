package session

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	MaxPreviewMessages = 15
	MaxContentLength   = 400
	MaxBytesToRead     = 500 * 1024 // Read last 500KB of file for long conversations
)

// JSONLEntry represents a single entry in Claude's conversation JSONL
type JSONLEntry struct {
	Type        string          `json:"type"`
	Message     *MessageContent `json:"message,omitempty"`
	Timestamp   string          `json:"timestamp,omitempty"`
	SessionID   string          `json:"sessionId,omitempty"`
	ParentUUID  string          `json:"parentUuid,omitempty"`
	IsSidechain bool            `json:"isSidechain,omitempty"`
}

// MessageContent represents the message structure in JSONL
type MessageContent struct {
	Role       string          `json:"role"`
	RawContent json.RawMessage `json:"content,omitempty"`
	Model      string          `json:"model,omitempty"`
}

// GetContent extracts text content from the message
// Handles both string content (user) and array content (assistant)
func (m *MessageContent) GetContent() string {
	if len(m.RawContent) == 0 {
		return ""
	}

	// Try as string first (user messages)
	var strContent string
	if err := json.Unmarshal(m.RawContent, &strContent); err == nil {
		return strContent
	}

	// Try as array (assistant messages)
	var parts []ContentPart
	if err := json.Unmarshal(m.RawContent, &parts); err == nil {
		return extractContent(parts)
	}

	return ""
}

// ContentPart represents a part of the message content
type ContentPart struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

// GetPreview reads the last N messages from a session's JSONL file
// Only reads the tail of the file for performance
func GetPreview(s *Session) ([]PreviewMessage, error) {
	if s.JSONLPath == "" {
		return nil, nil
	}

	file, err := os.Open(s.JSONLPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Only read the last MaxBytesToRead bytes
	var data []byte
	fileSize := stat.Size()
	if fileSize > MaxBytesToRead {
		_, err = file.Seek(-MaxBytesToRead, io.SeekEnd)
		if err != nil {
			return nil, err
		}
		data, err = io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		// Skip to first complete line (after first newline)
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
			data = data[idx+1:]
		}
	} else {
		data, err = io.ReadAll(file)
		if err != nil {
			return nil, err
		}
	}

	// Parse JSONL lines
	var messages []PreviewMessage
	lines := bytes.Split(data, []byte("\n"))

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var entry JSONLEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // Skip malformed lines
		}

		if entry.Message == nil {
			continue
		}

		msg := PreviewMessage{
			Role: entry.Message.Role,
		}

		// Parse timestamp if available
		if entry.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
				msg.Timestamp = t
			}
		}

		// Extract text content (handles both string and array formats)
		msg.Content = entry.Message.GetContent()
		if msg.Content == "" {
			continue
		}

		messages = append(messages, msg)
	}

	// Return last N messages
	if len(messages) > MaxPreviewMessages {
		messages = messages[len(messages)-MaxPreviewMessages:]
	}

	return messages, nil
}

// extractContent extracts readable text from message content
func extractContent(parts []ContentPart) string {
	var texts []string

	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
		case "tool_use":
			// Skip tool calls entirely
			continue
		case "tool_result":
			// Skip tool results
			continue
		}
	}

	content := strings.Join(texts, "\n")

	// Truncate if too long
	if len(content) > MaxContentLength {
		content = content[:MaxContentLength] + "..."
	}

	return content
}

// GetSessionStats returns statistics about a session
func GetSessionStats(s *Session) (messageCount int, lastActivity time.Time, err error) {
	if s.JSONLPath == "" {
		return 0, time.Time{}, nil
	}

	// Just get file info for quick stats
	info, err := os.Stat(s.JSONLPath)
	if err != nil {
		return 0, time.Time{}, err
	}

	return 0, info.ModTime(), nil
}

// SearchResult represents a content search match
type SearchResult struct {
	Session *Session
	Snippet string // Context around the match
	Role    string // user or assistant
}

// SearchContent searches through session content for a query string
// Only searches user/assistant text, not tool calls or bash commands
func SearchContent(sessions []*Session, query string) []SearchResult {
	if query == "" || len(sessions) == 0 {
		return nil
	}

	var results []SearchResult

	for _, s := range sessions {
		if s.JSONLPath == "" {
			continue
		}

		snippet := searchConversationText(s.JSONLPath, query)
		if snippet != "" {
			results = append(results, SearchResult{
				Session: s,
				Snippet: snippet,
				Role:    "user",
			})
		}
	}

	return results
}

// searchConversationText searches only in user/assistant text content (not tool calls)
func searchConversationText(path, query string) string {
	// Use rg/grep to find lines with text content containing the query
	// Pattern: match lines that have "text":" (text content) AND contain the query
	// This filters out tool_use, tool_result, bash commands etc.

	var cmd *exec.Cmd
	queryLower := strings.ToLower(query)

	if _, err := exec.LookPath("rg"); err == nil {
		// rg: search for lines containing both "text":" and the query
		cmd = exec.Command("rg", "-i", "-m1", query, path)
	} else {
		cmd = exec.Command("grep", "-i", "-m1", query, path)
	}

	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return ""
	}

	// Parse the matching line to check if it's text content (not tool/bash)
	line := string(output)

	// Skip if this is a tool call or bash command
	if strings.Contains(line, `"type":"tool_use"`) ||
		strings.Contains(line, `"type":"tool_result"`) ||
		strings.Contains(line, `"name":"Bash"`) ||
		strings.Contains(line, `"name":"Read"`) ||
		strings.Contains(line, `"name":"Write"`) ||
		strings.Contains(line, `"name":"Edit"`) ||
		strings.Contains(line, `"name":"Grep"`) ||
		strings.Contains(line, `"name":"Glob"`) {
		// Try to find a text-only match
		return searchTextOnly(path, query)
	}

	// Extract snippet with context
	return extractSnippetWithContext(line, queryLower)
}

// searchTextOnly searches specifically in "text":" fields
func searchTextOnly(path, query string) string {
	// Search for query appearing after "text":" pattern
	var cmd *exec.Cmd
	pattern := `"text":"[^"]*` + query

	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.Command("rg", "-i", "-m1", "-o", pattern+`[^"]*`, path)
	} else {
		cmd = exec.Command("grep", "-i", "-m1", "-oE", pattern+`[^"]*`, path)
	}

	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return ""
	}

	snippet := string(output)
	// Clean up
	snippet = strings.TrimPrefix(snippet, `"text":"`)
	snippet = strings.ReplaceAll(snippet, "\\n", " ")
	snippet = strings.ReplaceAll(snippet, "\\t", " ")

	return truncateSnippet(snippet, 150)
}

// extractSnippetWithContext extracts the query match with surrounding context
func extractSnippetWithContext(line, query string) string {
	lineLower := strings.ToLower(line)
	idx := strings.Index(lineLower, query)
	if idx < 0 {
		return ""
	}

	// Get 80 chars before and after
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 80
	if end > len(line) {
		end = len(line)
	}

	snippet := line[start:end]

	// Clean up JSON artifacts
	snippet = strings.ReplaceAll(snippet, "\\n", " ")
	snippet = strings.ReplaceAll(snippet, "\\t", " ")
	snippet = strings.ReplaceAll(snippet, "\"", "")
	snippet = strings.ReplaceAll(snippet, "\\", "")

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(line) {
		snippet = snippet + "..."
	}

	return truncateSnippet(snippet, 150)
}


// truncateSnippet truncates content to maxLen chars
func truncateSnippet(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// GetSessionTitle extracts a title from the first user message
// Reads only the first 20KB of the file for speed
func GetSessionTitle(s *Session) string {
	if s.JSONLPath == "" {
		return ""
	}

	file, err := os.Open(s.JSONLPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Read first 20KB
	data := make([]byte, 20*1024)
	n, err := file.Read(data)
	if err != nil && n == 0 {
		return ""
	}
	data = data[:n]

	// Find first user message
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var entry JSONLEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Message != nil && entry.Message.Role == "user" {
			content := entry.Message.GetContent()
			if content != "" {
				// Truncate to first line, max 60 chars
				if idx := strings.Index(content, "\n"); idx > 0 {
					content = content[:idx]
				}
				if len(content) > 60 {
					content = content[:57] + "..."
				}
				return content
			}
		}
	}

	return ""
}
