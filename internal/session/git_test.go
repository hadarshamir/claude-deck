package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitStatusString(t *testing.T) {
	s := &Session{GitBranch: "main"}
	if s.GitStatusString() != "main" {
		t.Errorf("GitStatusString() = %q, want 'main'", s.GitStatusString())
	}

	s.GitBranch = ""
	if s.GitStatusString() != "" {
		t.Errorf("GitStatusString() = %q, want empty", s.GitStatusString())
	}
}

func TestRefreshGitBranches(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create a temp git repo
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "test-repo")
	os.MkdirAll(gitDir, 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user (required for commit)
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitDir
	cmd.Run()

	// Create a commit to have a branch
	testFile := filepath.Join(gitDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = gitDir
	cmd.Run()

	sessions := []*Session{
		{ProjectPath: gitDir, GitBranch: ""},
		{ProjectPath: gitDir, GitBranch: ""}, // duplicate path
		{ProjectPath: "/nonexistent", GitBranch: ""},
		{ProjectPath: "", GitBranch: ""}, // empty path
	}

	RefreshGitBranches(sessions)

	// Both sessions with same path should get branch
	if sessions[0].GitBranch == "" {
		t.Error("expected GitBranch to be set for valid repo")
	}
	if sessions[1].GitBranch != sessions[0].GitBranch {
		t.Error("sessions with same path should have same branch")
	}
	// Non-existent path should remain empty
	if sessions[2].GitBranch != "" {
		t.Errorf("expected empty branch for nonexistent path, got %q", sessions[2].GitBranch)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Run("returns empty for non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		branch := getCurrentBranch(tmpDir)
		if branch != "" {
			t.Errorf("expected empty branch, got %q", branch)
		}
	})

	t.Run("returns empty for nonexistent directory", func(t *testing.T) {
		branch := getCurrentBranch("/nonexistent/path")
		if branch != "" {
			t.Errorf("expected empty branch, got %q", branch)
		}
	})

	t.Run("returns branch for git repo", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git init failed: %v", err)
		}

		// Configure and create initial commit
		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

		branch := getCurrentBranch(tmpDir)
		// Could be "main" or "master" depending on git config
		if branch == "" {
			t.Error("expected non-empty branch")
		}
	})
}
