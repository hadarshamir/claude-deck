package session

import (
	"testing"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusIdle, "idle"},
		{StatusWaiting, "waiting"},
		{StatusRunning, "running"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("Status.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusSymbol(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusIdle, "○"},
		{StatusWaiting, "◎"},
		{StatusRunning, "●"},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := tt.status.Symbol(); got != tt.want {
				t.Errorf("Status.Symbol() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/Users/test/project", []string{"Users", "test", "project"}},
		{"/single", []string{"single"}},
		{"", []string{}},
		{"/", []string{}},
		{"no/leading/slash", []string{"no", "leading", "slash"}},
		{"/trailing/slash/", []string{"trailing", "slash"}},
		{"/multiple//slashes", []string{"multiple", "slashes"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := splitPath(tt.path)
			if len(got) != len(tt.want) {
				t.Errorf("splitPath(%q) = %v, want %v", tt.path, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSessionFolderName(t *testing.T) {
	tests := []struct {
		name        string
		projectPath string
		want        string
	}{
		{"normal path", "/Users/test/my-project", "my-project"},
		{"single folder", "/project", "project"},
		{"empty path", "", ""},
		{"trailing slash", "/Users/test/project/", "project"},
		{"root (edge case)", "/", "/"},  // edge case - returns path itself
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{ProjectPath: tt.projectPath}
			if got := s.FolderName(); got != tt.want {
				t.Errorf("Session.FolderName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionListItem(t *testing.T) {
	s := &Session{ID: "sess-123", Name: "Test Session"}

	if s.ItemID() != "sess-123" {
		t.Errorf("Session.ItemID() = %q, want %q", s.ItemID(), "sess-123")
	}
	if s.ItemName() != "Test Session" {
		t.Errorf("Session.ItemName() = %q, want %q", s.ItemName(), "Test Session")
	}
	if s.IsGroup() {
		t.Error("Session.IsGroup() = true, want false")
	}
}

func TestGroupListItem(t *testing.T) {
	g := &Group{ID: "grp-456", Name: "Test Group"}

	if g.ItemID() != "grp-456" {
		t.Errorf("Group.ItemID() = %q, want %q", g.ItemID(), "grp-456")
	}
	if g.ItemName() != "Test Group" {
		t.Errorf("Group.ItemName() = %q, want %q", g.ItemName(), "Test Group")
	}
	if !g.IsGroup() {
		t.Error("Group.IsGroup() = false, want true")
	}
}
