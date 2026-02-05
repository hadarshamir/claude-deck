package ui

import (
	"testing"
	"time"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"hello", 3, "hel"},  // maxLen <= 3: no ellipsis
		{"hello", 2, "he"},
		{"hello", 1, "h"},
		{"hello", 0, ""},
		{"", 5, ""},
		{"日本語テスト", 5, "日本..."},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello wo"}, // no ellipsis, just truncate
		{"hello", 3, "hel"},
		{"", 5, ""},
		{"hello", 0, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestPadStr(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello     "},
		{"hello", 5, "hello"},
		{"hello", 3, "h.."},   // truncate with ".." suffix
		{"hello", 2, "he"},    // width <= 2: just truncate
		{"", 5, "     "},
		{"hello", 0, ""},
	}

	for _, tt := range tests {
		result := padStr(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("padStr(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
		}
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()

	tests := []struct {
		input    time.Time
		expected string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-1 * time.Minute), "1 minute ago"},
		{now.Add(-5 * time.Minute), "5 minutes ago"},
		{now.Add(-1 * time.Hour), "1 hour ago"},
		{now.Add(-2 * time.Hour), "2 hours ago"},
		{now.Add(-36 * time.Hour), "yesterday"},
		{now.Add(-72 * time.Hour), "3 days ago"},
	}

	for _, tt := range tests {
		result := formatTimeAgo(tt.input)
		if result != tt.expected {
			t.Errorf("formatTimeAgo(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello\nworld"},
		{"a b c d", 3, "a b\nc d"},
		{"", 10, ""},
		{"hello", 0, "hello"},
	}

	for _, tt := range tests {
		result := wrapText(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("wrapText(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
		}
	}
}

func TestCommonPrefix(t *testing.T) {
	tests := []struct {
		a, b     string
		expected string
	}{
		{"hello", "hello", "hello"},
		{"hello", "help", "hel"},
		{"abc", "xyz", ""},
		{"", "hello", ""},
		{"hello", "", ""},
		{"/Users/test/project", "/Users/test/other", "/Users/test/"},
	}

	for _, tt := range tests {
		result := commonPrefix(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("commonPrefix(%q, %q) = %q, want %q", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestStringSliceEqual(t *testing.T) {
	tests := []struct {
		a, b     []string
		expected bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a", "b"}, []string{"a", "c"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{nil, nil, true},
		{[]string{}, []string{}, true},
		{nil, []string{}, true},
	}

	for _, tt := range tests {
		result := stringSliceEqual(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("stringSliceEqual(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestIsWordChar(t *testing.T) {
	tests := []struct {
		input    byte
		expected bool
	}{
		{'a', true},
		{'Z', true},
		{'5', true},
		{'_', true},
		{' ', false},
		{'-', false},
		{'.', false},
	}

	for _, tt := range tests {
		result := isWordChar(tt.input)
		if result != tt.expected {
			t.Errorf("isWordChar(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestFindWordBoundaryLeft(t *testing.T) {
	tests := []struct {
		s        string
		pos      int
		expected int
	}{
		{"hello world", 6, 0},
		{"hello world", 11, 6},
		{"hello", 5, 0},
		{"hello", 0, 0},
		{"a b c", 4, 2},
	}

	for _, tt := range tests {
		result := findWordBoundaryLeft(tt.s, tt.pos)
		if result != tt.expected {
			t.Errorf("findWordBoundaryLeft(%q, %d) = %d, want %d", tt.s, tt.pos, result, tt.expected)
		}
	}
}

func TestFindWordBoundaryRight(t *testing.T) {
	tests := []struct {
		s        string
		pos      int
		expected int
	}{
		{"hello world", 0, 6},  // skip "hello" + space -> start of "world"
		{"hello world", 6, 11}, // skip "world" -> end
		{"hello", 0, 5},        // skip "hello" -> end
		{"hello", 5, 5},        // already at end
		{"a b c", 0, 2},        // skip "a" + space -> start of "b"
	}

	for _, tt := range tests {
		result := findWordBoundaryRight(tt.s, tt.pos)
		if result != tt.expected {
			t.Errorf("findWordBoundaryRight(%q, %d) = %d, want %d", tt.s, tt.pos, result, tt.expected)
		}
	}
}

func TestHandleTextInputKey(t *testing.T) {
	tests := []struct {
		text     string
		cursor   int
		key      string
		wantText string
		wantPos  int
		wantOK   bool
	}{
		// Character input
		{"hello", 5, "a", "helloa", 6, true},
		{"hello", 0, "x", "xhello", 1, true},
		{"hello", 2, "x", "hexllo", 3, true},

		// Backspace
		{"hello", 5, "backspace", "hell", 4, true},
		{"hello", 0, "backspace", "hello", 0, false}, // can't backspace at start
		{"hello", 2, "backspace", "hllo", 1, true},

		// Delete
		{"hello", 0, "delete", "ello", 0, true},
		{"hello", 5, "delete", "hello", 5, false}, // can't delete at end
		{"hello", 2, "delete", "helo", 2, true},

		// Movement
		{"hello", 5, "left", "hello", 4, true},
		{"hello", 0, "left", "hello", 0, false},  // can't move left at start
		{"hello", 0, "right", "hello", 1, true},
		{"hello", 5, "right", "hello", 5, false}, // can't move right at end
		{"hello", 2, "home", "hello", 0, true},
		{"hello", 2, "ctrl+a", "hello", 0, true},
		{"hello", 2, "end", "hello", 5, true},
		{"hello", 2, "ctrl+e", "hello", 5, true},

		// Word navigation
		{"hello world", 0, "alt+right", "hello world", 6, true},
		{"hello world", 6, "alt+left", "hello world", 0, true},

		// Line editing
		{"hello", 2, "ctrl+k", "he", 2, true},     // delete to end
		{"hello", 2, "ctrl+u", "llo", 0, true},    // delete to start

		// Unknown key
		{"hello", 2, "f1", "hello", 2, false},
	}

	for _, tt := range tests {
		gotText, gotPos, gotOK := handleTextInputKey(tt.text, tt.cursor, tt.key)
		if gotText != tt.wantText || gotPos != tt.wantPos || gotOK != tt.wantOK {
			t.Errorf("handleTextInputKey(%q, %d, %q) = (%q, %d, %v), want (%q, %d, %v)",
				tt.text, tt.cursor, tt.key, gotText, gotPos, gotOK, tt.wantText, tt.wantPos, tt.wantOK)
		}
	}
}

func TestMinMax(t *testing.T) {
	if max(1, 2) != 2 {
		t.Error("max(1, 2) should be 2")
	}
	if max(3, 1) != 3 {
		t.Error("max(3, 1) should be 3")
	}
	if min(1, 2) != 1 {
		t.Error("min(1, 2) should be 1")
	}
	if min(3, 1) != 1 {
		t.Error("min(3, 1) should be 1")
	}
}

func TestStatusStyle(t *testing.T) {
	// Just ensure it returns a style without panicking
	_ = StatusStyle("running")
	_ = StatusStyle("waiting")
	_ = StatusStyle("idle")
	_ = StatusStyle("unknown")
}

func TestRoleStyle(t *testing.T) {
	// Just ensure it returns a style without panicking
	_ = RoleStyle("user")
	_ = RoleStyle("assistant")
	_ = RoleStyle("unknown")
}

func TestApplyTheme(t *testing.T) {
	// Test valid themes
	for _, name := range ThemeNames {
		ApplyTheme(name)
		if CurrentThemeName != name {
			t.Errorf("ApplyTheme(%q) didn't set CurrentThemeName", name)
		}
	}

	// Test invalid theme falls back to Dracula
	ApplyTheme("NonexistentTheme")
	if CurrentThemeName != "Dracula" {
		t.Errorf("ApplyTheme with invalid name should fall back to Dracula, got %q", CurrentThemeName)
	}
}

func TestThemeNames(t *testing.T) {
	// Ensure all ThemeNames exist in Themes map
	for _, name := range ThemeNames {
		if _, ok := Themes[name]; !ok {
			t.Errorf("ThemeNames contains %q but Themes map doesn't", name)
		}
	}

	// Ensure Themes map doesn't have extra themes
	if len(ThemeNames) != len(Themes) {
		t.Errorf("ThemeNames has %d entries but Themes has %d", len(ThemeNames), len(Themes))
	}
}
