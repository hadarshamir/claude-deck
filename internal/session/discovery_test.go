package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeProjectPath(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    string
	}{
		{
			name:    "standard path",
			encoded: "-Users-hadar-Projects-myapp",
			want:    "/Users/hadar/Projects/myapp",
		},
		{
			name:    "single component",
			encoded: "-root",
			want:    "/root",
		},
		{
			name:    "empty string",
			encoded: "",
			want:    "",
		},
		{
			name:    "just dash",
			encoded: "-",
			want:    "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodeProjectPath(tt.encoded)
			if got != tt.want {
				t.Errorf("DecodeProjectPath(%q) = %q, want %q", tt.encoded, got, tt.want)
			}
		})
	}
}

func TestEncodeProjectPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "standard path",
			path: "/Users/hadar/Projects/myapp",
			want: "-Users-hadar-Projects-myapp",
		},
		{
			name: "path with underscores",
			path: "/Users/hadar/my_project",
			want: "-Users-hadar-my-project",
		},
		{
			name: "root",
			path: "/",
			want: "-",
		},
		{
			name: "empty",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeProjectPath(tt.path)
			if got != tt.want {
				t.Errorf("EncodeProjectPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFormatSessionName(t *testing.T) {
	testTime := time.Date(2024, 3, 15, 14, 30, 0, 0, time.Local)

	tests := []struct {
		name        string
		projectPath string
		want        string
	}{
		{
			name:        "normal path",
			projectPath: "/Users/hadar/Projects/myapp",
			want:        "myapp Mar 15 14:30",
		},
		{
			name:        "root path",
			projectPath: "/",
			want:        "root Mar 15 14:30",
		},
		{
			name:        "empty path",
			projectPath: "",
			want:        "root Mar 15 14:30",
		},
		{
			name:        "dot path",
			projectPath: ".",
			want:        "root Mar 15 14:30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSessionName(tt.projectPath, testTime)
			if got != tt.want {
				t.Errorf("formatSessionName(%q, ...) = %q, want %q", tt.projectPath, got, tt.want)
			}
		})
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid UUID v4",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  true,
		},
		{
			name:  "valid UUID without dashes (still valid per RFC)",
			input: "550e8400e29b41d4a716446655440000",
			want:  true,
		},
		{
			name:  "too short",
			input: "550e8400",
			want:  false,
		},
		{
			name:  "invalid characters",
			input: "550e8400-e29b-41d4-a716-44665544xxxx",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "history file",
			input: ".history",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidUUID(tt.input)
			if got != tt.want {
				t.Errorf("isValidUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetSessionFileInfo(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("extracts cwd and gitBranch", func(t *testing.T) {
		jsonlPath := filepath.Join(tmpDir, "session1.jsonl")
		content := `{"type":"summary","cwd":"/Users/test/project","gitBranch":"main"}
{"type":"message","message":{"role":"user","content":"hello"}}
`
		if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		info := GetSessionFileInfo(jsonlPath)

		if info.cwd != "/Users/test/project" {
			t.Errorf("cwd = %q, want '/Users/test/project'", info.cwd)
		}
		if info.gitBranch != "main" {
			t.Errorf("gitBranch = %q, want 'main'", info.gitBranch)
		}
		if !info.hasContent {
			t.Error("hasContent should be true (has user message)")
		}
	})

	t.Run("hasContent false when no messages", func(t *testing.T) {
		jsonlPath := filepath.Join(tmpDir, "session2.jsonl")
		content := `{"type":"summary","cwd":"/Users/test/project"}
`
		if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		info := GetSessionFileInfo(jsonlPath)

		if info.hasContent {
			t.Error("hasContent should be false (no user/assistant messages)")
		}
	})

	t.Run("empty file returns empty info", func(t *testing.T) {
		jsonlPath := filepath.Join(tmpDir, "empty.jsonl")
		if err := os.WriteFile(jsonlPath, []byte{}, 0644); err != nil {
			t.Fatal(err)
		}

		info := GetSessionFileInfo(jsonlPath)

		if info.cwd != "" || info.gitBranch != "" || info.hasContent {
			t.Error("expected empty info for empty file")
		}
	})

	t.Run("non-existent file returns empty info", func(t *testing.T) {
		info := GetSessionFileInfo("/non/existent/path.jsonl")

		if info.cwd != "" || info.gitBranch != "" || info.hasContent {
			t.Error("expected empty info for non-existent file")
		}
	})
}

func TestProjectPathFromJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("uses cwd from file when available", func(t *testing.T) {
		// Create directory structure like Claude's
		projectDir := filepath.Join(tmpDir, "-Users-fake-path")
		os.MkdirAll(projectDir, 0755)

		jsonlPath := filepath.Join(projectDir, "session.jsonl")
		content := `{"type":"summary","cwd":"/actual/path/from/file"}`
		if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := ProjectPathFromJSONL(jsonlPath)

		if got != "/actual/path/from/file" {
			t.Errorf("ProjectPathFromJSONL() = %q, want '/actual/path/from/file'", got)
		}
	})

	t.Run("falls back to decoded path when no cwd", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "-Users-test-project")
		os.MkdirAll(projectDir, 0755)

		jsonlPath := filepath.Join(projectDir, "session.jsonl")
		content := `{"type":"summary"}` // no cwd field
		if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := ProjectPathFromJSONL(jsonlPath)

		if got != "/Users/test/project" {
			t.Errorf("ProjectPathFromJSONL() = %q, want '/Users/test/project'", got)
		}
	})
}

func TestRefreshSession(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("updates LastAccessedAt from file", func(t *testing.T) {
		jsonlPath := filepath.Join(tmpDir, "session.jsonl")
		if err := os.WriteFile(jsonlPath, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}

		s := &Session{JSONLPath: jsonlPath}
		oldTime := s.LastAccessedAt

		// Modify file to update mtime
		time.Sleep(10 * time.Millisecond)
		if err := os.WriteFile(jsonlPath, []byte("{}\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := RefreshSession(s); err != nil {
			t.Fatalf("RefreshSession failed: %v", err)
		}

		if !s.LastAccessedAt.After(oldTime) {
			t.Error("LastAccessedAt should be updated")
		}
	})

	t.Run("returns nil for empty JSONLPath", func(t *testing.T) {
		s := &Session{JSONLPath: ""}
		if err := RefreshSession(s); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		s := &Session{JSONLPath: "/non/existent/file.jsonl"}
		if err := RefreshSession(s); err == nil {
			t.Error("expected error for non-existent file")
		}
	})
}

func TestClaudeProjectsDir(t *testing.T) {
	dir := ClaudeProjectsDir()
	if dir == "" {
		t.Error("ClaudeProjectsDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("ClaudeProjectsDir() = %q, want absolute path", dir)
	}
	if filepath.Base(dir) != "projects" {
		t.Errorf("ClaudeProjectsDir() should end with 'projects', got %q", dir)
	}
}

func TestDiscoverSessionsNoDirectory(t *testing.T) {
	// Set HOME to a temp dir without .claude/projects
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	sessions, err := DiscoverSessions()
	if err != nil {
		t.Fatalf("DiscoverSessions() error = %v", err)
	}
	if sessions != nil && len(sessions) != 0 {
		t.Errorf("expected empty sessions, got %d", len(sessions))
	}
}

func TestDiscoverSessionsWithSessions(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create .claude/projects structure
	projectsDir := filepath.Join(tmpDir, ".claude", "projects")
	projectDir := filepath.Join(projectsDir, "-Users-test-myproject")
	os.MkdirAll(projectDir, 0755)

	// Create a valid session JSONL
	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	jsonlPath := filepath.Join(projectDir, sessionID+".jsonl")
	content := `{"type":"summary","cwd":"/Users/test/myproject"}
{"type":"message","message":{"role":"user","content":"test"}}
`
	if err := os.WriteFile(jsonlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-UUID file (should be skipped)
	os.WriteFile(filepath.Join(projectDir, ".history.jsonl"), []byte("{}"), 0644)

	sessions, err := DiscoverSessions()
	if err != nil {
		t.Fatalf("DiscoverSessions() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.ClaudeSessionID != sessionID {
		t.Errorf("ClaudeSessionID = %q, want %q", s.ClaudeSessionID, sessionID)
	}
	if s.ProjectPath != "/Users/test/myproject" {
		t.Errorf("ProjectPath = %q, want '/Users/test/myproject'", s.ProjectPath)
	}
}

func TestGetSessionUUIDsAtPath(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create .claude/projects structure
	projectsDir := filepath.Join(tmpDir, ".claude", "projects")
	projectDir := filepath.Join(projectsDir, "-test-path")
	os.MkdirAll(projectDir, 0755)

	// Create session files
	uuid1 := "550e8400-e29b-41d4-a716-446655440001"
	uuid2 := "550e8400-e29b-41d4-a716-446655440002"
	os.WriteFile(filepath.Join(projectDir, uuid1+".jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(projectDir, uuid2+".jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(projectDir, "not-a-uuid.jsonl"), []byte("{}"), 0644)

	uuids := GetSessionUUIDsAtPath("/test/path")

	if len(uuids) != 2 {
		t.Errorf("expected 2 UUIDs, got %d", len(uuids))
	}
}

func TestGetSessionUUIDsAtPathNoDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	uuids := GetSessionUUIDsAtPath("/nonexistent/path")
	if len(uuids) != 0 {
		t.Errorf("expected 0 UUIDs for nonexistent path, got %d", len(uuids))
	}
}

func TestFindNewestSessionAtPath(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create .claude/projects structure
	projectsDir := filepath.Join(tmpDir, ".claude", "projects")
	projectDir := filepath.Join(projectsDir, "-test-path")
	os.MkdirAll(projectDir, 0755)

	// Create session files with different times
	uuid1 := "550e8400-e29b-41d4-a716-446655440001"
	uuid2 := "550e8400-e29b-41d4-a716-446655440002"

	os.WriteFile(filepath.Join(projectDir, uuid1+".jsonl"), []byte("{}"), 0644)
	// Wait a bit and create second file
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(projectDir, uuid2+".jsonl"), []byte("{}"), 0644)

	newest := FindNewestSessionAtPath("/test/path")

	if newest != uuid2 {
		t.Errorf("expected newest to be %q, got %q", uuid2, newest)
	}
}

func TestFindNewestSessionAtPathNoDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	newest := FindNewestSessionAtPath("/nonexistent/path")
	if newest != "" {
		t.Errorf("expected empty string for nonexistent path, got %q", newest)
	}
}

func TestGetProjectPathFromJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("extracts cwd", func(t *testing.T) {
		jsonlPath := filepath.Join(tmpDir, "session.jsonl")
		content := `{"cwd":"/Users/test/project"}`
		os.WriteFile(jsonlPath, []byte(content), 0644)

		got := GetProjectPathFromJSONL(jsonlPath)
		if got != "/Users/test/project" {
			t.Errorf("got %q, want '/Users/test/project'", got)
		}
	})

	t.Run("returns empty for no cwd", func(t *testing.T) {
		jsonlPath := filepath.Join(tmpDir, "no_cwd.jsonl")
		content := `{"type":"summary"}`
		os.WriteFile(jsonlPath, []byte(content), 0644)

		got := GetProjectPathFromJSONL(jsonlPath)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty for nonexistent file", func(t *testing.T) {
		got := GetProjectPathFromJSONL("/nonexistent/file.jsonl")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestGetSessionFileInfoGitBranch(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "session.jsonl")
	content := `{"cwd":"/path","gitBranch":"feature-branch"}`
	os.WriteFile(jsonlPath, []byte(content), 0644)

	info := GetSessionFileInfo(jsonlPath)

	if info.gitBranch != "feature-branch" {
		t.Errorf("gitBranch = %q, want 'feature-branch'", info.gitBranch)
	}
}

func TestGetSessionFileInfoHasContentAssistant(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "session.jsonl")
	content := `{"type":"message","message":{"role":"assistant","content":"response"}}`
	os.WriteFile(jsonlPath, []byte(content), 0644)

	info := GetSessionFileInfo(jsonlPath)

	if !info.hasContent {
		t.Error("hasContent should be true for assistant message")
	}
}
