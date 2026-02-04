package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// StorageDir returns the path to our metadata directory
func StorageDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-sessions")
}

// StorageFile returns the path to the sessions metadata file
func StorageFile() string {
	return filepath.Join(StorageDir(), "sessions.json")
}

// Settings represents user preferences
type Settings struct {
	PreferredTerminal  string `json:"preferred_terminal"`   // "Auto", "iTerm2", "Ghostty", "Kitty", "Terminal"
	ActiveExpanded     *bool  `json:"active_expanded,omitempty"`   // Active group expanded state
	InactiveExpanded   *bool  `json:"inactive_expanded,omitempty"` // Inactive group expanded state
	Theme              string `json:"theme,omitempty"`             // Color theme name
}

// StorageData represents the persisted data structure
type StorageData struct {
	Sessions []*Session `json:"sessions"`
	Groups   []*Group   `json:"groups"`
	Settings *Settings  `json:"settings,omitempty"`
}

// LoadStorage loads the persisted session metadata
func LoadStorage() (*StorageData, error) {
	data := &StorageData{
		Sessions: make([]*Session, 0),
		Groups:   make([]*Group, 0),
		Settings: &Settings{PreferredTerminal: "Auto"},
	}

	file := StorageFile()
	content, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return data, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(content, data); err != nil {
		return nil, err
	}

	// Ensure settings exists
	if data.Settings == nil {
		data.Settings = &Settings{PreferredTerminal: "Auto"}
	}

	return data, nil
}

// SaveStorage persists the session metadata
func SaveStorage(data *StorageData) error {
	dir := StorageDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(StorageFile(), content, 0644)
}

// MergeSessions combines discovered sessions with stored metadata
func MergeSessions(discovered []*Session, stored *StorageData) []*Session {
	// Build a map of stored sessions by Claude session ID + project path
	storedMap := make(map[string]*Session)
	for _, s := range stored.Sessions {
		key := s.ClaudeSessionID + ":" + s.ProjectPath
		storedMap[key] = s
	}

	// Find min order so new sessions appear at top
	minOrder := 1
	for _, s := range stored.Sessions {
		if s.Order < minOrder {
			minOrder = s.Order
		}
	}

	// Merge discovered sessions with stored metadata
	var result []*Session
	var newSessions []*Session
	for _, d := range discovered {
		key := d.ClaudeSessionID + ":" + d.ProjectPath
		if s, ok := storedMap[key]; ok {
			// Use stored metadata but update runtime fields
			s.JSONLPath = d.JSONLPath
			s.LastAccessedAt = d.LastAccessedAt
			s.GitBranch = d.GitBranch
			result = append(result, s)
		} else {
			// New session discovered - collect for ordering
			newSessions = append(newSessions, d)
		}
	}

	// Sort new sessions by last accessed (most recent first) and assign low order numbers
	sort.Slice(newSessions, func(i, j int) bool {
		return newSessions[i].LastAccessedAt.After(newSessions[j].LastAccessedAt)
	})
	for i, s := range newSessions {
		s.Order = minOrder - len(newSessions) + i
		result = append(result, s)
	}

	// Ensure all sessions have unique, non-zero Order values
	// This fixes legacy sessions that all have Order=0
	needsReorder := false
	orderUsed := make(map[int]bool)
	for _, s := range result {
		if s.Order == 0 || orderUsed[s.Order] {
			needsReorder = true
			break
		}
		orderUsed[s.Order] = true
	}

	if needsReorder {
		// Sort by last accessed time (most recent first)
		sort.Slice(result, func(i, j int) bool {
			return result[i].LastAccessedAt.After(result[j].LastAccessedAt)
		})
		// Assign sequential Order values
		for i, s := range result {
			s.Order = i + 1
		}
	}

	return result
}

// Manager handles session and group operations
type Manager struct {
	Sessions []*Session
	Groups   []*Group
	Settings *Settings
	storage  *StorageData
}

// NewManager creates a new session manager
func NewManager() (*Manager, error) {
	m := &Manager{}
	if err := m.Load(); err != nil {
		return nil, err
	}
	return m, nil
}

// Load discovers sessions and loads metadata
func (m *Manager) Load() error {
	// Load stored metadata
	stored, err := LoadStorage()
	if err != nil {
		return err
	}
	m.storage = stored
	m.Groups = stored.Groups
	m.Settings = stored.Settings

	// Capture original Order values BEFORE merge (since merge modifies in place)
	originalOrders := make(map[string]int)
	for _, s := range stored.Sessions {
		key := s.ClaudeSessionID + ":" + s.ProjectPath
		originalOrders[key] = s.Order
	}

	// Check if we had sessions before (to detect new ones)
	hadSessions := len(stored.Sessions)

	// Discover sessions from Claude's data
	discovered, err := DiscoverSessions()
	if err != nil {
		return err
	}

	// Merge with stored metadata
	m.Sessions = MergeSessions(discovered, stored)

	// Auto-save if Order values were updated or new sessions discovered
	needsSave := len(m.Sessions) != hadSessions
	if !needsSave {
		for _, s := range m.Sessions {
			key := s.ClaudeSessionID + ":" + s.ProjectPath
			if orig, ok := originalOrders[key]; !ok || orig != s.Order {
				needsSave = true
				break
			}
		}
	}

	if needsSave {
		m.Save()
	}

	return nil
}

// Save persists the current state
func (m *Manager) Save() error {
	data := &StorageData{
		Sessions: m.Sessions,
		Groups:   m.Groups,
		Settings: m.Settings,
	}
	return SaveStorage(data)
}

// GetPreferredTerminal returns the preferred terminal setting
func (m *Manager) GetPreferredTerminal() string {
	if m.Settings == nil {
		return "Auto"
	}
	return m.Settings.PreferredTerminal
}

// SetPreferredTerminal updates the preferred terminal setting
func (m *Manager) SetPreferredTerminal(terminal string) error {
	if m.Settings == nil {
		m.Settings = &Settings{}
	}
	m.Settings.PreferredTerminal = terminal
	return m.Save()
}

// GetTheme returns the theme setting
func (m *Manager) GetTheme() string {
	if m.Settings == nil || m.Settings.Theme == "" {
		return "Catppuccin Mocha"
	}
	return m.Settings.Theme
}

// SetTheme updates the theme setting
func (m *Manager) SetTheme(theme string) error {
	if m.Settings == nil {
		m.Settings = &Settings{}
	}
	m.Settings.Theme = theme
	return m.Save()
}

// GetActiveExpanded returns the Active group expanded state (default true)
func (m *Manager) GetActiveExpanded() bool {
	if m.Settings == nil || m.Settings.ActiveExpanded == nil {
		return true
	}
	return *m.Settings.ActiveExpanded
}

// SetActiveExpanded updates the Active group expanded state
func (m *Manager) SetActiveExpanded(expanded bool) {
	if m.Settings == nil {
		m.Settings = &Settings{}
	}
	m.Settings.ActiveExpanded = &expanded
}

// GetInactiveExpanded returns the Inactive group expanded state (default true)
func (m *Manager) GetInactiveExpanded() bool {
	if m.Settings == nil || m.Settings.InactiveExpanded == nil {
		return true
	}
	return *m.Settings.InactiveExpanded
}

// SetInactiveExpanded updates the Inactive group expanded state
func (m *Manager) SetInactiveExpanded(expanded bool) {
	if m.Settings == nil {
		m.Settings = &Settings{}
	}
	m.Settings.InactiveExpanded = &expanded
}

// FindSession finds a session by ID
func (m *Manager) FindSession(id string) *Session {
	for _, s := range m.Sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// FindGroup finds a group by ID
func (m *Manager) FindGroup(id string) *Group {
	for _, g := range m.Groups {
		if g.ID == id {
			return g
		}
	}
	return nil
}

// RenameSession updates a session's name
func (m *Manager) RenameSession(id, name string) error {
	s := m.FindSession(id)
	if s == nil {
		return nil
	}
	s.Name = name
	s.Renamed = true // Mark as manually renamed so tab title sync doesn't override
	return m.Save()
}

// TogglePin toggles a session's pinned state
func (m *Manager) TogglePin(id string) error {
	s := m.FindSession(id)
	if s == nil {
		return nil
	}
	s.Pinned = !s.Pinned
	return m.Save()
}

// DeleteSession removes a session from our metadata (doesn't delete Claude's data)
func (m *Manager) DeleteSession(id string) error {
	for i, s := range m.Sessions {
		if s.ID == id {
			m.Sessions = append(m.Sessions[:i], m.Sessions[i+1:]...)
			return m.Save()
		}
	}
	return nil
}

// MoveSession moves a session to a group
func (m *Manager) MoveSession(sessionID, groupPath string) error {
	s := m.FindSession(sessionID)
	if s == nil {
		return nil
	}
	s.GroupPath = groupPath
	return m.Save()
}

// SwapSessions swaps the position (order and group) of two sessions
func (m *Manager) SwapSessions(sessionID1, sessionID2 string) error {
	s1 := m.FindSession(sessionID1)
	s2 := m.FindSession(sessionID2)
	if s1 == nil || s2 == nil {
		return nil
	}
	// Swap group and order
	s1.GroupPath, s2.GroupPath = s2.GroupPath, s1.GroupPath
	s1.Order, s2.Order = s2.Order, s1.Order
	return m.Save()
}

// CreateGroup creates a new group
func (m *Manager) CreateGroup(name, parentPath string) (*Group, error) {
	path := name
	if parentPath != "" {
		path = parentPath + "/" + name
	}

	g := &Group{
		ID:       generateGroupID(),
		Name:     name,
		Path:     path,
		Order:    len(m.Groups),
		Expanded: true,
	}
	m.Groups = append(m.Groups, g)
	return g, m.Save()
}

// RenameGroup updates a group's name
func (m *Manager) RenameGroup(id, name string) error {
	g := m.FindGroup(id)
	if g == nil {
		return nil
	}
	g.Name = name
	// Update path (simple case - just update the name part)
	// For nested groups this would need more work
	g.Path = name
	return m.Save()
}

// DeleteGroup removes a group and moves its sessions to root
func (m *Manager) DeleteGroup(id string) error {
	g := m.FindGroup(id)
	if g == nil {
		return nil
	}

	// Move sessions out of this group
	for _, s := range m.Sessions {
		if s.GroupPath == g.Path {
			s.GroupPath = ""
		}
	}

	// Remove the group
	for i, grp := range m.Groups {
		if grp.ID == id {
			m.Groups = append(m.Groups[:i], m.Groups[i+1:]...)
			break
		}
	}

	return m.Save()
}

// ToggleGroupExpanded toggles a group's expanded state
func (m *Manager) ToggleGroupExpanded(id string) error {
	g := m.FindGroup(id)
	if g == nil {
		return nil
	}
	g.Expanded = !g.Expanded
	return m.Save()
}

// SessionsInGroup returns sessions belonging to a specific group, sorted by Order
func (m *Manager) SessionsInGroup(groupPath string) []*Session {
	var result []*Session
	for _, s := range m.Sessions {
		if s.GroupPath == groupPath {
			result = append(result, s)
		}
	}
	// Sort by Order (stable position unless manually moved)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Order < result[j].Order
	})
	return result
}

// UngroupedSessions returns sessions not in any group (or in a deleted group), sorted by Order
func (m *Manager) UngroupedSessions() []*Session {
	// Build set of valid group paths
	validGroups := make(map[string]bool)
	for _, g := range m.Groups {
		validGroups[g.Path] = true
	}

	var result []*Session
	for _, s := range m.Sessions {
		// Include if no group or group doesn't exist (orphaned)
		if s.GroupPath == "" || !validGroups[s.GroupPath] {
			result = append(result, s)
		}
	}
	// Sort by Order (stable position unless manually moved)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Order < result[j].Order
	})
	return result
}

func generateGroupID() string {
	return "grp-" + randomShortID()
}

func randomShortID() string {
	// Simple random ID generation
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[int(os.Getpid()+i*17)%len(chars)]
	}
	return string(b)
}
