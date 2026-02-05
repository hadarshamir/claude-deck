package session

import (
	"testing"
)

func TestHasClaudeIndicator(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"unsaved indicator", "✳ my-project", true},
		{"spinner indicator", "⠂ my-project", true},
		{"braille low", "⠀ test", true},
		{"braille high", "⠿ test", true},
		{"no indicator", "my-project", false},
		{"empty string", "", false},
		{"normal char first", "a project", false},
		{"number first", "123 project", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasClaudeIndicator(tt.title)
			if got != tt.want {
				t.Errorf("hasClaudeIndicator(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestHasSpinnerIndicator(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"spinner 1", "⠂ working", true},
		{"spinner 2", "⠄ working", true},
		{"spinner 3", "⠆ working", true},
		{"spinner 4", "⠇ working", true},
		{"spinner 5", "⠃ working", true},
		{"spinner 6", "⠁ working", true},
		{"unsaved indicator (not spinner)", "✳ project", false},
		{"no indicator", "my-project", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSpinnerIndicator(tt.title)
			if got != tt.want {
				t.Errorf("hasSpinnerIndicator(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestCleanClaudeTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"removes unsaved indicator", "✳ my-project", "my-project"},
		{"removes spinner", "⠂ my-project", "my-project"},
		{"trims whitespace", "⠂  spaced  ", "spaced"},
		{"no indicator unchanged", "my-project", "my-project"},
		{"empty string", "", ""},
		{"only indicator", "✳", ""},
		{"indicator with spaces", "✳   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanClaudeTitle(tt.title)
			if got != tt.want {
				t.Errorf("cleanClaudeTitle(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestApplyStatusUpdates(t *testing.T) {
	t.Run("applies status changes", func(t *testing.T) {
		sessions := []*Session{
			{ClaudeSessionID: "uuid-1", Status: StatusIdle},
			{ClaudeSessionID: "uuid-2", Status: StatusIdle},
		}

		updates := []StatusUpdate{
			{SessionID: "uuid-1", Status: StatusRunning},
			{SessionID: "uuid-2", Status: StatusWaiting},
		}

		changed, _ := ApplyStatusUpdates(sessions, updates)

		if !changed {
			t.Error("expected changed=true")
		}
		if sessions[0].Status != StatusRunning {
			t.Errorf("session[0].Status = %v, want Running", sessions[0].Status)
		}
		if sessions[1].Status != StatusWaiting {
			t.Errorf("session[1].Status = %v, want Waiting", sessions[1].Status)
		}
	})

	t.Run("applies name changes", func(t *testing.T) {
		sessions := []*Session{
			{ClaudeSessionID: "uuid-1", Name: "Old Name"},
		}

		updates := []StatusUpdate{
			{SessionID: "uuid-1", Status: StatusIdle, Name: "New Name"},
		}

		changed, needsSave := ApplyStatusUpdates(sessions, updates)

		if !changed {
			t.Error("expected changed=true")
		}
		if !needsSave {
			t.Error("expected needsSave=true for name change")
		}
		if sessions[0].Name != "New Name" {
			t.Errorf("Name = %q, want 'New Name'", sessions[0].Name)
		}
	})

	t.Run("applies windowID changes", func(t *testing.T) {
		sessions := []*Session{
			{ClaudeSessionID: "uuid-1", KittyWindowID: 0},
		}

		updates := []StatusUpdate{
			{SessionID: "uuid-1", Status: StatusRunning, KittyWindowID: 123},
		}

		changed, needsSave := ApplyStatusUpdates(sessions, updates)

		if !changed {
			t.Error("expected changed=true")
		}
		if !needsSave {
			t.Error("expected needsSave=true for windowID change")
		}
		if sessions[0].KittyWindowID != 123 {
			t.Errorf("KittyWindowID = %d, want 123", sessions[0].KittyWindowID)
		}
	})

	t.Run("clears stale windowID when claimed by another session", func(t *testing.T) {
		sessions := []*Session{
			{ClaudeSessionID: "uuid-1", KittyWindowID: 100},
			{ClaudeSessionID: "uuid-2", KittyWindowID: 0},
		}

		// uuid-2 is claiming window 100 (which uuid-1 had)
		updates := []StatusUpdate{
			{SessionID: "uuid-2", Status: StatusRunning, KittyWindowID: 100},
		}

		changed, needsSave := ApplyStatusUpdates(sessions, updates)

		if !changed {
			t.Error("expected changed=true")
		}
		if !needsSave {
			t.Error("expected needsSave=true")
		}
		if sessions[0].KittyWindowID != 0 {
			t.Errorf("session[0].KittyWindowID = %d, want 0 (should be cleared)", sessions[0].KittyWindowID)
		}
		if sessions[1].KittyWindowID != 100 {
			t.Errorf("session[1].KittyWindowID = %d, want 100", sessions[1].KittyWindowID)
		}
	})

	t.Run("no changes returns false", func(t *testing.T) {
		sessions := []*Session{
			{ClaudeSessionID: "uuid-1", Status: StatusRunning, Name: "Test", KittyWindowID: 123},
		}

		updates := []StatusUpdate{
			{SessionID: "uuid-1", Status: StatusRunning, Name: "", KittyWindowID: 123},
		}

		changed, needsSave := ApplyStatusUpdates(sessions, updates)

		if changed {
			t.Error("expected changed=false when no actual changes")
		}
		if needsSave {
			t.Error("expected needsSave=false when no changes")
		}
	})

	t.Run("empty name in update doesn't overwrite", func(t *testing.T) {
		sessions := []*Session{
			{ClaudeSessionID: "uuid-1", Name: "Keep This"},
		}

		updates := []StatusUpdate{
			{SessionID: "uuid-1", Status: StatusIdle, Name: ""}, // empty name
		}

		ApplyStatusUpdates(sessions, updates)

		if sessions[0].Name != "Keep This" {
			t.Errorf("Name = %q, should not be overwritten by empty name", sessions[0].Name)
		}
	})
}

func TestClaimWindowID(t *testing.T) {
	t.Run("claims window and clears from other session", func(t *testing.T) {
		sessions := []*Session{
			{ID: "s1", KittyWindowID: 100},
			{ID: "s2", KittyWindowID: 0},
		}

		modified := ClaimWindowID(sessions, sessions[1], 100)

		if !modified {
			t.Error("expected modified=true")
		}
		if sessions[0].KittyWindowID != 0 {
			t.Errorf("s1.KittyWindowID = %d, want 0", sessions[0].KittyWindowID)
		}
		if sessions[1].KittyWindowID != 100 {
			t.Errorf("s2.KittyWindowID = %d, want 100", sessions[1].KittyWindowID)
		}
	})

	t.Run("claims window when no conflict", func(t *testing.T) {
		sessions := []*Session{
			{ID: "s1", KittyWindowID: 0},
			{ID: "s2", KittyWindowID: 0},
		}

		modified := ClaimWindowID(sessions, sessions[0], 200)

		if modified {
			t.Error("expected modified=false when no conflict")
		}
		if sessions[0].KittyWindowID != 200 {
			t.Errorf("s1.KittyWindowID = %d, want 200", sessions[0].KittyWindowID)
		}
	})

	t.Run("invalid windowID returns false", func(t *testing.T) {
		sessions := []*Session{
			{ID: "s1", KittyWindowID: 100},
		}

		modified := ClaimWindowID(sessions, sessions[0], 0)
		if modified {
			t.Error("expected modified=false for windowID=0")
		}

		modified = ClaimWindowID(sessions, sessions[0], -1)
		if modified {
			t.Error("expected modified=false for windowID=-1")
		}
	})

	t.Run("claiming own windowID doesn't modify", func(t *testing.T) {
		sessions := []*Session{
			{ID: "s1", KittyWindowID: 100},
		}

		modified := ClaimWindowID(sessions, sessions[0], 100)

		if modified {
			t.Error("expected modified=false when claiming own windowID")
		}
		if sessions[0].KittyWindowID != 100 {
			t.Errorf("KittyWindowID = %d, want 100", sessions[0].KittyWindowID)
		}
	})

	t.Run("clears multiple sessions with same windowID", func(t *testing.T) {
		// Edge case: multiple sessions somehow have same windowID
		sessions := []*Session{
			{ID: "s1", KittyWindowID: 100},
			{ID: "s2", KittyWindowID: 100},
			{ID: "s3", KittyWindowID: 0},
		}

		modified := ClaimWindowID(sessions, sessions[2], 100)

		if !modified {
			t.Error("expected modified=true")
		}
		if sessions[0].KittyWindowID != 0 {
			t.Errorf("s1.KittyWindowID = %d, want 0", sessions[0].KittyWindowID)
		}
		if sessions[1].KittyWindowID != 0 {
			t.Errorf("s2.KittyWindowID = %d, want 0", sessions[1].KittyWindowID)
		}
		if sessions[2].KittyWindowID != 100 {
			t.Errorf("s3.KittyWindowID = %d, want 100", sessions[2].KittyWindowID)
		}
	})
}
