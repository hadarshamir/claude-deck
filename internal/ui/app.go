package ui

import (
	"fmt"
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

	watcher      *fsnotify.Watcher
	showHelp     bool // true when help overlay is visible
	showTerminal bool // true when terminal selection overlay is visible
	terminalCursor int // cursor position in terminal selection
	showTheme    bool // true when theme selection overlay is visible
	themeCursor  int  // cursor position in theme selection
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
			if session.RefreshStatuses(a.manager.Sessions) {
				a.manager.Save() // Persist tab title names
			}
			// Refresh git branches (one git call per unique path)
			session.RefreshGitBranches(a.manager.Sessions)
		}
		return sessionsLoadedMsg{err: err}
	}
}

// tickStatus periodically updates session statuses
func (a *App) tickStatus() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return statusTickMsg{}
	})
}

// tickDiscovery periodically refreshes git info (every 30s to save battery)
func (a *App) tickDiscovery() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return sessionDiscoveryTickMsg{}
	})
}

type statusTickMsg struct{}
type statusRefreshedMsg struct {
	updates   []session.StatusUpdate
	noChanges bool // true if nothing changed - skip re-render
}
type clearStatusMsg struct{}
type sessionsLoadedMsg struct {
	err error
}
type sessionDiscoveryTickMsg struct{}

// setStatus sets a status message and returns a command to clear it after 2 seconds
func (a *App) setStatus(msg string) tea.Cmd {
	a.statusMsg = msg
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// watchFiles sets up file watching
func (a *App) watchFiles() tea.Cmd {
	return func() tea.Msg {
		// Watch the Claude projects directory
		if err := a.watcher.Add(session.ClaudeProjectsDir()); err != nil {
			return nil
		}

		for {
			select {
			case event, ok := <-a.watcher.Events:
				if !ok {
					return nil
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					return fileChangedMsg{path: event.Name}
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

		if msg.String() == "q" && !a.dialog.IsOpen() && !a.search.IsActive() {
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

	case tea.MouseMsg:
		return a.handleMouse(msg)

	case statusTickMsg:
		// Compute status updates async
		sessions := a.manager.Sessions
		return a, func() tea.Msg {
			updates, _, anyChanged := session.ComputeStatuses(sessions)
			return statusRefreshedMsg{updates: updates, noChanges: !anyChanged}
		}

	case statusRefreshedMsg:
		// Skip if nothing changed
		if msg.noChanges {
			return a, a.tickStatus()
		}

		// Apply updates to session objects (modifies Status field in place)
		// No need to update pointers - sessions haven't been reloaded
		_, needsSave := session.ApplyStatusUpdates(a.manager.Sessions, msg.updates)

		// Save if persisted fields changed (names, window IDs)
		if needsSave {
			a.manager.Save()
		}

		return a, a.tickStatus()

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
		// Apply terminal preference from settings
		if pref := a.manager.GetPreferredTerminal(); pref != "" {
			terminal.SetPreferredTerminal(terminal.ParseTerminal(pref))
		}
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
		cmds = append(cmds, a.tickStatus())
		cmds = append(cmds, a.tickDiscovery())
		return a, tea.Batch(cmds...)

	case sessionDiscoveryTickMsg:
		// Git branch comes from JSONL - no polling needed
		return a, a.tickDiscovery()

	case fileChangedMsg:
		// Refresh preview if the changed file is the current session
		if item := a.list.SelectedItem(); item != nil && !item.IsGroup() {
			if item.Session.JSONLPath == msg.path {
				return a, a.preview.Refresh()
			}
		}
		return a, nil

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
			if msg.String() == "esc" || msg.String() == "h" || msg.String() == "enter" || msg.String() == "q" {
				a.showHelp = false
			}
		}
		return a, nil
	}

	// Handle terminal selection overlay
	if a.showTerminal {
		if msg, ok := msg.(tea.KeyMsg); ok {
			allTerminals := terminal.AllTerminals()
			switch msg.String() {
			case "esc", "q":
				a.showTerminal = false
			case "enter":
				// Apply selection
				selected := allTerminals[a.terminalCursor]
				a.manager.SetPreferredTerminal(selected.String())
				terminal.SetPreferredTerminal(selected)
				a.showTerminal = false
				return a, a.setStatus("Terminal set to " + selected.String())
			case "up", "k":
				if a.terminalCursor > 0 {
					a.terminalCursor--
				}
			case "down", "j":
				if a.terminalCursor < len(allTerminals)-1 {
					a.terminalCursor++
				}
			}
		}
		return a, nil
	}

	// Handle theme selection overlay
	if a.showTheme {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "esc", "q":
				a.showTheme = false
			case "enter":
				// Apply selection
				selected := ThemeNames[a.themeCursor]
				a.manager.SetTheme(selected)
				ApplyTheme(selected)
				a.showTheme = false
				return a, a.setStatus("Theme set to " + selected)
			case "up", "k":
				if a.themeCursor > 0 {
					a.themeCursor--
					ApplyTheme(ThemeNames[a.themeCursor]) // Live preview
				}
			case "down", "j":
				if a.themeCursor < len(ThemeNames)-1 {
					a.themeCursor++
					ApplyTheme(ThemeNames[a.themeCursor]) // Live preview
				}
			}
		}
		return a, nil
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
				windowID, err := terminal.OpenSession(item.Session.ProjectPath, item.Session.ClaudeSessionID, session.GetActiveWindowID(item.Session))
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
			if id != "" && newName != "" {
				if isGroup {
					a.manager.RenameGroup(id, newName)
				} else {
					a.manager.RenameSession(id, newName)
				}
				a.list.Refresh()
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
				if item.Session.KittyWindowID > 0 {
					terminal.CloseKittyWindow(item.Session.KittyWindowID)
					item.Session.KittyWindowID = 0
				}
				item.Session.Status = session.StatusIdle
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

		case key.Matches(msg, a.keys.Terminal):
			a.showTerminal = true
			// Set cursor to current selection
			currentPref := terminal.GetPreferredTerminal()
			for i, t := range terminal.AllTerminals() {
				if t == currentPref {
					a.terminalCursor = i
					break
				}
			}
			return a, nil

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

		case key.Matches(msg, a.keys.NewSession):
			// New session - use selected session's project or prompt
			if item := a.list.SelectedItem(); item != nil && !item.IsGroup() {
				terminal.NewSession(item.Session.ProjectPath)
				return a, a.setStatus("Opening new session in " + item.Session.FolderName() + "...")
			} else {
				return a, a.setStatus("Select a session first")
			}

		case msg.String() == "R":
			// Refresh
			a.manager.Load()
			if session.RefreshStatuses(a.manager.Sessions) {
				a.manager.Save()
			}
			// Git info refreshes async via discovery tick
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
	windowID, err := terminal.OpenSession(item.Session.ProjectPath, item.Session.ClaudeSessionID, session.GetActiveWindowID(item.Session))
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
		// Show loading screen with spinner, centered if we have dimensions
		loadingText := titleStyle.Render("Claude Deck") + "\n\n  ⏳ Loading sessions..."
		if a.width > 0 && a.height > 0 {
			// Center vertically
			padding := a.height / 3
			return strings.Repeat("\n", padding) + loadingText
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
	helpText := "Enter:open  /,?:search  R:rename  K:kill  D:delete  P:pin  H:help  q:quit"
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
	if a.showTerminal {
		return a.renderTerminalSelect()
	}
	if a.showTheme {
		return a.renderThemeSelect()
	}

	return view
}

// renderHelp renders a centered help screen
func (a *App) renderHelp() string {
	helpBox := `╭───────────────────────────────────────╮
│           Claude Deck Help            │
├───────────────────────────────────────┤
│  Navigation                           │
│    ↑/↓      Move up/down              │
│    ⇧↑/⇧↓    Move up/down fast (5x)    │
│    ←/→      Collapse/expand group     │
│    Tab      Switch panel focus        │
│                                       │
│  Actions (Shift + key)                │
│    Enter    Open session in terminal  │
│    G        Create new group          │
│    N        New session (same folder) │
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
│    T        Select terminal emulator  │
│    C        Select color theme        │
│                                       │
│  Other                                │
│    H        Show this help            │
│    q        Quit                      │
│                                       │
│         Press any key to close        │
╰───────────────────────────────────────╯`

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, helpBox)
}

// renderTerminalSelect renders a centered terminal selection screen
func (a *App) renderTerminalSelect() string {
	allTerminals := terminal.AllTerminals()
	currentPref := terminal.GetPreferredTerminal()

	// Build option lines - inner width is 29 chars (31 total with │ on each side)
	var optionLines []string
	for i, t := range allTerminals {
		cursor := "  "
		if i == a.terminalCursor {
			cursor = "> "
		}
		name := t.String()
		if t == currentPref {
			name += " (current)"
		}
		// Format: space + cursor(2) + name, padded to 29 total inner
		line := " " + cursor + name
		for len(line) < 29 {
			line += " "
		}
		optionLines = append(optionLines, "│"+line+"│")
	}

	box := "╭─────────────────────────────╮\n" +
		"│      Select Terminal        │\n" +
		"├─────────────────────────────┤\n" +
		strings.Join(optionLines, "\n") + "\n" +
		"│                             │\n" +
		"│  j/k:navigate Enter:select  │\n" +
		"│         Esc:cancel          │\n" +
		"╰─────────────────────────────╯"

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
}

func (a *App) renderThemeSelect() string {
	// Build option lines - inner width is 29 chars (31 total with │ on each side)
	var optionLines []string
	for i, name := range ThemeNames {
		cursor := "  "
		if i == a.themeCursor {
			cursor = "> "
		}
		displayName := name
		if name == CurrentThemeName {
			displayName += " ✓"
		}
		// Format: space + cursor(2) + name, padded to 29 total inner
		line := " " + cursor + displayName
		for len(line) < 29 {
			line += " "
		}
		optionLines = append(optionLines, "│"+line+"│")
	}

	box := "╭─────────────────────────────╮\n" +
		"│        Select Theme         │\n" +
		"├─────────────────────────────┤\n" +
		strings.Join(optionLines, "\n") + "\n" +
		"│                             │\n" +
		"│  j/k:navigate Enter:select  │\n" +
		"│         Esc:cancel          │\n" +
		"╰─────────────────────────────╯"

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
}
