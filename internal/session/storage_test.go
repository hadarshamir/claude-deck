package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMergeSessions(t *testing.T) {
	now := time.Now()

	t.Run("new session gets added", func(t *testing.T) {
		discovered := []*Session{
			{ClaudeSessionID: "uuid-1", Name: "Session 1", LastAccessedAt: now},
		}
		stored := &StorageData{Sessions: []*Session{}}

		result := MergeSessions(discovered, stored)

		if len(result) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result))
		}
		if result[0].ClaudeSessionID != "uuid-1" {
			t.Errorf("expected session uuid-1, got %s", result[0].ClaudeSessionID)
		}
	})

	t.Run("stored metadata is preserved", func(t *testing.T) {
		discovered := []*Session{
			{ClaudeSessionID: "uuid-1", Name: "Discovered Name", JSONLPath: "/path/to/file.jsonl"},
		}
		stored := &StorageData{
			Sessions: []*Session{
				{ClaudeSessionID: "uuid-1", Name: "Custom Name", GroupPath: "my-group", Pinned: true, Order: 5},
			},
		}

		result := MergeSessions(discovered, stored)

		if len(result) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result))
		}
		if result[0].Name != "Custom Name" {
			t.Errorf("expected name 'Custom Name', got %q", result[0].Name)
		}
		if result[0].GroupPath != "my-group" {
			t.Errorf("expected group 'my-group', got %q", result[0].GroupPath)
		}
		if !result[0].Pinned {
			t.Error("expected Pinned=true")
		}
		if result[0].JSONLPath != "/path/to/file.jsonl" {
			t.Errorf("expected JSONLPath to be updated, got %q", result[0].JSONLPath)
		}
	})

	t.Run("pending sessions with KittyWindowID are preserved", func(t *testing.T) {
		discovered := []*Session{}
		stored := &StorageData{
			Sessions: []*Session{
				{ClaudeSessionID: "pending-uuid", KittyWindowID: 123, Name: "Pending"},
			},
		}

		result := MergeSessions(discovered, stored)

		if len(result) != 1 {
			t.Fatalf("expected 1 session (pending), got %d", len(result))
		}
		if result[0].Status != StatusWaiting {
			t.Errorf("expected StatusWaiting, got %v", result[0].Status)
		}
	})

	t.Run("sessions without KittyWindowID are not preserved", func(t *testing.T) {
		discovered := []*Session{}
		stored := &StorageData{
			Sessions: []*Session{
				{ClaudeSessionID: "old-uuid", KittyWindowID: 0, Name: "Old Session"},
			},
		}

		result := MergeSessions(discovered, stored)

		if len(result) != 0 {
			t.Fatalf("expected 0 sessions, got %d", len(result))
		}
	})
}

func TestManagerFindSession(t *testing.T) {
	m := &Manager{
		Sessions: []*Session{
			{ID: "sess-1", Name: "Session 1"},
			{ID: "sess-2", Name: "Session 2"},
		},
	}

	t.Run("finds existing session", func(t *testing.T) {
		s := m.FindSession("sess-1")
		if s == nil {
			t.Fatal("expected to find session")
		}
		if s.Name != "Session 1" {
			t.Errorf("expected 'Session 1', got %q", s.Name)
		}
	})

	t.Run("returns nil for non-existent session", func(t *testing.T) {
		s := m.FindSession("non-existent")
		if s != nil {
			t.Error("expected nil for non-existent session")
		}
	})
}

func TestManagerFindGroup(t *testing.T) {
	m := &Manager{
		Groups: []*Group{
			{ID: "grp-1", Name: "Group 1"},
			{ID: "grp-2", Name: "Group 2"},
		},
	}

	t.Run("finds existing group", func(t *testing.T) {
		g := m.FindGroup("grp-1")
		if g == nil {
			t.Fatal("expected to find group")
		}
		if g.Name != "Group 1" {
			t.Errorf("expected 'Group 1', got %q", g.Name)
		}
	})

	t.Run("returns nil for non-existent group", func(t *testing.T) {
		g := m.FindGroup("non-existent")
		if g != nil {
			t.Error("expected nil for non-existent group")
		}
	})
}

func TestManagerSessionsInGroup(t *testing.T) {
	m := &Manager{
		Sessions: []*Session{
			{ID: "s1", GroupPath: "group-a", Order: 2},
			{ID: "s2", GroupPath: "group-b", Order: 1},
			{ID: "s3", GroupPath: "group-a", Order: 1},
			{ID: "s4", GroupPath: "", Order: 3},
		},
	}

	result := m.SessionsInGroup("group-a")

	if len(result) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(result))
	}
	// Should be sorted by Order
	if result[0].ID != "s3" {
		t.Errorf("expected first session to be s3 (order 1), got %s", result[0].ID)
	}
	if result[1].ID != "s1" {
		t.Errorf("expected second session to be s1 (order 2), got %s", result[1].ID)
	}
}

func TestManagerUngroupedSessions(t *testing.T) {
	m := &Manager{
		Groups: []*Group{
			{ID: "grp-1", Path: "valid-group"},
		},
		Sessions: []*Session{
			{ID: "s1", GroupPath: "valid-group", Order: 1},
			{ID: "s2", GroupPath: "", Order: 2},
			{ID: "s3", GroupPath: "deleted-group", Order: 3}, // orphaned
		},
	}

	result := m.UngroupedSessions()

	if len(result) != 2 {
		t.Fatalf("expected 2 ungrouped sessions, got %d", len(result))
	}
	// Should include both ungrouped and orphaned
	ids := make(map[string]bool)
	for _, s := range result {
		ids[s.ID] = true
	}
	if !ids["s2"] || !ids["s3"] {
		t.Errorf("expected s2 and s3, got %v", ids)
	}
}

func TestManagerSettings(t *testing.T) {
	t.Run("GetTheme returns default when nil", func(t *testing.T) {
		m := &Manager{Settings: nil}
		if m.GetTheme() != "Dracula" {
			t.Errorf("expected default theme, got %q", m.GetTheme())
		}
	})

	t.Run("GetTheme returns default when empty", func(t *testing.T) {
		m := &Manager{Settings: &Settings{Theme: ""}}
		if m.GetTheme() != "Dracula" {
			t.Errorf("expected default theme, got %q", m.GetTheme())
		}
	})

	t.Run("GetTheme returns set value", func(t *testing.T) {
		m := &Manager{Settings: &Settings{Theme: "Nord"}}
		if m.GetTheme() != "Nord" {
			t.Errorf("expected 'Nord', got %q", m.GetTheme())
		}
	})

	t.Run("GetActiveExpanded returns true by default", func(t *testing.T) {
		m := &Manager{Settings: nil}
		if !m.GetActiveExpanded() {
			t.Error("expected true by default")
		}
	})

	t.Run("GetActiveExpanded returns set value", func(t *testing.T) {
		expanded := false
		m := &Manager{Settings: &Settings{ActiveExpanded: &expanded}}
		if m.GetActiveExpanded() {
			t.Error("expected false")
		}
	})

	t.Run("GetResumeOnStartup returns false by default", func(t *testing.T) {
		m := &Manager{Settings: nil}
		if m.GetResumeOnStartup() {
			t.Error("expected false by default")
		}
	})
}

func TestManagerFavoritePaths(t *testing.T) {
	m := &Manager{Settings: &Settings{
		FavoritePaths: []string{"/path/one", "/path/two"},
	}}

	t.Run("GetFavoritePaths returns paths", func(t *testing.T) {
		paths := m.GetFavoritePaths()
		if len(paths) != 2 {
			t.Errorf("expected 2 paths, got %d", len(paths))
		}
	})

	t.Run("GetFavoritePaths returns nil when settings nil", func(t *testing.T) {
		m2 := &Manager{Settings: nil}
		paths := m2.GetFavoritePaths()
		if paths != nil {
			t.Errorf("expected nil, got %v", paths)
		}
	})

	t.Run("IsFavoritePath returns true for favorite", func(t *testing.T) {
		if !m.IsFavoritePath("/path/one") {
			t.Error("expected /path/one to be favorite")
		}
	})

	t.Run("IsFavoritePath returns false for non-favorite", func(t *testing.T) {
		if m.IsFavoritePath("/path/three") {
			t.Error("expected /path/three to not be favorite")
		}
	})
}

func TestManagerGetUniqueProjectPaths(t *testing.T) {
	now := time.Now()
	m := &Manager{
		Sessions: []*Session{
			{ProjectPath: "/path/a", LastAccessedAt: now.Add(-2 * time.Hour)},
			{ProjectPath: "/path/b", LastAccessedAt: now.Add(-1 * time.Hour)},
			{ProjectPath: "/path/a", LastAccessedAt: now}, // more recent
			{ProjectPath: "", LastAccessedAt: now},       // empty path
		},
	}

	paths := m.GetUniqueProjectPaths()

	if len(paths) != 2 {
		t.Fatalf("expected 2 unique paths, got %d", len(paths))
	}
	// Should be sorted by most recent first
	if paths[0] != "/path/a" {
		t.Errorf("expected /path/a first (most recent), got %s", paths[0])
	}
	if paths[1] != "/path/b" {
		t.Errorf("expected /path/b second, got %s", paths[1])
	}
}

func TestSwapSessions(t *testing.T) {
	// Create temp dir for storage file
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create .claude-sessions dir
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Sessions: []*Session{
			{ID: "s1", GroupPath: "group-a", Order: 1},
			{ID: "s2", GroupPath: "group-b", Order: 2},
		},
		Settings: &Settings{},
	}

	err := m.SwapSessions("s1", "s2")
	if err != nil {
		t.Fatalf("SwapSessions failed: %v", err)
	}

	s1 := m.FindSession("s1")
	s2 := m.FindSession("s2")

	if s1.GroupPath != "group-b" {
		t.Errorf("s1.GroupPath = %q, want 'group-b'", s1.GroupPath)
	}
	if s2.GroupPath != "group-a" {
		t.Errorf("s2.GroupPath = %q, want 'group-a'", s2.GroupPath)
	}
	if s1.Order != 2 {
		t.Errorf("s1.Order = %d, want 2", s1.Order)
	}
	if s2.Order != 1 {
		t.Errorf("s2.Order = %d, want 1", s2.Order)
	}
}

func TestRenameSession(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Sessions: []*Session{
			{ID: "s1", Name: "Original", Renamed: false},
		},
		Settings: &Settings{},
	}

	t.Run("rename sets name and Renamed flag", func(t *testing.T) {
		m.RenameSession("s1", "Custom Name")
		s := m.FindSession("s1")
		if s.Name != "Custom Name" {
			t.Errorf("Name = %q, want 'Custom Name'", s.Name)
		}
		if !s.Renamed {
			t.Error("Renamed should be true")
		}
	})

	t.Run("empty name resets to dynamic", func(t *testing.T) {
		m.RenameSession("s1", "")
		s := m.FindSession("s1")
		if s.Renamed {
			t.Error("Renamed should be false after reset")
		}
	})
}

func TestTogglePin(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Sessions: []*Session{
			{ID: "s1", Pinned: false},
		},
		Settings: &Settings{},
	}

	m.TogglePin("s1")
	if !m.FindSession("s1").Pinned {
		t.Error("expected Pinned=true after first toggle")
	}

	m.TogglePin("s1")
	if m.FindSession("s1").Pinned {
		t.Error("expected Pinned=false after second toggle")
	}
}

func TestDeleteSession(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Sessions: []*Session{
			{ID: "s1", Name: "Session 1"},
			{ID: "s2", Name: "Session 2"},
		},
		Settings: &Settings{},
	}

	m.DeleteSession("s1")

	if len(m.Sessions) != 1 {
		t.Errorf("expected 1 session after delete, got %d", len(m.Sessions))
	}
	if m.Sessions[0].ID != "s2" {
		t.Errorf("expected s2 to remain, got %s", m.Sessions[0].ID)
	}
}

func TestMoveSession(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Sessions: []*Session{
			{ID: "s1", GroupPath: ""},
		},
		Settings: &Settings{},
	}

	m.MoveSession("s1", "new-group")

	if m.FindSession("s1").GroupPath != "new-group" {
		t.Errorf("GroupPath = %q, want 'new-group'", m.FindSession("s1").GroupPath)
	}
}

func TestCreateGroup(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Groups:   []*Group{},
		Settings: &Settings{},
	}

	g, err := m.CreateGroup("Test Group", "")
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	if g.Name != "Test Group" {
		t.Errorf("Name = %q, want 'Test Group'", g.Name)
	}
	if g.Path != "Test Group" {
		t.Errorf("Path = %q, want 'Test Group'", g.Path)
	}
	if !g.Expanded {
		t.Error("Expanded should be true by default")
	}
	if len(m.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(m.Groups))
	}
}

func TestCreateGroupWithParent(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Groups:   []*Group{},
		Settings: &Settings{},
	}

	g, _ := m.CreateGroup("Child", "Parent")

	if g.Path != "Parent/Child" {
		t.Errorf("Path = %q, want 'Parent/Child'", g.Path)
	}
}

func TestRenameGroup(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Groups: []*Group{
			{ID: "grp-1", Name: "Old Name", Path: "Old Name"},
		},
		Settings: &Settings{},
	}

	m.RenameGroup("grp-1", "New Name")

	g := m.FindGroup("grp-1")
	if g.Name != "New Name" {
		t.Errorf("Name = %q, want 'New Name'", g.Name)
	}
	if g.Path != "New Name" {
		t.Errorf("Path = %q, want 'New Name'", g.Path)
	}
}

func TestDeleteGroup(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Groups: []*Group{
			{ID: "grp-1", Name: "Group 1", Path: "Group 1"},
		},
		Sessions: []*Session{
			{ID: "s1", GroupPath: "Group 1"},
			{ID: "s2", GroupPath: ""},
		},
		Settings: &Settings{},
	}

	m.DeleteGroup("grp-1")

	if len(m.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(m.Groups))
	}
	// Session should be moved out of deleted group
	if m.FindSession("s1").GroupPath != "" {
		t.Errorf("session s1 should have empty GroupPath, got %q", m.FindSession("s1").GroupPath)
	}
}

func TestToggleGroupExpanded(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{
		Groups: []*Group{
			{ID: "grp-1", Expanded: true},
		},
		Settings: &Settings{},
	}

	m.ToggleGroupExpanded("grp-1")
	if m.FindGroup("grp-1").Expanded {
		t.Error("expected Expanded=false after first toggle")
	}

	m.ToggleGroupExpanded("grp-1")
	if !m.FindGroup("grp-1").Expanded {
		t.Error("expected Expanded=true after second toggle")
	}
}

func TestRemovePendingSession(t *testing.T) {
	m := &Manager{
		Sessions: []*Session{
			{ID: "s1", ClaudeSessionID: "uuid-1"},
			{ID: "s2", ClaudeSessionID: "uuid-2"},
		},
	}

	m.RemovePendingSession("s1")
	if len(m.Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(m.Sessions))
	}

	// Also works with ClaudeSessionID
	m.RemovePendingSession("uuid-2")
	if len(m.Sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(m.Sessions))
	}
}

func TestSettersWithNilSettings(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	t.Run("SetActiveExpanded creates settings", func(t *testing.T) {
		m := &Manager{Settings: nil}
		m.SetActiveExpanded(false)
		if m.Settings == nil {
			t.Error("Settings should be created")
		}
		if m.GetActiveExpanded() {
			t.Error("expected false")
		}
	})

	t.Run("SetInactiveExpanded creates settings", func(t *testing.T) {
		m := &Manager{Settings: nil}
		m.SetInactiveExpanded(false)
		if m.Settings == nil {
			t.Error("Settings should be created")
		}
		if m.GetInactiveExpanded() {
			t.Error("expected false")
		}
	})

	t.Run("SetLastActiveSessionIDs creates settings", func(t *testing.T) {
		m := &Manager{Settings: nil}
		m.SetLastActiveSessionIDs([]string{"a", "b"})
		if m.Settings == nil {
			t.Error("Settings should be created")
		}
		ids := m.GetLastActiveSessionIDs()
		if len(ids) != 2 {
			t.Errorf("expected 2 ids, got %d", len(ids))
		}
	})
}

func TestGetInactiveExpanded(t *testing.T) {
	t.Run("nil settings returns true", func(t *testing.T) {
		m := &Manager{Settings: nil}
		if !m.GetInactiveExpanded() {
			t.Error("expected true by default")
		}
	})

	t.Run("nil pointer returns true", func(t *testing.T) {
		m := &Manager{Settings: &Settings{InactiveExpanded: nil}}
		if !m.GetInactiveExpanded() {
			t.Error("expected true by default")
		}
	})

	t.Run("returns set value", func(t *testing.T) {
		f := false
		m := &Manager{Settings: &Settings{InactiveExpanded: &f}}
		if m.GetInactiveExpanded() {
			t.Error("expected false")
		}
	})
}

func TestToggleFavoritePath(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{Settings: &Settings{}}

	// Add path
	m.ToggleFavoritePath("/path/one")
	if !m.IsFavoritePath("/path/one") {
		t.Error("path should be favorite after adding")
	}

	// Remove path
	m.ToggleFavoritePath("/path/one")
	if m.IsFavoritePath("/path/one") {
		t.Error("path should not be favorite after removing")
	}
}

func TestToggleFavoritePathNilSettings(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{Settings: nil}
	m.ToggleFavoritePath("/new/path")

	if m.Settings == nil {
		t.Error("Settings should be created")
	}
	if !m.IsFavoritePath("/new/path") {
		t.Error("path should be favorite")
	}
}

func TestGetLastActiveSessionIDs(t *testing.T) {
	t.Run("nil settings returns nil", func(t *testing.T) {
		m := &Manager{Settings: nil}
		if m.GetLastActiveSessionIDs() != nil {
			t.Error("expected nil")
		}
	})

	t.Run("returns set value", func(t *testing.T) {
		m := &Manager{Settings: &Settings{LastActiveSessionIDs: []string{"a", "b"}}}
		ids := m.GetLastActiveSessionIDs()
		if len(ids) != 2 {
			t.Errorf("expected 2 ids, got %d", len(ids))
		}
	})
}

func TestSetTheme(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{Settings: nil}
	m.SetTheme("Nord")

	if m.Settings == nil {
		t.Error("Settings should be created")
	}
	if m.GetTheme() != "Nord" {
		t.Errorf("Theme = %q, want 'Nord'", m.GetTheme())
	}
}

func TestSetResumeOnStartup(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)
	os.MkdirAll(filepath.Join(tmpDir, ".claude-sessions"), 0755)

	m := &Manager{Settings: nil}
	m.SetResumeOnStartup(true)

	if m.Settings == nil {
		t.Error("Settings should be created")
	}
	if !m.GetResumeOnStartup() {
		t.Error("expected true")
	}
}

func TestStorageFile(t *testing.T) {
	file := StorageFile()
	if file == "" {
		t.Error("StorageFile() returned empty string")
	}
	if filepath.Base(file) != "sessions.json" {
		t.Errorf("StorageFile() should end with 'sessions.json', got %q", file)
	}
}

func TestStorageDir(t *testing.T) {
	dir := StorageDir()
	if dir == "" {
		t.Error("StorageDir() returned empty string")
	}
	if filepath.Base(dir) != ".claude-sessions" {
		t.Errorf("StorageDir() should end with '.claude-sessions', got %q", dir)
	}
}

func TestLoadStorageNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	data, err := LoadStorage()
	if err != nil {
		t.Fatalf("LoadStorage() error = %v", err)
	}
	if data == nil {
		t.Fatal("LoadStorage() returned nil")
	}
	if len(data.Sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(data.Sessions))
	}
	if data.Settings == nil {
		t.Error("Settings should not be nil")
	}
}

func TestLoadStorageWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create .claude-sessions directory and file
	sessionsDir := filepath.Join(tmpDir, ".claude-sessions")
	os.MkdirAll(sessionsDir, 0755)

	content := `{
		"sessions": [{"id": "s1", "name": "Test Session"}],
		"groups": [{"id": "g1", "name": "Test Group"}],
		"settings": {"theme": "Nord"}
	}`
	os.WriteFile(filepath.Join(sessionsDir, "sessions.json"), []byte(content), 0644)

	data, err := LoadStorage()
	if err != nil {
		t.Fatalf("LoadStorage() error = %v", err)
	}
	if len(data.Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(data.Sessions))
	}
	if data.Sessions[0].Name != "Test Session" {
		t.Errorf("Name = %q, want 'Test Session'", data.Sessions[0].Name)
	}
	if len(data.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(data.Groups))
	}
	if data.Settings.Theme != "Nord" {
		t.Errorf("Theme = %q, want 'Nord'", data.Settings.Theme)
	}
}

func TestLoadStorageInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	sessionsDir := filepath.Join(tmpDir, ".claude-sessions")
	os.MkdirAll(sessionsDir, 0755)
	os.WriteFile(filepath.Join(sessionsDir, "sessions.json"), []byte("not json"), 0644)

	_, err := LoadStorage()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadStorageNilSettings(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	sessionsDir := filepath.Join(tmpDir, ".claude-sessions")
	os.MkdirAll(sessionsDir, 0755)

	// JSON without settings field
	content := `{"sessions": [], "groups": []}`
	os.WriteFile(filepath.Join(sessionsDir, "sessions.json"), []byte(content), 0644)

	data, err := LoadStorage()
	if err != nil {
		t.Fatalf("LoadStorage() error = %v", err)
	}
	if data.Settings == nil {
		t.Error("Settings should be created if nil")
	}
}

func TestSaveStorage(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	data := &StorageData{
		Sessions: []*Session{{ID: "s1", Name: "Test"}},
		Groups:   []*Group{{ID: "g1", Name: "Group"}},
		Settings: &Settings{Theme: "Dark"},
	}

	err := SaveStorage(data)
	if err != nil {
		t.Fatalf("SaveStorage() error = %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(filepath.Join(tmpDir, ".claude-sessions", "sessions.json"))
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if len(content) == 0 {
		t.Error("saved file is empty")
	}
}

func TestMergeSessionsOrderAssignment(t *testing.T) {
	now := time.Now()

	// Test that new sessions get proper order values
	discovered := []*Session{
		{ClaudeSessionID: "new-1", LastAccessedAt: now.Add(-1 * time.Hour)},
		{ClaudeSessionID: "new-2", LastAccessedAt: now},
	}
	stored := &StorageData{
		Sessions: []*Session{
			{ClaudeSessionID: "old-1", Order: 5},
		},
	}

	result := MergeSessions(discovered, stored)

	// New sessions should have order values less than existing min (5)
	for _, s := range result {
		if s.ClaudeSessionID == "new-1" || s.ClaudeSessionID == "new-2" {
			if s.Order >= 5 {
				t.Errorf("new session order %d should be less than 5", s.Order)
			}
		}
	}
}

func TestMergeSessionsOrderReassignment(t *testing.T) {
	now := time.Now()

	// Test that duplicate Order=0 triggers reordering
	discovered := []*Session{
		{ClaudeSessionID: "s1", LastAccessedAt: now.Add(-1 * time.Hour)},
		{ClaudeSessionID: "s2", LastAccessedAt: now},
	}
	stored := &StorageData{
		Sessions: []*Session{
			{ClaudeSessionID: "s1", Order: 0},
			{ClaudeSessionID: "s2", Order: 0}, // duplicate Order=0
		},
	}

	result := MergeSessions(discovered, stored)

	// After reordering, each should have unique order
	orders := make(map[int]bool)
	for _, s := range result {
		if orders[s.Order] {
			t.Errorf("duplicate order value: %d", s.Order)
		}
		orders[s.Order] = true
	}
}
