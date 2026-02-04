package session

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// kittyWindow represents a window in kitty's JSON output
type kittyWindow struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	Cmdline []string `json:"cmdline"`
	Cwd     string   `json:"cwd"`
}

// kittyTab represents a tab in kitty's JSON output
type kittyTab struct {
	Title   string        `json:"title"`
	Windows []kittyWindow `json:"windows"`
}

// kittyOSWindow represents an OS window in kitty's JSON output
type kittyOSWindow struct {
	Tabs []kittyTab `json:"tabs"`
}

// activeSession tracks info about an active session in kitty
type activeSession struct {
	windowID    int    // unique ID from kitty
	sessionID   string // from --resume flag, if present
	projectPath string // cwd of the window
	tabTitle    string // title of the tab (for name sync)
	hasSpinner  bool   // true if title has spinner (⠂⠄⠆⠇⠃⠁) indicating active work
}

// StatusUpdate holds the computed status and name for a session
type StatusUpdate struct {
	SessionID     string
	Status        Status
	OldStatus     Status // for change detection
	Name          string // non-empty if name should be updated
	OldName       string // for change detection
	KittyWindowID int    // matched window ID (for reliable future matching)
}

// ComputeStatuses computes status updates without modifying sessions (thread-safe)
// Returns (updates, namesChanged, anyChanged)
func ComputeStatuses(sessions []*Session) ([]StatusUpdate, bool, bool) {
	var updates []StatusUpdate
	namesChanged := false
	anyChanged := false

	// Get active kitty sessions
	activeSessions := getKittyActiveSessions()

	// Sort sessions by JSONL modification time (most recent first)
	type sessionWithMtime struct {
		session *Session
		mtime   time.Time
	}
	sortable := make([]sessionWithMtime, len(sessions))
	for i, s := range sessions {
		var mtime time.Time
		if s.JSONLPath != "" {
			if info, err := os.Stat(s.JSONLPath); err == nil {
				mtime = info.ModTime()
			}
		}
		sortable[i] = sessionWithMtime{session: s, mtime: mtime}
	}

	// Sort by mtime descending (most recent first)
	for i := 0; i < len(sortable)-1; i++ {
		for j := i + 1; j < len(sortable); j++ {
			if sortable[j].mtime.After(sortable[i].mtime) {
				sortable[i], sortable[j] = sortable[j], sortable[i]
			}
		}
	}

	// Track which project paths have been matched to sessions
	matchedWindows := make(map[int]bool)

	// Process in order of recency so most recent session gets matched first
	for _, sw := range sortable {
		status, tabTitle, windowID, strongMatch := detectSessionStatus(sw.session, activeSessions, matchedWindows)

		update := StatusUpdate{
			SessionID:     sw.session.ClaudeSessionID,
			Status:        status,
			OldStatus:     sw.session.Status,
			OldName:       sw.session.Name,
			KittyWindowID: windowID,
		}

		// Track if status changed
		if status != sw.session.Status {
			anyChanged = true
		}

		// Track if window ID changed (for reliable future matching)
		// Only store window ID for strong matches to avoid polluting data
		if strongMatch && windowID != sw.session.KittyWindowID {
			anyChanged = true
		} else if !strongMatch {
			// Don't store window ID for weak (path-only) matches
			update.KittyWindowID = sw.session.KittyWindowID // Keep existing
		}

		// Compute name update from tab title ONLY for strong matches
		// Path-only matches are unreliable - could be wrong session
		if strongMatch && tabTitle != "" && !sw.session.Renamed {
			cleanTitle := cleanClaudeTitle(tabTitle)
			if cleanTitle != "" && cleanTitle != sw.session.Name {
				update.Name = cleanTitle
				namesChanged = true
				anyChanged = true
			}
		}

		updates = append(updates, update)
	}

	return updates, namesChanged, anyChanged
}

// ApplyStatusUpdates applies computed updates to sessions (must be called from main thread)
// Returns (changed, needsSave) where needsSave is true if persisted fields (name, windowID) changed
func ApplyStatusUpdates(sessions []*Session, updates []StatusUpdate) (bool, bool) {
	changed := false
	needsSave := false

	// Build map for quick lookup
	updateMap := make(map[string]StatusUpdate)
	for _, u := range updates {
		updateMap[u.SessionID] = u
	}

	// First pass: collect which window IDs are being claimed
	claimedWindows := make(map[int]string) // windowID -> sessionID that's claiming it
	for _, u := range updates {
		if u.KittyWindowID > 0 {
			claimedWindows[u.KittyWindowID] = u.SessionID
		}
	}

	for _, s := range sessions {
		if u, ok := updateMap[s.ClaudeSessionID]; ok {
			if s.Status != u.Status {
				s.Status = u.Status
				changed = true
			}
			if u.Name != "" && s.Name != u.Name {
				s.Name = u.Name
				changed = true
				needsSave = true
			}
			// Store matched window ID for reliable future matching
			if u.KittyWindowID != s.KittyWindowID {
				s.KittyWindowID = u.KittyWindowID
				changed = true
				needsSave = true
			}
		} else {
			// Session not in updates - check if its stored window ID was claimed by another session
			if s.KittyWindowID > 0 {
				if claimer, claimed := claimedWindows[s.KittyWindowID]; claimed && claimer != s.ClaudeSessionID {
					// Another session now owns this window - clear our stale reference
					s.KittyWindowID = 0
					changed = true
					needsSave = true
				}
			}
		}
	}

	return changed, needsSave
}

// RefreshStatuses updates status for all sessions (convenience wrapper)
// Returns true if any persisted fields were updated (caller should save)
func RefreshStatuses(sessions []*Session) bool {
	updates, _, _ := ComputeStatuses(sessions)
	_, needsSave := ApplyStatusUpdates(sessions, updates)
	return needsSave
}

// ClaimWindowID sets the window ID for a session and clears it from any other sessions
// that had it stored. Returns true if any other session was modified.
func ClaimWindowID(sessions []*Session, claimingSession *Session, windowID int) bool {
	if windowID <= 0 {
		return false
	}
	modified := false
	for _, s := range sessions {
		if s == claimingSession {
			continue
		}
		if s.KittyWindowID == windowID {
			s.KittyWindowID = 0
			modified = true
		}
	}
	claimingSession.KittyWindowID = windowID
	return modified
}

// GetActiveWindowID returns the kitty window ID for a session if it has an active tab
// Returns 0 if no active tab found
func GetActiveWindowID(s *Session) int {
	// First check if we have a stored window ID
	if s.KittyWindowID > 0 {
		// Verify it's still active
		activeSessions := getKittyActiveSessions()
		for _, active := range activeSessions {
			if active.windowID == s.KittyWindowID {
				return s.KittyWindowID
			}
		}
		// Window no longer exists, clear it
		s.KittyWindowID = 0
	}

	activeSessions := getKittyActiveSessions()

	// Check for session ID match
	for _, active := range activeSessions {
		if active.sessionID != "" && active.sessionID == s.ClaudeSessionID {
			return active.windowID
		}
	}

	// Check for project path match (less reliable)
	for _, active := range activeSessions {
		if active.sessionID != "" {
			continue
		}
		pathMatch := active.projectPath == s.ProjectPath ||
			strings.HasPrefix(active.projectPath, s.ProjectPath+"/")
		if pathMatch {
			return active.windowID
		}
	}

	return 0
}

// hasClaudeIndicator checks if a title has Claude's status indicators
func hasClaudeIndicator(title string) bool {
	if title == "" {
		return false
	}
	// Claude uses: ✳ (unsaved changes), ⠂⠄⠆⠇⠃⠁ (spinner animation)
	r := []rune(title)[0]
	return r == '✳' || (r >= '⠀' && r <= '⠿') // Braille pattern range
}

// hasSpinnerIndicator checks if a title has Claude's spinner (active work)
func hasSpinnerIndicator(title string) bool {
	if title == "" {
		return false
	}
	r := []rune(title)[0]
	return r >= '⠀' && r <= '⠿' // Braille pattern range (spinner)
}

// cleanClaudeTitle removes Claude's status indicators from a title
func cleanClaudeTitle(title string) string {
	if title == "" {
		return ""
	}
	runes := []rune(title)
	// Skip leading indicator character if present
	if len(runes) > 0 {
		r := runes[0]
		if r == '✳' || (r >= '⠀' && r <= '⠿') {
			title = string(runes[1:])
		}
	}
	return strings.TrimSpace(title)
}

// getKittyActiveSessions returns info about active claude sessions in kitty
func getKittyActiveSessions() []activeSession {
	var result []activeSession

	// Try kitty @ ls
	cmd := exec.Command("kitty", "@", "ls")
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	var osWindows []kittyOSWindow
	if err := json.Unmarshal(output, &osWindows); err != nil {
		return result
	}

	// Look for claude processes
	for _, osWin := range osWindows {
		for _, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				cmdline := strings.Join(win.Cmdline, " ")

				// Check if this window is running claude:
				// - cmdline contains "claude", OR
				// - tab/window title has Claude indicator (✳ for unsaved, ⠂⠄⠆⠇⠃⠁ for spinner)
				isClaudeTab := strings.Contains(cmdline, "claude") ||
					hasClaudeIndicator(tab.Title) ||
					hasClaudeIndicator(win.Title)

				if !isClaudeTab {
					continue
				}

				// Use window title for name (more accurate than tab title)
				title := win.Title
				if title == "" {
					title = tab.Title
				}

				active := activeSession{
					windowID:    win.ID,
					projectPath: win.Cwd,
					tabTitle:    title,
					hasSpinner:  hasSpinnerIndicator(title),
				}

				// Extract session ID from --resume flag
				// Could be direct arg or inside a zsh -c '...' string
				if idx := strings.Index(cmdline, "--resume "); idx != -1 {
					rest := cmdline[idx+9:] // skip "--resume "
					// Extract the session ID (UUID format: 8-4-4-4-12 hex chars)
					// Stop at any non-UUID character
					sessionID := ""
					for _, c := range rest {
						if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-' {
							sessionID += string(c)
						} else {
							break
						}
					}
					if sessionID != "" {
						active.sessionID = sessionID
					}
				}

				result = append(result, active)
			}
		}
	}

	// Sort tabs with spinners first (actively working Claude) to match most recent sessions
	sort.Slice(result, func(i, j int) bool {
		if result[i].hasSpinner != result[j].hasSpinner {
			return result[i].hasSpinner // spinner first
		}
		return false // maintain order otherwise
	})

	return result
}

// detectSessionStatus determines the status of a session
// Returns (status, tabTitle, windowID, strongMatch) where:
// - tabTitle is non-empty if matched to an active tab
// - windowID is the matched kitty window ID (for storing back on session)
// - strongMatch is true if we're confident this is the right tab (--resume or stored window ID)
func detectSessionStatus(s *Session, activeSessions []activeSession, matchedWindows map[int]bool) (Status, string, int, bool) {
	// Check if session has an active kitty tab
	hasKittyTab := false
	tabTitle := ""
	matchedWindowID := 0
	strongMatch := false // true if we're confident about the match (--resume or stored window ID)

	// 1. First, try to match by stored KittyWindowID (for sessions opened via claude-deck)
	// BUT only if the tab doesn't have a --resume flag pointing to a different session
	// AND the window isn't already matched to another session
	if s.KittyWindowID > 0 {
		for _, active := range activeSessions {
			if active.windowID == s.KittyWindowID {
				// If window already matched to another session, skip (our stored ID is stale)
				if matchedWindows[active.windowID] {
					break
				}
				// If tab has explicit session ID pointing to a different session, don't match
				// (the other session will match this tab via method 2)
				if active.sessionID != "" && active.sessionID != s.ClaudeSessionID {
					break // Window was reused for different session
				}
				hasKittyTab = true
				tabTitle = active.tabTitle
				matchedWindowID = active.windowID
				strongMatch = true // We trust stored window IDs
				matchedWindows[active.windowID] = true
				break
			}
		}
	}

	// 2. Then try to match by session ID from --resume flag (most reliable - ground truth)
	if !hasKittyTab {
		for _, active := range activeSessions {
			if active.sessionID != "" && active.sessionID == s.ClaudeSessionID {
				// Skip if already matched to another session (shouldn't happen, but safety check)
				if matchedWindows[active.windowID] {
					continue
				}
				hasKittyTab = true
				tabTitle = active.tabTitle
				matchedWindowID = active.windowID
				strongMatch = true // --resume is ground truth
				matchedWindows[active.windowID] = true
				break
			}
		}
	}

	// 3. For sessions without direct match, check by project path (WEAK match - don't sync names)
	if !hasKittyTab && s.JSONLPath != "" {
		for _, active := range activeSessions {
			if active.sessionID != "" {
				continue // Skip tabs that have explicit session IDs
			}
			if matchedWindows[active.windowID] {
				continue // Already assigned to another session
			}

			// Match if tab cwd equals session project path, OR
			// tab cwd is under session project path (user cd'd into subdirectory)
			pathMatch := active.projectPath == s.ProjectPath ||
				strings.HasPrefix(active.projectPath, s.ProjectPath+"/")
			if !pathMatch {
				continue
			}

			// Found an unassigned tab at/under this project path
			// strongMatch stays false - path matching is unreliable
			matchedWindows[active.windowID] = true
			matchedWindowID = active.windowID
			hasKittyTab = true
			tabTitle = active.tabTitle
			break
		}
	}

	if !hasKittyTab {
		return StatusIdle, "", 0, false
	}

	// Session has active tab - check JSONL for activity
	if s.JSONLPath == "" {
		return StatusWaiting, tabTitle, matchedWindowID, strongMatch
	}

	info, err := os.Stat(s.JSONLPath)
	if err != nil {
		return StatusWaiting, tabTitle, matchedWindowID, strongMatch
	}

	age := time.Since(info.ModTime())

	// Recent file activity = running (Claude is actively working)
	if age < 10*time.Second {
		return StatusRunning, tabTitle, matchedWindowID, strongMatch
	}

	// No recent activity - check if waiting for user input
	// (tool approval, question, etc.)
	return StatusWaiting, tabTitle, matchedWindowID, strongMatch
}

// needsUserInput checks if the session is waiting for user input
// by examining the last few entries in the JSONL file
func needsUserInput(jsonlPath string) bool {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read last 50KB of file to find recent messages
	const tailSize = 50 * 1024
	info, err := file.Stat()
	if err != nil {
		return false
	}

	start := info.Size() - tailSize
	if start < 0 {
		start = 0
	}

	file.Seek(start, 0)
	data, err := io.ReadAll(file)
	if err != nil {
		return false
	}

	content := string(data)

	// Find the last complete JSON line
	lines := strings.Split(content, "\n")

	// Look at the last few lines for indicators
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-10; i-- {
		line := lines[i]
		if line == "" {
			continue
		}

		// Check for tool use requiring approval
		if strings.Contains(line, `"type":"tool_use"`) &&
		   !strings.Contains(line, `"tool_result"`) {
			// Has tool_use but no result yet - might be waiting for approval
			return true
		}

		// Check for assistant asking a question (ends with ?)
		if strings.Contains(line, `"role":"assistant"`) &&
		   strings.Contains(line, `"type":"text"`) {
			// This is an assistant text message - check if it's asking something
			if strings.Contains(line, "?\"") || strings.Contains(line, "?\n") {
				return true
			}
		}

		// Found a user message - not waiting for input
		if strings.Contains(line, `"role":"user"`) {
			return false
		}
	}

	return false
}
