package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/hadar/claude-deck/internal/session"
	"github.com/hadar/claude-deck/internal/terminal"
)

// Focus indicates which pane has focus
type Focus int

const (
	FocusList Focus = iota
	FocusPreview
)

// Layout constants
const (
	headerHeight = 1
	statusHeight = 1
	listWidthPct = 40
)

// App is the main Bubble Tea model
type App struct {
	manager *session.Manager
	list    *ListModel
	preview *PreviewModel
	search  *SearchModel
	dialog  *DialogModel
	keys    ListKeyMap

	focus      Focus
	width      int
	height     int
	quitting   bool
	loading    bool // true while loading sessions
	err        error
	statusMsg  string
	horizontal bool // true = side by side, false = stacked

	// Layout measurements
	listWidth     int
	previewWidth  int
	contentHeight int
	listStartY    int // Y offset where list content starts (for mouse)

	watcher   *fsnotify.Watcher
	showHelp  bool // true when help overlay is visible
	showTheme bool // true when theme selection overlay is visible
	themeCursor  int  // cursor position in theme selection

	// New session dialog
	showNewSession        bool     // true when new session dialog is visible
	newSessionPaths       []string // list of paths to show (favorites + recent)
	newSessionCursor      int      // cursor position in paths list
	newSessionFocus       int      // 0=name, 1=path, 2=list
	newSessionName        string   // session name input
	newSessionNameCursor int    // cursor in name field
	newSessionPath       string // path input
	newSessionPathCursor int    // cursor in path field

	// Pending new session - waiting to be matched by window ID or file watcher
	pendingRenamePath    string
	pendingRenameName    string
	pendingRenameWindowID int // kitty window ID for matching

	// Skip next status save to avoid race condition with rename
	skipNextStatusSave bool
}

// NewApp creates a new application instance
func NewApp() (*App, error) {
	// Create empty manager - will load data async
	manager := &session.Manager{}

	app := &App{
		manager:    manager,
		list:       NewListModel(manager),
		preview:    NewPreviewModel(),
		search:     NewSearchModel(),
		dialog:     NewDialogModel(),
		keys:       DefaultListKeyMap(),
		focus:      FocusList,
		horizontal: true,
		loading:    true,
	}

	// Setup file watcher for live preview updates
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		app.watcher = watcher
	}

	return app, nil
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	// Set terminal title and load sessions
	return tea.Batch(
		tea.SetWindowTitle("Claude Deck"),
		a.loadSessions(),
	)
}

// loadSessions loads session data in the background
func (a *App) loadSessions() tea.Cmd {
	return func() tea.Msg {
		err := a.manager.Load()
		if err == nil {
			// Use aggressive mode on startup to sync names even for path-only matches
			if session.RefreshStatusesAggressive(a.manager.Sessions) {
				a.manager.Save() // Persist tab title names and window IDs
			}
			// Refresh git branches (one git call per unique path)
			session.RefreshGitBranches(a.manager.Sessions)
		}
		return sessionsLoadedMsg{err: err}
	}
}

type statusRefreshedMsg struct {
	updates         []session.StatusUpdate
	activeWindowIDs map[int]bool // all active kitty window IDs
	noChanges       bool         // true if nothing changed - skip re-render
}
type clearStatusMsg struct{}
type sessionsLoadedMsg struct {
	err error
}

// setStatus sets a status message and returns a command to clear it after 2 seconds
func (a *App) setStatus(msg string) tea.Cmd {
	a.statusMsg = msg
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// refreshStatusesAsync computes status updates asynchronously
func (a *App) refreshStatusesAsync() tea.Cmd {
	sessions := a.manager.Sessions
	return func() tea.Msg {
		updates, activeWindowIDs, _, anyChanged := session.ComputeStatuses(sessions)
		return statusRefreshedMsg{updates: updates, activeWindowIDs: activeWindowIDs, noChanges: !anyChanged}
	}
}

// watchFiles sets up file watching for session changes
func (a *App) watchFiles() tea.Cmd {
	return func() tea.Msg {
		projectsDir := session.ClaudeProjectsDir()

		// Watch the top-level projects directory for new project dirs
		if err := a.watcher.Add(projectsDir); err != nil {
			return nil
		}

		// Watch all existing project subdirectories for new/changed JSONL files
		entries, err := os.ReadDir(projectsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					subdir := filepath.Join(projectsDir, entry.Name())
					a.watcher.Add(subdir)
				}
			}
		}

		for {
			select {
			case event, ok := <-a.watcher.Events:
				if !ok {
					return nil
				}
				// New JSONL file created = new session
				if event.Op&fsnotify.Create == fsnotify.Create && strings.HasSuffix(event.Name, ".jsonl") {
					return newSessionFileMsg{path: event.Name}
				}
				// Existing JSONL modified = refresh preview
				if event.Op&fsnotify.Write == fsnotify.Write && strings.HasSuffix(event.Name, ".jsonl") {
					return fileChangedMsg{path: event.Name}
				}
				// New project directory created = watch it
				if event.Op&fsnotify.Create == fsnotify.Create {
					info, err := os.Stat(event.Name)
					if err == nil && info.IsDir() {
						a.watcher.Add(event.Name)
					}
				}
			case _, ok := <-a.watcher.Errors:
				if !ok {
					return nil
				}
			}
		}
	}
}

type fileChangedMsg struct {
	path string
}

type newSessionFileMsg struct {
	path string
}

// Update handles messages
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keys first
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Quit on Ctrl+C or q (when not in dialog/search)
		if msg.String() == "ctrl+c" {
			a.quitting = true
			if a.watcher != nil {
				a.watcher.Close()
			}
			return a, tea.Quit
		}

		if msg.String() == "Q" && !a.dialog.IsOpen() && !a.search.IsActive() {
			a.quitting = true
			if a.watcher != nil {
				a.watcher.Close()
			}
			return a, tea.Quit
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateLayout()
		return a, nil

	case tea.FocusMsg:
		// Window gained focus - refresh status and sync tab names
		return a, a.refreshStatusesAsync()

	case tea.MouseMsg:
		return a.handleMouse(msg)

	case statusRefreshedMsg:
		// Skip if nothing changed and no pending session
		if msg.noChanges && a.pendingRenameWindowID == 0 {
			return a, nil
		}

		// Apply updates to session objects (modifies Status field in place)
		// No need to update pointers - sessions haven't been reloaded
		changed, needsSave := session.ApplyStatusUpdates(a.manager.Sessions, msg.updates)

		// Remove pending sessions whose windows no longer exist
		if msg.activeWindowIDs != nil {
			var remainingSessions []*session.Session
			for _, s := range a.manager.Sessions {
				isPending := strings.HasPrefix(s.ClaudeSessionID, "pending-")
				windowGone := s.KittyWindowID > 0 && !msg.activeWindowIDs[s.KittyWindowID]
				if isPending && windowGone {
					// Pending session's window was closed - remove it
					changed = true
					needsSave = true
					continue
				}
				remainingSessions = append(remainingSessions, s)
			}
			a.manager.Sessions = remainingSessions
		}

		// Check for pending new session matched by window ID
		var statusCmd tea.Cmd
		if a.pendingRenameWindowID > 0 {
			// Check if the pending window was closed
			windowStillExists := msg.activeWindowIDs != nil && msg.activeWindowIDs[a.pendingRenameWindowID]
			if !windowStillExists {
				// Window was closed before JSONL was created - clear pending state
				a.pendingRenamePath = ""
				a.pendingRenameName = ""
				a.pendingRenameWindowID = 0
			} else {
				// Try to match to a session
				matched := false
				for _, s := range a.manager.Sessions {
					if s.KittyWindowID == a.pendingRenameWindowID {
						// Found the session - apply pending name
						if a.pendingRenameName != "" && !s.Renamed {
							a.manager.RenameSession(s.ID, a.pendingRenameName)
							statusCmd = a.setStatus("Created: " + a.pendingRenameName)
						}
						changed = true
						needsSave = true
						matched = true
						break
					}
				}
				// Clear pending state if matched (otherwise keep waiting for JSONL)
				if matched {
					a.pendingRenamePath = ""
					a.pendingRenameName = ""
					a.pendingRenameWindowID = 0
				}
			}
		}

		// Track active sessions for resume on startup
		a.trackActiveSessions()

		// Save if persisted fields changed (names, window IDs)
		// But skip if we just did a rename to avoid race condition
		if needsSave && !a.skipNextStatusSave {
			a.manager.Save()
		}
		a.skipNextStatusSave = false

		// Refresh list if any status changed (moves sessions between Active/Inactive)
		if changed {
			a.list.Refresh()
		}

		if statusCmd != nil {
			return a, statusCmd
		}
		return a, nil

	case clearStatusMsg:
		a.statusMsg = ""
		return a, nil

	case sessionsLoadedMsg:
		a.loading = false
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.list.ReloadExpansionState()
		a.list.Refresh()
		// Apply theme from settings
		if theme := a.manager.GetTheme(); theme != "" {
			ApplyTheme(theme)
		}
		// Start other background tasks
		var cmds []tea.Cmd
		if item := a.list.SelectedItem(); item != nil && !item.IsGroup() {
			if cmd := a.preview.SetSession(item.Session); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if a.watcher != nil {
			cmds = append(cmds, a.watchFiles())
		}
		// Restore previously active sessions if enabled
		if cmd := a.restoreSessions(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)


	case fileChangedMsg:
		// JSONL file was modified - trigger status refresh for all sessions
		// This captures activity from any Claude tab
		var cmds []tea.Cmd

		// Restart file watcher (it returns after each event)
		if a.watcher != nil {
			cmds = append(cmds, a.watchFiles())
		}

		// Refresh preview if the changed file is the current session
		if item := a.list.SelectedItem(); item != nil && !item.IsGroup() {
			if item.Session.JSONLPath == msg.path {
				cmds = append(cmds, a.preview.Refresh())
			}
		}

		// Trigger async status refresh
		cmds = append(cmds, a.refreshStatusesAsync())
		return a, tea.Batch(cmds...)

	case newSessionFileMsg:
		// Build commands - always restart watcher
		var cmds []tea.Cmd
		if a.watcher != nil {
			cmds = append(cmds, a.watchFiles())
		}

		statusMsg := "New session discovered"

		// Check if we have a pending new session waiting for this file
		if a.pendingRenameWindowID > 0 {
			// Remove the pending session before reloading
			pendingID := fmt.Sprintf("pending-%d", a.pendingRenameWindowID)
			a.manager.RemovePendingSession(pendingID)
		}

		// Reload sessions to discover new JSONL
		a.manager.Load()

		// Match pending session to discovered one
		if a.pendingRenamePath != "" {
			newProjectPath := session.ProjectPathFromJSONL(msg.path)

			if newProjectPath == a.pendingRenamePath {
				newest := session.FindNewestSessionAtPath(a.pendingRenamePath)
				if newest != "" {
					a.skipNextStatusSave = true

					s := a.manager.FindSession(newest)
					if s != nil {
						// Apply the pending name
						if a.pendingRenameName != "" {
							a.manager.RenameSession(s.ID, a.pendingRenameName)
							statusMsg = "Created: " + a.pendingRenameName
						}
						// Claim the window ID
						s.Status = session.StatusWaiting
						if a.pendingRenameWindowID > 0 {
							session.ClaimWindowID(a.manager.Sessions, s, a.pendingRenameWindowID)
							s.KittyWindowID = a.pendingRenameWindowID
						}
						a.manager.Save()
					}
				}
			}
		}

		// Clear pending state
		a.pendingRenamePath = ""
		a.pendingRenameName = ""
		a.pendingRenameWindowID = 0

		a.list.Refresh()
		cmds = append(cmds, a.setStatus(statusMsg))
		return a, tea.Batch(cmds...)

	case PreviewLoadedMsg:
		a.preview.HandleLoaded(msg)
		return a, nil

	case ContentSearchResultsMsg:
		a.list.HandleContentSearchResults(msg)
		return a, a.updateSelectedPreview()
	}

	// Handle help overlay
	if a.showHelp {
		if msg, ok := msg.(tea.KeyMsg); ok {
			// Any key closes help
			if msg.String() == "esc" || msg.String() == "h" || msg.String() == "enter" || msg.String() == "Q" {
				a.showHelp = false
			}
		}
		return a, nil
	}

	// Handle theme selection overlay
	if a.showTheme {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "esc", "Q":
				a.showTheme = false
			case "enter":
				// Apply selection
				selected := ThemeNames[a.themeCursor]
				a.manager.SetTheme(selected)
				ApplyTheme(selected)
				a.showTheme = false
				return a, a.setStatus("Theme set to " + selected)
			case "up":
				if a.themeCursor > 0 {
					a.themeCursor--
					ApplyTheme(ThemeNames[a.themeCursor]) // Live preview
				}
			case "down":
				if a.themeCursor < len(ThemeNames)-1 {
					a.themeCursor++
					ApplyTheme(ThemeNames[a.themeCursor]) // Live preview
				}
			}
		}
		return a, nil
	}

	// Handle new session dialog
	if a.showNewSession {
		return a.updateNewSessionDialog(msg)
	}

	// Handle dialog if open
	if a.dialog.IsOpen() {
		return a.updateDialog(msg)
	}

	// Handle search if active
	if a.list.IsSearching() {
		return a.updateSearch(msg)
	}

	// Handle content search if active
	if a.list.IsContentSearching() {
		return a.updateContentSearch(msg)
	}

	// Handle in-place rename if active
	if a.list.IsRenaming() {
		return a.updateRename(msg)
	}

	// Handle moving mode
	if a.list.IsMoving() {
		return a.updateMoving(msg)
	}

	// Handle new group creation
	if a.list.IsCreatingGroup() {
		return a.updateNewGroup(msg)
	}

	// Handle delete confirmation
	if a.list.IsDeleting() {
		return a.updateDelete(msg)
	}

	// Handle normal navigation
	return a.updateNormal(msg)
}

// handleMouse processes mouse events
func (a *App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Y offset: header (1) + border (1) + table header (1) = 3
	const listYOffset = 3

	// Handle mouse motion for hover
	if msg.Action == tea.MouseActionMotion {
		if msg.X < a.listWidth+2 {
			listY := msg.Y - listYOffset
			a.list.SetHover(listY)
		} else {
			a.list.ClearHover()
		}
		return a, nil
	}

	// Handle scroll wheel in list panel
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if msg.X < a.listWidth+2 {
			a.list.MoveUp()
			return a, a.updateSelectedPreview()
		}
		return a, nil

	case tea.MouseButtonWheelDown:
		if msg.X < a.listWidth+2 {
			a.list.MoveDown()
			return a, a.updateSelectedPreview()
		}
		return a, nil

	case tea.MouseButtonLeft:
		// Handle click (on press)
		if msg.Action != tea.MouseActionPress {
			return a, nil
		}

		// Click in list panel
		if msg.X < a.listWidth+2 {
			a.focus = FocusList
			listY := msg.Y - listYOffset
			if listY >= 0 && a.list.HandleClick(listY) {
				return a, a.updateSelectedPreview()
			}
		} else {
			// Click in preview panel
			a.focus = FocusPreview
		}
		return a, nil
	}

	return a, nil
}

// updateDialog handles dialog interactions
func (a *App) updateDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return a.handleDialogConfirm()
		case "esc":
			a.dialog.Close()
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.dialog, cmd = a.dialog.Update(msg)
	return a, cmd
}

// handleDialogConfirm processes dialog confirmation
func (a *App) handleDialogConfirm() (tea.Model, tea.Cmd) {
	switch a.dialog.Type() {
	case DialogRename:
		if name := a.dialog.Value(); name != "" {
			a.manager.RenameSession(a.dialog.TargetID(), name)
			// Update kitty tab title if session has active window
			if s := a.manager.FindSession(a.dialog.TargetID()); s != nil {
				if windowID := session.GetActiveWindowID(s); windowID > 0 {
					terminal.SetKittyTabTitle(windowID, name)
				}
			}
			a.list.Refresh()
			a.dialog.Close()
			return a, a.setStatus("Session renamed")
		}

	case DialogRenameGroup:
		if name := a.dialog.Value(); name != "" {
			a.manager.RenameGroup(a.dialog.TargetID(), name)
			a.list.Refresh()
			a.dialog.Close()
			return a, a.setStatus("Group renamed")
		}

	case DialogNewGroup:
		if name := a.dialog.Value(); name != "" {
			a.manager.CreateGroup(name, "")
			a.list.Refresh()
			a.dialog.Close()
			return a, a.setStatus("Group created")
		}

	case DialogDelete:
		if a.dialog.Confirm() {
			// Determine if it's a group or session
			var msg string
			if g := a.manager.FindGroup(a.dialog.TargetID()); g != nil {
				a.manager.DeleteGroup(a.dialog.TargetID())
				msg = "Group deleted"
			} else {
				a.manager.DeleteSession(a.dialog.TargetID())
				msg = "Session removed"
			}
			a.list.Refresh()
			a.dialog.Close()
			return a, a.setStatus(msg)
		}

	case DialogMove:
		if group := a.dialog.SelectedGroup(); group != nil {
			a.manager.MoveSession(a.dialog.TargetID(), group.Path)
			a.list.Refresh()
			a.dialog.Close()
			return a, a.setStatus("Session moved")
		}
	}

	a.dialog.Close()
	return a, nil
}

// updateSearch handles search interactions
func (a *App) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			a.list.ConfirmSearch()
			return a.handleOpen()
		case "esc":
			a.list.CancelSearch()
			return a, nil
		case "up", "down", "ctrl+p", "ctrl+n":
			a.list.HandleSearchKey(msg.String())
			return a, a.updateSelectedPreview()
		default:
			a.list.HandleSearchKey(msg.String())
			return a, nil
		}
	}
	return a, nil
}

// updateContentSearch handles content search interactions
func (a *App) updateContentSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Get selected item BEFORE clearing search results
			item := a.list.SelectedItem()
			a.list.ConfirmContentSearch()
			// Open the captured item
			if item != nil && !item.IsGroup() {
				windowID, err := terminal.OpenSession(item.Session.ProjectPath, item.Session.ClaudeSessionID, session.GetActiveWindowID(item.Session), item.Session.Name)
				if err != nil {
					return a, a.setStatus("Error: " + err.Error())
				}
				// Store window ID for reliable tab matching (and clear from other sessions)
				if windowID > 0 {
					session.ClaimWindowID(a.manager.Sessions, item.Session, windowID)
					a.manager.Save()
				}
				// Immediately mark session as active (will be refined by next status tick)
				if item.Session.Status == session.StatusIdle {
					item.Session.Status = session.StatusWaiting
					a.list.Refresh() // Move to Active group
				}
				return a, a.setStatus("Opened in new tab")
			}
			return a, nil
		case "esc":
			a.list.CancelContentSearch()
			return a, nil
		case "up", "down", "ctrl+p", "ctrl+n":
			a.list.HandleContentSearchKey(msg.String())
			return a, a.updateSelectedPreview()
		default:
			shouldSearch := a.list.HandleContentSearchKey(msg.String())
			if shouldSearch {
				return a, a.list.RunContentSearch()
			}
			return a, nil
		}
	}
	return a, nil
}

// updateRename handles in-place rename
func (a *App) updateRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			id, newName, isGroup := a.list.ConfirmRename()
			if id != "" {
				if isGroup {
					if newName != "" {
						a.manager.RenameGroup(id, newName)
					}
				} else {
					a.manager.RenameSession(id, newName) // empty name resets to dynamic
					// Update kitty tab title if session has active window
					if s := a.manager.FindSession(id); s != nil {
						if windowID := session.GetActiveWindowID(s); windowID > 0 {
							if newName != "" {
								terminal.SetKittyTabTitle(windowID, newName)
							} else {
								// Reset to dynamic window title
								terminal.ResetKittyTabTitle(windowID)
							}
						}
					}
				}
				a.list.Refresh()
				if newName == "" {
					return a, a.setStatus("Reset to dynamic name")
				}
				return a, a.setStatus("Renamed")
			}
			return a, nil
		case "esc":
			a.list.CancelRename()
			return a, nil
		default:
			a.list.HandleRenameKey(msg.String())
			return a, nil
		}
	}
	return a, nil
}

// updateMoving handles moving mode - navigate with arrows, enter to confirm
func (a *App) updateMoving(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			sessionID, groupPath, _ := a.list.ConfirmMove()
			if sessionID != "" && groupPath != "" {
				a.manager.MoveSession(sessionID, groupPath)
				a.list.Refresh()
				return a, a.setStatus("Moved to group")
			}
			return a, nil
		case "esc":
			a.list.CancelMoving()
			a.statusMsg = ""
			return a, nil
		case "up":
			a.list.MoveUp()
			return a, nil
		case "down":
			a.list.MoveDown()
			return a, nil
		}
	}
	return a, nil
}

// updateNewGroup handles new group creation
func (a *App) updateNewGroup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			name := a.list.ConfirmNewGroup()
			if name != "" {
				a.manager.CreateGroup(name, "")
				a.list.Refresh()
				return a, a.setStatus("Group created")
			}
			return a, nil
		case "esc":
			a.list.CancelNewGroup()
			return a, nil
		default:
			a.list.HandleNewGroupKey(msg.String())
			return a, nil
		}
	}
	return a, nil
}

// updateDelete handles delete confirmation (y/n)
func (a *App) updateDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			id, isGroup := a.list.ConfirmDelete()
			if id != "" {
				var msg string
				if isGroup {
					a.manager.DeleteGroup(id)
					msg = "Group deleted"
				} else {
					a.manager.DeleteSession(id)
					msg = "Session removed"
				}
				a.list.Refresh()
				return a, a.setStatus(msg)
			}
			return a, nil
		case "n", "N", "esc":
			a.list.CancelDelete()
			return a, nil
		}
	}
	return a, nil
}

// updateNormal handles normal navigation
func (a *App) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, a.keys.ShiftUp):
			a.list.MoveUpFast()
			return a, a.updateSelectedPreview()

		case key.Matches(msg, a.keys.ShiftDown):
			a.list.MoveDownFast()
			return a, a.updateSelectedPreview()

		case key.Matches(msg, a.keys.Up):
			a.list.MoveUp()
			return a, a.updateSelectedPreview()

		case key.Matches(msg, a.keys.Down):
			a.list.MoveDown()
			return a, a.updateSelectedPreview()

		case key.Matches(msg, a.keys.Left):
			if item := a.list.SelectedItem(); item != nil && item.IsGroup() {
				if item.Group.Expanded {
					a.list.ToggleGroup()
				}
			}

		case key.Matches(msg, a.keys.Right):
			if item := a.list.SelectedItem(); item != nil && item.IsGroup() {
				if !item.Group.Expanded {
					a.list.ToggleGroup()
				}
			}

		case key.Matches(msg, a.keys.Enter):
			return a.handleOpen()

		case key.Matches(msg, a.keys.Search):
			a.list.StartSearch()

		case key.Matches(msg, a.keys.ContentSearch):
			a.list.StartContentSearch()

		case key.Matches(msg, a.keys.Rename):
			if item := a.list.SelectedItem(); item != nil {
				// Don't allow renaming Active/Inactive groups
				if item.IsGroup() && (item.Group.ID == "__active__" || item.Group.ID == "__inactive__") {
					break
				}
				a.list.StartRename()
			}

		case key.Matches(msg, a.keys.Delete):
			// Delete only works for user-created groups (not Active/Inactive)
			if item := a.list.SelectedItem(); item != nil && item.IsGroup() {
				if item.Group.ID != "__active__" && item.Group.ID != "__inactive__" {
					a.list.StartDelete()
				}
			}

		case key.Matches(msg, a.keys.Kill):
			// Kill closes the tab and moves session to inactive
			if item := a.list.SelectedItem(); item != nil && !item.IsGroup() {
				// Find window ID using all matching strategies (stored ID, --resume flag, project path)
				windowID := session.FindWindowIDForSession(item.Session)
				if windowID > 0 {
					terminal.CloseKittyWindow(windowID)
				}
				item.Session.KittyWindowID = 0
				item.Session.Status = session.StatusIdle
				// Update last_active_sessions so killed session won't be resumed
				a.trackActiveSessions()
				a.manager.Save()
				a.list.Refresh()
				return a, a.setStatus("Session killed")
			}

		case key.Matches(msg, a.keys.Move):
			if a.list.StartMoving() {
				a.statusMsg = "Move: ↑↓ select group, Enter to confirm, Esc to cancel"
			}

		case key.Matches(msg, a.keys.NewGroup):
			a.list.StartNewGroup()

		case key.Matches(msg, a.keys.Pin):
			if item := a.list.SelectedItem(); item != nil && !item.IsGroup() {
				a.manager.TogglePin(item.ID())
				a.list.Refresh()
				if item.Session.Pinned {
					return a, a.setStatus("Pinned")
				} else {
					return a, a.setStatus("Unpinned")
				}
			}

		case key.Matches(msg, a.keys.Layout):
			a.horizontal = !a.horizontal
			a.updateLayout()
			if a.horizontal {
				return a, a.setStatus("Vertical layout")
			} else {
				return a, a.setStatus("Horizontal layout")
			}

		case key.Matches(msg, a.keys.Theme):
			a.showTheme = true
			// Set cursor to current theme
			for i, name := range ThemeNames {
				if name == CurrentThemeName {
					a.themeCursor = i
					break
				}
			}
			return a, nil

		case key.Matches(msg, a.keys.Resume):
			// Toggle resume on startup setting
			current := a.manager.GetResumeOnStartup()
			a.manager.SetResumeOnStartup(!current)
			if !current {
				return a, a.setStatus("Resume on startup: enabled")
			}
			return a, a.setStatus("Resume on startup: disabled")

		case key.Matches(msg, a.keys.NewSession):
			// Open new session dialog
			a.showNewSession = true
			a.newSessionFocus = 1 // Start on path field
			a.newSessionName = ""
			a.newSessionNameCursor = 0
			a.newSessionPath = ""
			a.newSessionPathCursor = 0
			a.buildNewSessionPaths()
			a.newSessionCursor = 0
			// Default to selected session's path if available
			if item := a.list.SelectedItem(); item != nil && !item.IsGroup() {
				a.newSessionPath = a.shortenPath(item.Session.ProjectPath)
				a.newSessionPathCursor = len(a.newSessionPath)
			}
			return a, nil

		case msg.String() == "ctrl+r":
			// Manual refresh
			a.manager.Load()
			if session.RefreshStatuses(a.manager.Sessions) {
				a.manager.Save()
			}
			a.list.Refresh()
			return a, a.setStatus("Refreshed")

		case msg.String() == "tab":
			// Switch focus between panels
			if a.focus == FocusList {
				a.focus = FocusPreview
			} else {
				a.focus = FocusList
			}

		case msg.String() == "H":
			a.showHelp = true
		}
	}

	return a, nil
}

// handleOpen opens the selected session
func (a *App) handleOpen() (tea.Model, tea.Cmd) {
	item := a.list.SelectedItem()
	if item == nil {
		return a, nil
	}

	if item.IsGroup() {
		// Toggle group expansion
		a.list.ToggleGroup()
		return a, nil
	}

	// Open session in new terminal tab
	windowID, err := terminal.OpenSession(item.Session.ProjectPath, item.Session.ClaudeSessionID, session.GetActiveWindowID(item.Session), item.Session.Name)
	if err != nil {
		return a, a.setStatus("Error: " + err.Error())
	}
	// Store window ID for reliable tab matching (and clear from other sessions)
	if windowID > 0 {
		session.ClaimWindowID(a.manager.Sessions, item.Session, windowID)
		a.manager.Save()
	}
	// Immediately mark session as active (will be refined by next status tick)
	if item.Session.Status == session.StatusIdle {
		item.Session.Status = session.StatusWaiting
		a.list.Refresh() // Move to Active group
	}
	return a, a.setStatus("Opened in new tab")
}

// updateSelectedPreview updates preview for selected item (async)
func (a *App) updateSelectedPreview() tea.Cmd {
	item := a.list.SelectedItem()
	if item == nil || item.IsGroup() {
		a.preview.SetSearchSnippet("")
		return a.preview.SetSession(nil)
	}
	// Pass search snippet if in content search mode
	a.preview.SetSearchSnippet(a.list.GetSearchSnippet())
	return a.preview.SetSession(item.Session)
}

// updateLayout recalculates component sizes
func (a *App) updateLayout() {
	// Total height minus: header(1) + borders(2) + status(1) = 4
	contentHeight := a.height - 4

	if a.horizontal {
		// Side by side layout
		a.listWidth = a.width * listWidthPct / 100
		a.previewWidth = a.width - a.listWidth - 3
		a.contentHeight = contentHeight
		a.listStartY = 2

		a.list.SetSize(a.listWidth-2, contentHeight-2)
		a.preview.SetSize(a.previewWidth-2, contentHeight-2)
	} else {
		// Stacked layout - list on top, preview below
		listH := contentHeight * 35 / 100
		previewH := contentHeight - listH - 2
		a.listWidth = a.width - 4
		a.previewWidth = a.width - 4
		a.contentHeight = listH
		a.listStartY = 2

		a.list.SetSize(a.listWidth, listH-2)
		a.preview.SetSize(a.previewWidth, previewH-2)
	}
}

// View renders the application with fixed layout
func (a *App) View() string {
	if a.quitting {
		return ""
	}

	if a.loading {
		// Show loading screen with spinner, centered
		loadingText := titleStyle.Render("Claude Deck") + "\n\n⏳ Loading sessions..."
		if a.width > 0 && a.height > 0 {
			return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, loadingText)
		}
		return loadingText
	}

	if a.width == 0 || a.height == 0 {
		return ""
	}

	if a.err != nil {
		return titleStyle.Render("Claude Deck") + "\n\n  Error: " + a.err.Error()
	}

	// Header with session count
	sessionCount := len(a.manager.Sessions)
	header := titleStyle.Render("Claude Deck") + "  " + helpStyle.Render(fmt.Sprintf("(%d sessions)", sessionCount))
	headerPadded := header + strings.Repeat(" ", max(0, a.width-lipgloss.Width(header)))

	// Render panel contents
	listContent := a.list.View()
	previewContent := a.preview.View()

	var mainContent string
	contentHeight := a.height - 4

	if a.horizontal {
		// Side by side layout (||)
		listW := a.width * listWidthPct / 100
		previewW := a.width - listW

		listStyle := panelStyle
		previewStyle := panelStyle
		if a.focus == FocusList {
			listStyle = activePanelStyle
		} else {
			previewStyle = activePanelStyle
		}

		listPanel := listStyle.Width(listW - 2).Height(contentHeight).Render(listContent)
		previewPanel := previewStyle.Width(previewW - 2).Height(contentHeight).Render(previewContent)

		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, listPanel, previewPanel)
	} else {
		// Stacked layout (=)
		listH := contentHeight * 35 / 100
		previewH := contentHeight - listH - 2
		panelW := a.width - 2

		listStyle := panelStyle
		previewStyle := panelStyle
		if a.focus == FocusList {
			listStyle = activePanelStyle
		} else {
			previewStyle = activePanelStyle
		}

		listPanel := listStyle.Width(panelW).Height(listH).Render(listContent)
		previewPanel := previewStyle.Width(panelW).Height(previewH).Render(previewContent)

		mainContent = lipgloss.JoinVertical(lipgloss.Left, listPanel, previewPanel)
	}

	// Status bar - help on left, messages on right
	helpText := "Enter:open  N:new  /,?:search  R:rename  K:kill  P:pin  H:help  Q:quit"
	var statusLine string
	if a.statusMsg != "" {
		gap := a.width - len(helpText) - len(a.statusMsg) - 4
		if gap < 1 {
			gap = 1
		}
		statusLine = helpStyle.Render(helpText) + strings.Repeat(" ", gap) + helpStyle.Render("│ "+a.statusMsg)
	} else {
		statusLine = helpStyle.Render(helpText)
	}
	statusPadded := statusLine + strings.Repeat(" ", max(0, a.width-lipgloss.Width(statusLine)))

	// Build final view
	var view string
	if a.dialog.IsOpen() {
		view = lipgloss.JoinVertical(lipgloss.Left,
			headerPadded,
			mainContent,
			statusPadded,
			"",
			a.dialog.View(),
		)
	} else {
		view = lipgloss.JoinVertical(lipgloss.Left,
			headerPadded,
			mainContent,
			statusPadded,
		)
	}

	// Show overlays as full screen replacement when active
	if a.showHelp {
		return a.renderHelp()
	}
	if a.showTheme {
		return a.renderThemeSelect()
	}
	if a.showNewSession {
		return a.renderNewSessionDialog()
	}

	return view
}

// trackActiveSessions updates the list of active session IDs for resume
func (a *App) trackActiveSessions() {
	var activeIDs []string
	for _, s := range a.manager.Sessions {
		if s.Status != session.StatusIdle {
			activeIDs = append(activeIDs, s.ClaudeSessionID)
		}
	}

	// Compare with stored list to avoid unnecessary saves
	stored := a.manager.GetLastActiveSessionIDs()
	if !stringSliceEqual(activeIDs, stored) {
		a.manager.SetLastActiveSessionIDs(activeIDs)
		a.manager.Save()
	}
}

// stringSliceEqual compares two string slices for equality
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// restoreSessions reopens previously active sessions if resume is enabled
func (a *App) restoreSessions() tea.Cmd {
	if !a.manager.GetResumeOnStartup() {
		return nil
	}

	lastActive := a.manager.GetLastActiveSessionIDs()
	if len(lastActive) == 0 {
		return nil
	}

	// Build map for quick lookup by ClaudeSessionID
	sessionMap := make(map[string]*session.Session)
	for _, s := range a.manager.Sessions {
		sessionMap[s.ClaudeSessionID] = s
	}

	var cmds []tea.Cmd
	for _, id := range lastActive {
		s := sessionMap[id]
		if s == nil {
			continue // Session was deleted
		}
		if s.Status != session.StatusIdle {
			continue // Already active (has open tab)
		}
		// Capture session for closure
		sess := s
		cmds = append(cmds, func() tea.Msg {
			terminal.OpenSession(sess.ProjectPath, sess.ClaudeSessionID, 0, sess.Name)
			return nil
		})
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// renderHelp renders a centered help screen
func (a *App) renderHelp() string {
	helpBox := `╭───────────────────────────────────────╮
│           Claude Deck Help            │
├───────────────────────────────────────┤
│  Navigation                           │
│    ↑/↓      Move up/down              │
│    ⇧↑/⇧↓    Move up/down fast         │
│    ←/→      Collapse/expand group     │
│    Tab      Switch panel focus        │
│                                       │
│  Actions (Shift + key)                │
│    Enter    Open session in terminal  │
│    G        Create new group          │
│    N        New session (pick folder) │
│    R        Rename session/group      │
│    K        Kill session (close tab)  │
│    D        Delete group              │
│    M        Move session to group     │
│    P        Pin/unpin session         │
│                                       │
│  Search                               │
│    /        Search by name            │
│    ?        Search in content         │
│                                       │
│  Settings                             │
│    L        Toggle layout (|| / =)    │
│    C        Select color theme        │
│    S        Toggle resume on startup  │
│                                       │
│  Other                                │
│    Ctrl+R   Refresh status/names      │
│    H        Show this help            │
│    Q        Quit                      │
│                                       │
│         Press any key to close        │
╰───────────────────────────────────────╯`

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, helpBox)
}

func (a *App) renderThemeSelect() string {
	const innerWidth = 32
	var lines []string

	lines = append(lines, "╭────────────────────────────────╮")
	lines = append(lines, "│         Select Theme           │")
	lines = append(lines, "├────────────────────────────────┤")

	// Theme options
	for i, name := range ThemeNames {
		isSelected := i == a.themeCursor
		isCurrent := name == CurrentThemeName

		cursor := "  "
		if isSelected {
			cursor = "> "
		}
		check := "  "
		if isCurrent {
			check = " ✓"
		}

		// Build line content: cursor(2) + name + padding + check(2) = 32
		padLen := innerWidth - 2 - len(name) - 2
		if padLen < 0 {
			padLen = 0
		}
		content := cursor + name + strings.Repeat(" ", padLen) + check

		if isSelected {
			// Preview the theme's colors on hover
			if theme, ok := Themes[name]; ok {
				previewStyle := lipgloss.NewStyle().
					Background(theme.Surface).
					Foreground(theme.Primary).
					Bold(true).
					Width(innerWidth)
				content = previewStyle.Render(content)
			}
		}
		lines = append(lines, "│"+content+"│")
	}

	lines = append(lines, "├────────────────────────────────┤")
	lines = append(lines, "│ ↑↓:navigate Enter:ok Esc:cancel│")
	lines = append(lines, "╰────────────────────────────────╯")

	box := strings.Join(lines, "\n")
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
}

// buildNewSessionPaths builds the list of paths for the new session dialog
func (a *App) buildNewSessionPaths() {
	a.newSessionPaths = nil

	// Add favorites first (starred)
	favorites := a.manager.GetFavoritePaths()
	for _, p := range favorites {
		a.newSessionPaths = append(a.newSessionPaths, p)
	}

	// Add recent paths from sessions (excluding duplicates from favorites)
	recentPaths := a.manager.GetUniqueProjectPaths()
	favSet := make(map[string]bool)
	for _, p := range favorites {
		favSet[p] = true
	}
	for _, p := range recentPaths {
		if !favSet[p] {
			a.newSessionPaths = append(a.newSessionPaths, p)
		}
	}

	// If no paths at all, add home directory
	if len(a.newSessionPaths) == 0 {
		home, _ := os.UserHomeDir()
		a.newSessionPaths = append(a.newSessionPaths, home)
	}
}

// updateNewSessionDialog handles input for the new session dialog
func (a *App) updateNewSessionDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	switch keyMsg.String() {
	case "esc":
		a.showNewSession = false
		return a, nil

	case "enter":
		// Use path from input or selected from list
		path := a.newSessionPath
		if path == "" && a.newSessionCursor >= 0 && a.newSessionCursor < len(a.newSessionPaths) {
			path = a.newSessionPaths[a.newSessionCursor]
		}
		if path == "" {
			return a, nil
		}
		path = a.expandPath(path)
		if _, err := os.Stat(path); err != nil {
			return a, a.setStatus("Path not found: " + path)
		}
		a.showNewSession = false

		// Clear kitty_window_id from existing sessions to prevent path-matching
		for _, s := range a.manager.Sessions {
			if s.ProjectPath == path {
				s.KittyWindowID = 0
			}
		}

		// Get the session name (for tab title)
		name := strings.TrimSpace(a.newSessionName)
		if name != "" {
			name = a.makeUniqueName(name)
		}

		// Open new session and get window ID
		windowID, err := terminal.NewSession(path, name)
		if err != nil {
			return a, a.setStatus("Error: " + err.Error())
		}

		// Create a pending session immediately (before JSONL exists)
		// This uses a temporary ID that will be updated when JSONL is created
		if windowID > 0 {
			pendingID := fmt.Sprintf("pending-%d", windowID)
			now := time.Now()
			pendingSession := &session.Session{
				ID:              pendingID,
				ClaudeSessionID: pendingID,
				Name:            name,
				ProjectPath:     path,
				KittyWindowID:   windowID,
				Status:          session.StatusWaiting,
				Renamed:         name != "",
				CreatedAt:       now,
				LastAccessedAt:  now,
			}
			if name == "" {
				pendingSession.Name = filepath.Base(path) + " (new)"
			}
			a.manager.Sessions = append(a.manager.Sessions, pendingSession)
			a.manager.Save()
		}

		// Store pending state for when JSONL is created
		a.pendingRenamePath = path
		a.pendingRenameName = name
		a.pendingRenameWindowID = windowID

		a.list.Refresh()

		if name != "" {
			return a, a.setStatus("Created: " + name)
		}
		return a, a.setStatus("Opening new session in " + filepath.Base(path) + "...")

	case "tab":
		// Switch focus: name -> path -> list, with autocomplete on path
		if a.newSessionFocus == 0 {
			a.newSessionFocus = 1
		} else if a.newSessionFocus == 1 {
			// Try autocomplete first, if nothing changes move to list
			oldPath := a.newSessionPath
			a.autocompleteNewSessionPath()
			if a.newSessionPath == oldPath && len(a.newSessionPaths) > 0 {
				a.newSessionFocus = 2
			}
		} else {
			a.newSessionFocus = 0
		}
		return a, nil

	case "shift+tab":
		// Reverse focus
		if a.newSessionFocus == 0 {
			if len(a.newSessionPaths) > 0 {
				a.newSessionFocus = 2
			} else {
				a.newSessionFocus = 1
			}
		} else if a.newSessionFocus == 1 {
			a.newSessionFocus = 0
		} else {
			a.newSessionFocus = 1
		}
		return a, nil

	case "up":
		if a.newSessionFocus == 0 {
			// Already at top
		} else if a.newSessionFocus == 1 {
			a.newSessionFocus = 0
		} else if a.newSessionFocus == 2 {
			if a.newSessionCursor > 0 {
				a.newSessionCursor--
				// Update path field to match selection
				a.newSessionPath = a.shortenPath(a.newSessionPaths[a.newSessionCursor])
				a.newSessionPathCursor = len(a.newSessionPath)
			} else {
				a.newSessionFocus = 1
			}
		}
		return a, nil

	case "down":
		if a.newSessionFocus == 0 {
			a.newSessionFocus = 1
		} else if a.newSessionFocus == 1 {
			if len(a.newSessionPaths) > 0 {
				a.newSessionFocus = 2
				a.newSessionCursor = 0
				// Update path field to match selection
				a.newSessionPath = a.shortenPath(a.newSessionPaths[0])
				a.newSessionPathCursor = len(a.newSessionPath)
			}
		} else if a.newSessionFocus == 2 {
			if a.newSessionCursor < len(a.newSessionPaths)-1 {
				a.newSessionCursor++
				// Update path field to match selection
				a.newSessionPath = a.shortenPath(a.newSessionPaths[a.newSessionCursor])
				a.newSessionPathCursor = len(a.newSessionPath)
			}
		}
		return a, nil

	case "P":
		// Toggle pin on selected path
		if a.newSessionCursor >= 0 && a.newSessionCursor < len(a.newSessionPaths) {
			path := a.newSessionPaths[a.newSessionCursor]
			a.manager.ToggleFavoritePath(path)
			oldPath := path
			a.filterNewSessionPaths()
			for i, p := range a.newSessionPaths {
				if p == oldPath {
					a.newSessionCursor = i
					break
				}
			}
		}
		return a, nil

	default:
		// Handle text input based on focus
		if a.newSessionFocus == 0 {
			newText, newCursor, handled := handleTextInputKey(a.newSessionName, a.newSessionNameCursor, keyMsg.String())
			if handled {
				a.newSessionName = newText
				a.newSessionNameCursor = newCursor
			}
		} else if a.newSessionFocus == 1 {
			oldPath := a.newSessionPath
			newText, newCursor, handled := handleTextInputKey(a.newSessionPath, a.newSessionPathCursor, keyMsg.String())
			if handled {
				a.newSessionPath = newText
				a.newSessionPathCursor = newCursor
				if newText != oldPath {
					a.filterNewSessionPaths()
				}
			}
		}
		return a, nil
	}
}

// expandPath expands ~ to home directory
func (a *App) expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	} else if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	return path
}

// filterNewSessionPaths filters the path list based on current input
func (a *App) filterNewSessionPaths() {
	a.buildNewSessionPaths()

	if a.newSessionPath == "" {
		return
	}

	input := strings.ToLower(a.newSessionPath)
	expandedInput := a.expandPath(a.newSessionPath)

	var filtered []string
	for _, p := range a.newSessionPaths {
		shortPath := a.shortenPath(p)
		// Match if input is substring of path or shortened path
		if strings.Contains(strings.ToLower(p), input) ||
			strings.Contains(strings.ToLower(shortPath), input) ||
			strings.HasPrefix(p, expandedInput) {
			filtered = append(filtered, p)
		}
	}

	// Also add filesystem completions if input looks like a path
	if strings.HasPrefix(a.newSessionPath, "/") || strings.HasPrefix(a.newSessionPath, "~") {
		fsCompletions := a.getPathCompletions(expandedInput)
		for _, c := range fsCompletions {
			// Add if not already in list
			found := false
			for _, p := range filtered {
				if p == c {
					found = true
					break
				}
			}
			if !found {
				filtered = append(filtered, c)
			}
		}
	}

	a.newSessionPaths = filtered
	if a.newSessionCursor >= len(filtered) {
		a.newSessionCursor = max(0, len(filtered)-1)
	}
}

// getPathCompletions returns directory completions for a path prefix
func (a *App) getPathCompletions(prefix string) []string {
	var completions []string

	dir := filepath.Dir(prefix)
	base := filepath.Base(prefix)

	// If prefix ends with /, list that directory
	if strings.HasSuffix(prefix, "/") || prefix == "" {
		dir = prefix
		base = ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return completions
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // Skip hidden
		}
		if base == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(base)) {
			completions = append(completions, filepath.Join(dir, name))
		}
	}

	// Limit to 5 completions
	if len(completions) > 5 {
		completions = completions[:5]
	}

	return completions
}

// autocompleteNewSessionPath completes the path input
func (a *App) autocompleteNewSessionPath() {
	if a.newSessionPath == "" {
		// If no input but item selected, use that
		if a.newSessionCursor >= 0 && a.newSessionCursor < len(a.newSessionPaths) {
			a.newSessionPath = a.shortenPath(a.newSessionPaths[a.newSessionCursor])
			a.newSessionPathCursor = len(a.newSessionPath)
		}
		return
	}

	expandedInput := a.expandPath(a.newSessionPath)
	completions := a.getPathCompletions(expandedInput)

	if len(completions) == 1 {
		// Single match - complete to it
		completed := a.shortenPath(completions[0])
		a.newSessionPath = completed + "/"
		a.newSessionPathCursor = len(a.newSessionPath)
		a.filterNewSessionPaths()
	} else if len(completions) > 1 {
		// Multiple matches - complete to common prefix
		common := completions[0]
		for _, c := range completions[1:] {
			common = commonPrefix(common, c)
		}
		if len(common) > len(expandedInput) {
			a.newSessionPath = a.shortenPath(common)
			a.newSessionPathCursor = len(a.newSessionPath)
			a.filterNewSessionPaths()
		}
	}
}

// commonPrefix returns the common prefix of two strings
func commonPrefix(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:minLen]
}

// renderNewSessionDialog renders the new session path selection dialog
func (a *App) renderNewSessionDialog() string {
	innerWidth := 56

	// Build the box
	hLine := strings.Repeat("─", innerWidth)

	var lines []string
	lines = append(lines, "╭"+hLine+"╮")

	// Title
	title := "New Claude Session"
	titlePad := (innerWidth - len(title)) / 2
	lines = append(lines, "│"+strings.Repeat(" ", titlePad)+title+strings.Repeat(" ", innerWidth-titlePad-len(title))+"│")
	lines = append(lines, "├"+hLine+"┤")

	// Name field
	nameLabel := "  Name: "
	nameFocused := a.newSessionFocus == 0
	nameText := a.newSessionName
	if nameFocused {
		if a.newSessionNameCursor <= len(nameText) {
			nameText = nameText[:a.newSessionNameCursor] + "_" + nameText[a.newSessionNameCursor:]
		}
	}
	if nameText == "" && !nameFocused {
		nameText = "(optional)"
	}
	nameContent := nameLabel + nameText
	if len(nameContent) > innerWidth {
		nameContent = nameContent[:innerWidth]
	}
	nameContent = nameContent + strings.Repeat(" ", innerWidth-len(nameContent))
	if nameFocused {
		nameContent = selectedItemStyle.Render(nameContent)
	}
	lines = append(lines, "│"+nameContent+"│")

	// Path field
	pathLabel := "  Path: "
	pathFocused := a.newSessionFocus == 1
	pathText := a.newSessionPath
	if pathFocused {
		if a.newSessionPathCursor <= len(pathText) {
			pathText = pathText[:a.newSessionPathCursor] + "_" + pathText[a.newSessionPathCursor:]
		}
	}
	pathContent := pathLabel + pathText
	if len(pathContent) > innerWidth {
		pathContent = pathContent[:innerWidth]
	}
	pathContent = pathContent + strings.Repeat(" ", innerWidth-len(pathContent))
	if pathFocused {
		pathContent = selectedItemStyle.Render(pathContent)
	}
	lines = append(lines, "│"+pathContent+"│")

	// Separator before path list
	lines = append(lines, "├"+hLine+"┤")

	// Build favorite set
	favorites := a.manager.GetFavoritePaths()
	favSet := make(map[string]bool)
	for _, p := range favorites {
		favSet[p] = true
	}

	// Path list
	maxVisible := 8
	startIdx := 0
	if a.newSessionCursor >= maxVisible {
		startIdx = a.newSessionCursor - maxVisible + 1
	}

	listFocused := a.newSessionFocus == 2
	for i := startIdx; i < len(a.newSessionPaths) && i < startIdx+maxVisible; i++ {
		path := a.newSessionPaths[i]
		displayPath := a.shortenPath(path)
		isFav := favSet[path]
		isSelected := i == a.newSessionCursor

		// Truncate path if needed
		maxPathDisplay := innerWidth - 6
		if len(displayPath) > maxPathDisplay {
			displayPath = displayPath[:maxPathDisplay-2] + ".."
		}

		// Build the line content
		cursor := "  "
		if isSelected && listFocused {
			cursor = "> "
		}
		star := "  "
		if isFav {
			star = "✦ "
		}

		content := cursor + star + displayPath
		padLen := innerWidth - 4 - len(displayPath)
		if padLen > 0 {
			content += strings.Repeat(" ", padLen)
		}

		// Apply highlight style if selected
		if isSelected {
			content = selectedItemStyle.Render(content)
		}
		lines = append(lines, "│"+content+"│")
	}

	// Fill empty slots if list is short
	for i := len(a.newSessionPaths); i < maxVisible; i++ {
		lines = append(lines, "│"+strings.Repeat(" ", innerWidth)+"│")
	}

	// Help line
	lines = append(lines, "├"+hLine+"┤")
	helpLine := "Tab:next  P:pin  Enter:create  Esc:cancel"
	helpPad := (innerWidth - len(helpLine)) / 2
	lines = append(lines, "│"+strings.Repeat(" ", helpPad)+helpLine+strings.Repeat(" ", innerWidth-helpPad-len(helpLine))+"│")

	lines = append(lines, "╰"+hLine+"╯")

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n"))
}

// shortenPath shortens a path for display, replacing home dir with ~
func (a *App) shortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// makeUniqueName returns a unique session name by appending (1), (2), etc. if needed
func (a *App) makeUniqueName(name string) string {
	// Check if name already exists
	exists := func(n string) bool {
		for _, s := range a.manager.Sessions {
			if s.Name == n {
				return true
			}
		}
		return false
	}

	if !exists(name) {
		return name
	}

	// Try adding suffix
	for i := 1; i <= 100; i++ {
		candidate := fmt.Sprintf("%s (%d)", name, i)
		if !exists(candidate) {
			return candidate
		}
	}
	return name
}
