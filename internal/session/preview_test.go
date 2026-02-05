package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMessageContentGetContent(t *testing.T) {
	tests := []struct {
		name       string
		rawContent string
		want       string
	}{
		{
			name:       "string content (user message)",
			rawContent: `"Hello, how are you?"`,
			want:       "Hello, how are you?",
		},
		{
			name:       "array content with text",
			rawContent: `[{"type":"text","text":"This is the response."}]`,
			want:       "This is the response.",
		},
		{
			name:       "array content with multiple text parts",
			rawContent: `[{"type":"text","text":"First part."},{"type":"text","text":"Second part."}]`,
			want:       "First part.\nSecond part.",
		},
		{
			name:       "array content with tool_use (should be skipped)",
			rawContent: `[{"type":"text","text":"Let me help."},{"type":"tool_use","name":"Bash"}]`,
			want:       "Let me help.",
		},
		{
			name:       "empty content",
			rawContent: ``,
			want:       "",
		},
		{
			name:       "null content",
			rawContent: `null`,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MessageContent{
				RawContent: json.RawMessage(tt.rawContent),
			}
			if got := m.GetContent(); got != tt.want {
				t.Errorf("GetContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractContent(t *testing.T) {
	tests := []struct {
		name  string
		parts []ContentPart
		want  string
	}{
		{
			name:  "single text part",
			parts: []ContentPart{{Type: "text", Text: "Hello"}},
			want:  "Hello",
		},
		{
			name: "multiple text parts",
			parts: []ContentPart{
				{Type: "text", Text: "First"},
				{Type: "text", Text: "Second"},
			},
			want: "First\nSecond",
		},
		{
			name: "text and tool_use mixed",
			parts: []ContentPart{
				{Type: "text", Text: "Before tool"},
				{Type: "tool_use", Name: "Bash"},
				{Type: "text", Text: "After tool"},
			},
			want: "Before tool\nAfter tool",
		},
		{
			name: "tool_result should be skipped",
			parts: []ContentPart{
				{Type: "text", Text: "Response"},
				{Type: "tool_result"},
			},
			want: "Response",
		},
		{
			name:  "empty parts",
			parts: []ContentPart{},
			want:  "",
		},
		{
			name: "empty text field",
			parts: []ContentPart{
				{Type: "text", Text: ""},
				{Type: "text", Text: "Valid"},
			},
			want: "Valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractContent(tt.parts); got != tt.want {
				t.Errorf("extractContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractContentTruncation(t *testing.T) {
	// Create content longer than MaxContentLength
	longText := ""
	for i := 0; i < MaxContentLength+100; i++ {
		longText += "x"
	}

	parts := []ContentPart{{Type: "text", Text: longText}}
	got := extractContent(parts)

	if len(got) > MaxContentLength+3 { // +3 for "..."
		t.Errorf("extractContent() length = %d, want <= %d", len(got), MaxContentLength+3)
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("extractContent() should end with '...', got %q", got[len(got)-10:])
	}
}

func TestTruncateSnippet(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "long string truncated",
			input:  "hello world this is long",
			maxLen: 10,
			want:   "hello worl...",
		},
		{
			name:   "newlines converted to spaces",
			input:  "line1\nline2",
			maxLen: 20,
			want:   "line1 line2",
		},
		{
			name:   "tabs converted to spaces",
			input:  "col1\tcol2",
			maxLen: 20,
			want:   "col1 col2",
		},
		{
			name:   "exact length",
			input:  "12345",
			maxLen: 5,
			want:   "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateSnippet(tt.input, tt.maxLen); got != tt.want {
				t.Errorf("truncateSnippet(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestExtractSnippetWithContext(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		query string
		want  string
	}{
		{
			name:  "query at start",
			line:  "hello world this is a test",
			query: "hello",
			want:  "hello world this is a test",
		},
		{
			name:  "query in middle",
			line:  "the quick brown fox jumps over the lazy dog and then some more text",
			query: "fox",
			want:  "...the quick brown fox jumps over the lazy dog and then some more text",
		},
		{
			name:  "query not found",
			line:  "hello world",
			query: "xyz",
			want:  "",
		},
		{
			name:  "case insensitive",
			line:  "Hello World",
			query: "hello",
			want:  "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSnippetWithContext(tt.line, tt.query)
			// Just check it's non-empty when query exists
			if tt.want == "" && got != "" {
				t.Errorf("extractSnippetWithContext() = %q, want empty", got)
			}
			if tt.want != "" && got == "" {
				t.Errorf("extractSnippetWithContext() = empty, want non-empty")
			}
		})
	}
}

func TestGetPreview(t *testing.T) {
	// Create a temp JSONL file
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	// Write test JSONL content
	content := `{"type":"message","message":{"role":"user","content":"Hello"},"timestamp":"2024-01-01T10:00:00Z"}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Hi there!"}]},"timestamp":"2024-01-01T10:00:01Z"}
{"type":"message","message":{"role":"user","content":"How are you?"},"timestamp":"2024-01-01T10:00:02Z"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	messages, err := GetPreview(s)
	if err != nil {
		t.Fatalf("GetPreview() error = %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("GetPreview() returned %d messages, want 3", len(messages))
	}

	// Check first message
	if messages[0].Role != "user" {
		t.Errorf("messages[0].Role = %q, want %q", messages[0].Role, "user")
	}
	if messages[0].Content != "Hello" {
		t.Errorf("messages[0].Content = %q, want %q", messages[0].Content, "Hello")
	}

	// Check second message
	if messages[1].Role != "assistant" {
		t.Errorf("messages[1].Role = %q, want %q", messages[1].Role, "assistant")
	}
	if messages[1].Content != "Hi there!" {
		t.Errorf("messages[1].Content = %q, want %q", messages[1].Content, "Hi there!")
	}
}

func TestGetPreviewEmptyPath(t *testing.T) {
	s := &Session{JSONLPath: ""}
	messages, err := GetPreview(s)
	if err != nil {
		t.Fatalf("GetPreview() error = %v", err)
	}
	if messages != nil {
		t.Errorf("GetPreview() = %v, want nil", messages)
	}
}

func TestGetSessionTitle(t *testing.T) {
	// Create a temp JSONL file
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	// Write test JSONL content with first user message
	content := `{"type":"summary","sessionId":"abc123"}
{"type":"message","message":{"role":"user","content":"Help me with my Go code"},"timestamp":"2024-01-01T10:00:00Z"}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Sure!"}]}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	title := GetSessionTitle(s)

	if title != "Help me with my Go code" {
		t.Errorf("GetSessionTitle() = %q, want %q", title, "Help me with my Go code")
	}
}

func TestGetSessionTitleTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	// Create a message longer than 60 chars
	longMessage := "This is a very long message that should be truncated because it exceeds the maximum length"
	content := `{"type":"message","message":{"role":"user","content":"` + longMessage + `"}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	title := GetSessionTitle(s)

	if len(title) > 60 {
		t.Errorf("GetSessionTitle() length = %d, want <= 60", len(title))
	}
	if title[len(title)-3:] != "..." {
		t.Errorf("GetSessionTitle() should end with '...', got %q", title)
	}
}

func TestGetSessionTitleEmptyPath(t *testing.T) {
	s := &Session{JSONLPath: ""}
	title := GetSessionTitle(s)
	if title != "" {
		t.Errorf("expected empty title for empty path, got %q", title)
	}
}

func TestGetSessionTitleNonExistent(t *testing.T) {
	s := &Session{JSONLPath: "/nonexistent/file.jsonl"}
	title := GetSessionTitle(s)
	if title != "" {
		t.Errorf("expected empty title for non-existent file, got %q", title)
	}
}

func TestGetSessionTitleMultiline(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	// Message with newlines
	content := `{"type":"message","message":{"role":"user","content":"First line\nSecond line\nThird line"}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	title := GetSessionTitle(s)

	if title != "First line" {
		t.Errorf("expected 'First line', got %q", title)
	}
}

func TestGetSessionStats(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(jsonlPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	_, lastActivity, err := GetSessionStats(s)

	if err != nil {
		t.Fatalf("GetSessionStats failed: %v", err)
	}
	if lastActivity.IsZero() {
		t.Error("lastActivity should not be zero")
	}
}

func TestGetSessionStatsEmptyPath(t *testing.T) {
	s := &Session{JSONLPath: ""}
	count, lastActivity, err := GetSessionStats(s)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
	if !lastActivity.IsZero() {
		t.Error("lastActivity should be zero")
	}
}

func TestSearchContent(t *testing.T) {
	t.Run("empty query returns nil", func(t *testing.T) {
		sessions := []*Session{{JSONLPath: "/some/path"}}
		results := SearchContent(sessions, "")
		if results != nil {
			t.Error("expected nil for empty query")
		}
	})

	t.Run("empty sessions returns nil", func(t *testing.T) {
		results := SearchContent([]*Session{}, "test")
		if results != nil {
			t.Error("expected nil for empty sessions")
		}
	})

	t.Run("nil sessions returns nil", func(t *testing.T) {
		results := SearchContent(nil, "test")
		if results != nil {
			t.Error("expected nil for nil sessions")
		}
	})

	t.Run("skips sessions without JSONLPath", func(t *testing.T) {
		sessions := []*Session{{JSONLPath: ""}}
		results := SearchContent(sessions, "test")
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestGetPreviewLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "large.jsonl")

	// Create a file larger than MaxBytesToRead
	var content string
	// Add some filler content
	for i := 0; i < 600; i++ {
		content += `{"type":"filler","data":"` + string(make([]byte, 1000)) + `"}` + "\n"
	}
	// Add actual messages at the end
	content += `{"type":"message","message":{"role":"user","content":"final message"}}` + "\n"

	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	messages, err := GetPreview(s)

	if err != nil {
		t.Fatalf("GetPreview failed: %v", err)
	}
	// Should still find the message at the end
	if len(messages) == 0 {
		t.Error("expected to find messages")
	}
}

func TestGetPreviewMalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "malformed.jsonl")

	content := `not json at all
{"type":"message","message":{"role":"user","content":"valid message"}}
{incomplete json
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"response"}]}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	messages, err := GetPreview(s)

	if err != nil {
		t.Fatalf("GetPreview failed: %v", err)
	}
	// Should skip malformed lines and still get valid messages
	if len(messages) != 2 {
		t.Errorf("expected 2 valid messages, got %d", len(messages))
	}
}

func TestGetPreviewEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "empty_content.jsonl")

	content := `{"type":"message","message":{"role":"user","content":""}}
{"type":"message","message":{"role":"user","content":"has content"}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	messages, err := GetPreview(s)

	if err != nil {
		t.Fatalf("GetPreview failed: %v", err)
	}
	// Should skip message with empty content
	if len(messages) != 1 {
		t.Errorf("expected 1 message (skipping empty), got %d", len(messages))
	}
}

func TestGetPreviewMaxMessages(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "many.jsonl")

	// Create more than MaxPreviewMessages
	var content string
	for i := 0; i < MaxPreviewMessages+10; i++ {
		content += `{"type":"message","message":{"role":"user","content":"message ` + string(rune('A'+i%26)) + `"}}` + "\n"
	}

	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	messages, err := GetPreview(s)

	if err != nil {
		t.Fatalf("GetPreview failed: %v", err)
	}
	if len(messages) > MaxPreviewMessages {
		t.Errorf("expected at most %d messages, got %d", MaxPreviewMessages, len(messages))
	}
}

func TestMessageContentGetContentNil(t *testing.T) {
	m := &MessageContent{RawContent: nil}
	if m.GetContent() != "" {
		t.Error("expected empty string for nil content")
	}
}

func TestExtractContentOnlyToolUse(t *testing.T) {
	parts := []ContentPart{
		{Type: "tool_use", Name: "Bash"},
		{Type: "tool_result"},
	}
	got := extractContent(parts)
	if got != "" {
		t.Errorf("expected empty string when only tool parts, got %q", got)
	}
}

func TestSearchContentWithMatches(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "session.jsonl")

	// Create file with searchable content
	content := `{"type":"message","message":{"role":"user","content":"find the special keyword here"}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"found it"}]}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sessions := []*Session{{JSONLPath: jsonlPath}}
	results := SearchContent(sessions, "special keyword")

	if len(results) == 0 {
		t.Error("expected to find match")
	}
}

func TestSearchContentNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "session.jsonl")

	content := `{"type":"message","message":{"role":"user","content":"nothing interesting"}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sessions := []*Session{{JSONLPath: jsonlPath}}
	results := SearchContent(sessions, "xyznonexistent123")

	if len(results) != 0 {
		t.Errorf("expected no matches, got %d", len(results))
	}
}

func TestGetPreviewWithTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	content := `{"type":"message","message":{"role":"user","content":"hello"},"timestamp":"2024-03-15T10:30:00Z"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	messages, err := GetPreview(s)
	if err != nil {
		t.Fatalf("GetPreview failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Timestamp.IsZero() {
		t.Error("Timestamp should be parsed")
	}
}

func TestGetPreviewNoMessage(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	// Entry without message field
	content := `{"type":"summary","sessionId":"123"}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Session{JSONLPath: jsonlPath}
	messages, err := GetPreview(s)
	if err != nil {
		t.Fatalf("GetPreview failed: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestExtractSnippetWithContextLongLine(t *testing.T) {
	// Test with query far into the line (needs context trimming)
	longLine := "start " + string(make([]byte, 200)) + "FINDME" + string(make([]byte, 200)) + " end"
	got := extractSnippetWithContext(longLine, "findme")

	if got == "" {
		t.Error("expected non-empty snippet")
	}
	// Should be truncated
	if len(got) > 200 {
		t.Errorf("snippet too long: %d chars", len(got))
	}
}
