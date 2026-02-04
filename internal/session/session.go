package session

import (
	"time"
)

// Status represents the current state of a Claude session
type Status int

const (
	StatusIdle Status = iota
	StatusWaiting
	StatusRunning
)

func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusWaiting:
		return "waiting"
	default:
		return "idle"
	}
}

// Symbol returns the status indicator symbol
func (s Status) Symbol() string {
	switch s {
	case StatusRunning:
		return "●"
	case StatusWaiting:
		return "◎"
	default:
		return "○"
	}
}

// Session represents a Claude Code session
type Session struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Renamed         bool      `json:"renamed,omitempty"` // True if user manually renamed
	ProjectPath     string    `json:"project_path"`
	ClaudeSessionID string    `json:"claude_session_id"`
	GroupPath       string    `json:"group_path"`
	Order           int       `json:"order"`
	Pinned          bool      `json:"pinned"`
	CreatedAt       time.Time `json:"created_at"`
	LastAccessedAt  time.Time `json:"last_accessed_at"`
	KittyWindowID   int       `json:"kitty_window_id,omitempty"` // Window ID when opened in kitty

	// Runtime fields (not persisted)
	Status       Status `json:"-"`
	JSONLPath    string `json:"-"`
	MessageCount int    `json:"-"`
	Title        string `json:"-"` // Extracted from first user message

	// Git info (from Claude's JSONL - branch at time of session)
	GitBranch string `json:"-"`
}

// FolderName returns just the folder name from ProjectPath
func (s *Session) FolderName() string {
	if s.ProjectPath == "" {
		return ""
	}
	// Get last component of path
	parts := splitPath(s.ProjectPath)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return s.ProjectPath
}

func splitPath(path string) []string {
	var result []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// Group represents a folder/group for organizing sessions
type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Order    int    `json:"order"`
	Expanded bool   `json:"expanded"`
}

// ListItem is an interface for items that can appear in the session list
type ListItem interface {
	ItemID() string
	ItemName() string
	IsGroup() bool
}

func (s *Session) ItemID() string   { return s.ID }
func (s *Session) ItemName() string { return s.Name }
func (s *Session) IsGroup() bool    { return false }

func (g *Group) ItemID() string   { return g.ID }
func (g *Group) ItemName() string { return g.Name }
func (g *Group) IsGroup() bool    { return true }

// PreviewMessage represents a simplified message for display
type PreviewMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
}
