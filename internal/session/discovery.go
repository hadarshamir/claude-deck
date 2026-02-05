package session

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ClaudeProjectsDir returns the path to Claude Code's projects directory
func ClaudeProjectsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// DecodeProjectPath converts an encoded directory name back to the original path
// Note: Claude's encoding is lossy (/ _ and - all become -), so this is a fallback
func DecodeProjectPath(encoded string) string {
	if encoded == "" {
		return ""
	}
	parts := strings.Split(encoded, "-")
	return "/" + strings.Join(parts[1:], "/")
}

// sessionFileInfo holds info extracted from a JSONL file
type sessionFileInfo struct {
	cwd        string
	gitBranch  string
	hasContent bool
}

// GetSessionFileInfo reads info from a JSONL file
func GetSessionFileInfo(jsonlPath string) sessionFileInfo {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return sessionFileInfo{}
	}
	defer file.Close()

	// Read first 20KB to find cwd, gitBranch and check for content
	data := make([]byte, 20*1024)
	n, _ := file.Read(data)
	if n == 0 {
		return sessionFileInfo{}
	}

	content := string(data[:n])
	info := sessionFileInfo{}

	// Look for "cwd" field in the JSON
	if idx := strings.Index(content, `"cwd":"`); idx != -1 {
		start := idx + 7
		end := start
		for end < n && data[end] != '"' {
			end++
		}
		if end > start {
			info.cwd = string(data[start:end])
		}
	}

	// Look for "gitBranch" field in the JSON
	if idx := strings.Index(content, `"gitBranch":"`); idx != -1 {
		start := idx + 13
		end := start
		for end < n && data[end] != '"' {
			end++
		}
		if end > start {
			info.gitBranch = string(data[start:end])
		}
	}

	// Check if file has actual conversation content (user or assistant messages)
	info.hasContent = strings.Contains(content, `"role":"user"`) ||
		strings.Contains(content, `"role":"assistant"`)

	return info
}

// GetProjectPathFromJSONL reads the actual cwd from a JSONL file
func GetProjectPathFromJSONL(jsonlPath string) string {
	return GetSessionFileInfo(jsonlPath).cwd
}

// ProjectPathFromJSONL extracts the project path from a JSONL file path
// First tries to read the cwd from the JSONL file, falls back to decoding directory name
func ProjectPathFromJSONL(jsonlPath string) string {
	// Try reading the actual cwd from the JSONL file first (most accurate)
	if cwd := GetProjectPathFromJSONL(jsonlPath); cwd != "" {
		return cwd
	}
	// Fall back to decoding directory name (lossy - hyphens in names get confused)
	dir := filepath.Dir(jsonlPath)
	encoded := filepath.Base(dir)
	return DecodeProjectPath(encoded)
}

// EncodeProjectPath converts a path to the encoded directory name format
func EncodeProjectPath(path string) string {
	// Claude replaces both / and _ with -
	encoded := strings.ReplaceAll(path, "/", "-")
	encoded = strings.ReplaceAll(encoded, "_", "-")
	return encoded
}

// GetSessionUUIDsAtPath returns all session UUIDs (from JSONL filenames) for a project path
func GetSessionUUIDsAtPath(projectPath string) []string {
	projectsDir := ClaudeProjectsDir()
	encodedPath := EncodeProjectPath(projectPath)
	projectDir := filepath.Join(projectsDir, encodedPath)

	jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if err != nil {
		return nil
	}

	var uuids []string
	for _, jsonlPath := range jsonlFiles {
		sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
		if isValidUUID(sessionID) {
			uuids = append(uuids, sessionID)
		}
	}
	return uuids
}

// FindNewestSessionAtPath returns the UUID of the most recently modified JSONL at a path
func FindNewestSessionAtPath(projectPath string) string {
	projectsDir := ClaudeProjectsDir()
	encodedPath := EncodeProjectPath(projectPath)
	projectDir := filepath.Join(projectsDir, encodedPath)

	jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if err != nil {
		return ""
	}

	var newestUUID string
	var newestTime time.Time

	for _, jsonlPath := range jsonlFiles {
		sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
		if !isValidUUID(sessionID) {
			continue
		}

		info, err := os.Stat(jsonlPath)
		if err != nil {
			continue
		}

		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newestUUID = sessionID
		}
	}

	return newestUUID
}

// DiscoverSessions scans ~/.claude/projects for Claude Code sessions
func DiscoverSessions() ([]*Session, error) {
	projectsDir := ClaudeProjectsDir()

	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil // No projects directory yet
	}

	var sessions []*Session

	// Scan project directories
	projectEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	for _, projectEntry := range projectEntries {
		if !projectEntry.IsDir() {
			continue
		}

		projectPath := DecodeProjectPath(projectEntry.Name())
		projectDir := filepath.Join(projectsDir, projectEntry.Name())

		// Scan for JSONL files (each is a session)
		jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil {
			continue
		}

		for _, jsonlPath := range jsonlFiles {
			sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")

			// Skip non-UUID filenames (like .history.jsonl)
			if !isValidUUID(sessionID) {
				continue
			}

			// Get file info for timestamps
			info, err := os.Stat(jsonlPath)
			if err != nil {
				continue
			}

			// Get session info from JSONL (cwd and content check)
			fileInfo := GetSessionFileInfo(jsonlPath)

			actualPath := fileInfo.cwd
			if actualPath == "" {
				actualPath = projectPath // Fallback to decoded path
			}

			session := &Session{
				ID:              sessionID, // Use ClaudeSessionID directly as the unique ID
				Name:            formatSessionName(actualPath, info.ModTime()),
				ProjectPath:     actualPath,
				ClaudeSessionID: sessionID,
				JSONLPath:       jsonlPath,
				CreatedAt:       info.ModTime(), // Use mtime as approximation
				LastAccessedAt:  info.ModTime(),
				GitBranch:       fileInfo.gitBranch, // From Claude's JSONL
			}

			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// isValidUUID checks if a string looks like a UUID
func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// formatSessionName creates a default name for a session
func formatSessionName(projectPath string, modTime time.Time) string {
	// Use the project directory name + date
	base := filepath.Base(projectPath)
	if base == "" || base == "." || base == "/" {
		base = "root"
	}
	return base + " " + modTime.Format("Jan 2 15:04")
}

// RefreshSession updates runtime information for a session
func RefreshSession(s *Session) error {
	if s.JSONLPath == "" {
		return nil
	}

	info, err := os.Stat(s.JSONLPath)
	if err != nil {
		return err
	}

	s.LastAccessedAt = info.ModTime()
	return nil
}
