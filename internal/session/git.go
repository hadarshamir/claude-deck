package session

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// RefreshGitBranches updates git branch for all sessions (unique paths only)
// Called once on startup
func RefreshGitBranches(sessions []*Session) {
	// Group by unique path
	byPath := make(map[string][]*Session)
	for _, s := range sessions {
		if s.ProjectPath != "" {
			byPath[s.ProjectPath] = append(byPath[s.ProjectPath], s)
		}
	}

	// Get current branch for each unique path
	for path, pathSessions := range byPath {
		branch := getCurrentBranch(path)
		if branch != "" {
			for _, s := range pathSessions {
				s.GitBranch = branch
			}
		}
	}
}

// getCurrentBranch gets the current git branch for a directory
func getCurrentBranch(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// GitStatusString returns the git branch
func (s *Session) GitStatusString() string {
	return s.GitBranch
}
