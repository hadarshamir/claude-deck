package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hadar/claude-deck/internal/session"
)

// ListItem represents an item in the hierarchical list
type ListItem struct {
	Session      *session.Session
	Group        *session.Group
	Indent       int
	Filtered     bool
	IsLastInTree bool // true if this is the last child in a group (use └ instead of ├)
}

func (i ListItem) IsGroup() bool {
	return i.Group != nil
}

func (i ListItem) ID() string {
	if i.Group != nil {
		return i.Group.ID
	}
	return i.Session.ID
}

func (i ListItem) Name() string {
	if i.Group != nil {
		return i.Group.Name
	}
	return i.Session.Name
}

// ListModel manages the session list
type ListModel struct {
	items      []ListItem
	cursor     int
	hoverIndex int // -1 if no hover
	height     int
	width      int
	offset     int
	manager    *session.Manager
	filter     string
	filtered   []int

	// Auto-group expansion state
	activeExpanded   bool
	inactiveExpanded bool

	// In-place rename
	renaming     bool
	renameInput  string
	renameCursor int

	// Moving mode
	moving       bool
	moveTargetID string

	// New group mode
	creatingGroup  bool
	newGroupInput  string
	newGroupCursor int

	// Delete confirmation mode
	deleting       bool
	deleteTargetID string
	deleteIsGroup  bool

	// Search (rendered inside panel)
	searching    bool
	searchQuery  string
	searchCursor int

	// Content search mode
	contentSearching    bool
	contentSearchQuery  string
	contentSearchCursor int
	contentSearchResults []session.SearchResult
}

// NewListModel creates a new list model
func NewListModel(manager *session.Manager) *ListModel {
	m := &ListModel{
		manager:          manager,
		hoverIndex:       -1,
		activeExpanded:   manager.GetActiveExpanded(),
		inactiveExpanded: manager.GetInactiveExpanded(),
	}
	m.buildItems()
	return m
}

// buildItems constructs the flat list from groups and sessions
func (m *ListModel) buildItems() {
	m.items = nil

	// Separate sessions by category: pinned, active, inactive
	// Sessions with GroupPath go to their user group, not Active/Inactive
	var pinnedSessions, activeSessions, inactiveSessions []*session.Session
	for _, s := range m.manager.Sessions {
		if s.Pinned {
			pinnedSessions = append(pinnedSessions, s)
		} else if s.GroupPath != "" {
			// Session belongs to a user group - don't add to Active/Inactive
			continue
		} else if s.Status == session.StatusWaiting || s.Status == session.StatusRunning {
			activeSessions = append(activeSessions, s)
		} else {
			inactiveSessions = append(inactiveSessions, s)
		}
	}

	// Sort pinned by Order
	sort.Slice(pinnedSessions, func(i, j int) bool {
		return pinnedSessions[i].Order < pinnedSessions[j].Order
	})
	// Sort active by LastAccessedAt (most recent first = chronological open time)
	sort.Slice(activeSessions, func(i, j int) bool {
		return activeSessions[i].LastAccessedAt.After(activeSessions[j].LastAccessedAt)
	})
	// Sort inactive by Order (stable position)
	sort.Slice(inactiveSessions, func(i, j int) bool {
		return inactiveSessions[i].Order < inactiveSessions[j].Order
	})

	// Add pinned sessions first
	for _, s := range pinnedSessions {
		m.items = append(m.items, ListItem{Session: s, Indent: 0})
	}

	// Add "Active" group with active sessions
	if len(activeSessions) > 0 {
		activeGroup := &session.Group{
			ID:       "__active__",
			Name:     "Active",
			Path:     "__active__",
			Expanded: m.activeExpanded,
		}
		m.items = append(m.items, ListItem{Group: activeGroup, Indent: 0})
		if m.activeExpanded {
			for i, s := range activeSessions {
				isLast := i == len(activeSessions)-1
				m.items = append(m.items, ListItem{Session: s, Indent: 1, IsLastInTree: isLast})
			}
		}
	}

	// Add "Inactive" group with inactive sessions
	if len(inactiveSessions) > 0 {
		inactiveGroup := &session.Group{
			ID:       "__inactive__",
			Name:     "Inactive",
			Path:     "__inactive__",
			Expanded: m.inactiveExpanded,
		}
		m.items = append(m.items, ListItem{Group: inactiveGroup, Indent: 0})
		if m.inactiveExpanded {
			for i, s := range inactiveSessions {
				isLast := i == len(inactiveSessions)-1
				m.items = append(m.items, ListItem{Session: s, Indent: 1, IsLastInTree: isLast})
			}
		}
	}

	// Add user-created groups (always show, even if empty)
	for _, g := range m.manager.Groups {
		m.items = append(m.items, ListItem{Group: g, Indent: 0})

		if g.Expanded {
			sessionsInGroup := m.manager.SessionsInGroup(g.Path)
			for i, s := range sessionsInGroup {
				isLast := i == len(sessionsInGroup)-1
				m.items = append(m.items, ListItem{Session: s, Indent: 1, IsLastInTree: isLast})
			}
		}
	}

	m.applyFilter()
}

func (m *ListModel) applyFilter() {
	m.filtered = nil

	// Content search mode - use search results order, avoid duplicates
	if m.contentSearching && len(m.contentSearchResults) > 0 {
		seen := make(map[string]bool)
		for _, r := range m.contentSearchResults {
			if seen[r.Session.ID] {
				continue
			}
			seen[r.Session.ID] = true
			// Find this session in items (first occurrence only)
			for i, item := range m.items {
				if !item.IsGroup() && item.Session.ID == r.Session.ID {
					m.filtered = append(m.filtered, i)
					break
				}
			}
		}
	} else {
		// Normal name filter
		for i, item := range m.items {
			if m.filter == "" || m.matchesFilter(item) {
				m.filtered = append(m.filtered, i)
			}
		}
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m *ListModel) matchesFilter(item ListItem) bool {
	name := strings.ToLower(item.Name())
	filter := strings.ToLower(m.filter)
	return strings.Contains(name, filter)
}

func (m *ListModel) SetFilter(filter string) {
	m.filter = filter
	m.applyFilter()
}

func (m *ListModel) ClearFilter() {
	m.filter = ""
	m.applyFilter()
}

func (m *ListModel) Refresh() {
	// Remember selected item ID to restore after rebuild
	var selectedID string
	if item := m.SelectedItem(); item != nil {
		selectedID = item.ID()
	}

	m.buildItems()

	// Restore selection by ID
	if selectedID != "" {
		for i, idx := range m.filtered {
			if m.items[idx].ID() == selectedID {
				m.cursor = i
				m.ensureVisible()
				return
			}
		}
	}
}

// UpdateSessionPointers updates session pointers to match manager.Sessions
// without rebuilding the entire list structure. Returns true if any pointers changed.
func (m *ListModel) UpdateSessionPointers() bool {
	// Build lookup map of current sessions by ID
	sessionMap := make(map[string]*session.Session)
	for _, s := range m.manager.Sessions {
		sessionMap[s.ID] = s
	}

	changed := false
	for i := range m.items {
		if m.items[i].Session != nil {
			if newSession, ok := sessionMap[m.items[i].Session.ID]; ok {
				if m.items[i].Session != newSession {
					m.items[i].Session = newSession
					changed = true
				}
			}
		}
	}
	return changed
}

// ReloadExpansionState reloads expansion state from manager settings
func (m *ListModel) ReloadExpansionState() {
	m.activeExpanded = m.manager.GetActiveExpanded()
	m.inactiveExpanded = m.manager.GetInactiveExpanded()
}

func (m *ListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ListModel) SelectedItem() *ListItem {
	if len(m.filtered) == 0 {
		return nil
	}
	idx := m.filtered[m.cursor]
	return &m.items[idx]
}

func (m *ListModel) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.ensureVisible()
	}
}

func (m *ListModel) MoveDown() {
	if m.cursor < len(m.filtered)-1 {
		m.cursor++
		m.ensureVisible()
	}
}

// MoveUpFast moves cursor up by 5 items
func (m *ListModel) MoveUpFast() {
	m.cursor -= 5
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}

// MoveDownFast moves cursor down by 5 items
func (m *ListModel) MoveDownFast() {
	m.cursor += 5
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}

func (m *ListModel) MoveToTop() {
	m.cursor = 0
	m.offset = 0
}

func (m *ListModel) MoveToBottom() {
	m.cursor = max(0, len(m.filtered)-1)
	m.ensureVisible()
}

func (m *ListModel) SetCursor(pos int) {
	if pos >= 0 && pos < len(m.filtered) {
		m.cursor = pos
		m.ensureVisible()
	}
}

func (m *ListModel) HandleClick(y int) bool {
	targetIdx := m.offset + y
	if targetIdx >= 0 && targetIdx < len(m.filtered) {
		m.cursor = targetIdx
		return true
	}
	return false
}

// SetHover sets the hover index based on mouse Y position
func (m *ListModel) SetHover(y int) {
	targetIdx := m.offset + y
	if targetIdx >= 0 && targetIdx < len(m.filtered) {
		m.hoverIndex = targetIdx
	} else {
		m.hoverIndex = -1
	}
}

// ClearHover clears the hover state
func (m *ListModel) ClearHover() {
	m.hoverIndex = -1
}

func (m *ListModel) ensureVisible() {
	// Calculate actual visible data rows (excluding header and search)
	dataHeight := m.height - 1 // header
	if m.searching {
		dataHeight-- // search line
	}
	if dataHeight <= 0 {
		dataHeight = 10
	}

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+dataHeight {
		m.offset = m.cursor - dataHeight + 1
	}
}

func (m *ListModel) ToggleGroup() tea.Cmd {
	item := m.SelectedItem()
	if item == nil || !item.IsGroup() {
		return nil
	}

	// Handle auto-groups
	switch item.Group.ID {
	case "__active__":
		m.activeExpanded = !m.activeExpanded
		m.manager.SetActiveExpanded(m.activeExpanded)
		m.manager.Save()
	case "__inactive__":
		m.inactiveExpanded = !m.inactiveExpanded
		m.manager.SetInactiveExpanded(m.inactiveExpanded)
		m.manager.Save()
	default:
		m.manager.ToggleGroupExpanded(item.Group.ID)
	}
	m.buildItems()
	return nil
}

// StartRename enters rename mode for the selected item
func (m *ListModel) StartRename() {
	item := m.SelectedItem()
	if item == nil {
		return
	}
	m.renaming = true
	m.renameInput = item.Name()
	m.renameCursor = len(m.renameInput)
}

// IsRenaming returns true if in rename mode
func (m *ListModel) IsRenaming() bool {
	return m.renaming
}

// CancelRename exits rename mode without saving
func (m *ListModel) CancelRename() {
	m.renaming = false
	m.renameInput = ""
}

// ConfirmRename saves the rename and exits rename mode
func (m *ListModel) ConfirmRename() (id string, newName string, isGroup bool) {
	if !m.renaming {
		return "", "", false
	}
	item := m.SelectedItem()
	if item == nil {
		m.renaming = false
		return "", "", false
	}
	id = item.ID()
	newName = m.renameInput
	isGroup = item.IsGroup()
	m.renaming = false
	m.renameInput = ""
	return id, newName, isGroup
}

// HandleRenameKey processes a key during rename mode
func (m *ListModel) HandleRenameKey(key string) {
	newText, newCursor, _ := handleTextInputKey(m.renameInput, m.renameCursor, key)
	m.renameInput = newText
	m.renameCursor = newCursor
}

// StartMoving enters moving mode for the selected session
func (m *ListModel) StartMoving() bool {
	item := m.SelectedItem()
	if item == nil || item.IsGroup() {
		return false
	}
	m.moving = true
	m.moveTargetID = item.ID()
	return true
}

// IsMoving returns true if in moving mode
func (m *ListModel) IsMoving() bool {
	return m.moving
}

// CancelMoving exits moving mode
func (m *ListModel) CancelMoving() {
	m.moving = false
	m.moveTargetID = ""
}

// GetMoveTarget returns the session being moved and the target group
// Sessions can only be moved to groups, not swapped with other sessions
func (m *ListModel) GetMoveTarget() (sessionID string, targetGroupPath string, targetSessionID string) {
	if !m.moving {
		return "", "", ""
	}
	item := m.SelectedItem()
	if item == nil {
		return m.moveTargetID, "", ""
	}
	if item.IsGroup() {
		return m.moveTargetID, item.Group.Path, ""
	}
	// Sessions can only be moved to groups, not swapped
	return m.moveTargetID, "", ""
}

// ConfirmMove returns move info and exits moving mode
func (m *ListModel) ConfirmMove() (sessionID string, targetGroupPath string, targetSessionID string) {
	sessionID, targetGroupPath, targetSessionID = m.GetMoveTarget()
	m.moving = false
	m.moveTargetID = ""
	return sessionID, targetGroupPath, targetSessionID
}

// StartNewGroup enters new group creation mode
func (m *ListModel) StartNewGroup() {
	m.creatingGroup = true
	m.newGroupInput = ""
	m.newGroupCursor = 0
}

// IsCreatingGroup returns true if creating a new group
func (m *ListModel) IsCreatingGroup() bool {
	return m.creatingGroup
}

// CancelNewGroup exits new group mode
func (m *ListModel) CancelNewGroup() {
	m.creatingGroup = false
	m.newGroupInput = ""
}

// ConfirmNewGroup returns the new group name
func (m *ListModel) ConfirmNewGroup() string {
	name := m.newGroupInput
	m.creatingGroup = false
	m.newGroupInput = ""
	return name
}

// HandleNewGroupKey processes a key during new group mode
func (m *ListModel) HandleNewGroupKey(key string) {
	newText, newCursor, _ := handleTextInputKey(m.newGroupInput, m.newGroupCursor, key)
	m.newGroupInput = newText
	m.newGroupCursor = newCursor
}

// StartDelete enters delete confirmation mode
func (m *ListModel) StartDelete() bool {
	item := m.SelectedItem()
	if item == nil {
		return false
	}
	m.deleting = true
	m.deleteTargetID = item.ID()
	m.deleteIsGroup = item.IsGroup()
	return true
}

// IsDeleting returns true if in delete confirmation mode
func (m *ListModel) IsDeleting() bool {
	return m.deleting
}

// CancelDelete exits delete mode
func (m *ListModel) CancelDelete() {
	m.deleting = false
	m.deleteTargetID = ""
}

// ConfirmDelete returns the ID and type to delete
func (m *ListModel) ConfirmDelete() (id string, isGroup bool) {
	id = m.deleteTargetID
	isGroup = m.deleteIsGroup
	m.deleting = false
	m.deleteTargetID = ""
	return id, isGroup
}

// DeleteTargetName returns the name of item being deleted
func (m *ListModel) DeleteTargetName() string {
	item := m.SelectedItem()
	if item == nil {
		return ""
	}
	return item.Name()
}

// StartSearch enters search mode
func (m *ListModel) StartSearch() {
	m.searching = true
	m.searchQuery = ""
	m.searchCursor = 0
}

// IsSearching returns true if in search mode
func (m *ListModel) IsSearching() bool {
	return m.searching
}

// CancelSearch exits search mode and clears filter
func (m *ListModel) CancelSearch() {
	m.searching = false
	m.searchQuery = ""
	m.filter = ""
	m.applyFilter()
}

// ConfirmSearch exits search mode but keeps filter
func (m *ListModel) ConfirmSearch() {
	m.searching = false
}

// HandleSearchKey processes a key during search mode
func (m *ListModel) HandleSearchKey(key string) {
	switch key {
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}
		return
	case "down", "ctrl+n":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.ensureVisible()
		}
		return
	}
	// Handle text input
	newText, newCursor, handled := handleTextInputKey(m.searchQuery, m.searchCursor, key)
	if handled {
		m.searchQuery = newText
		m.searchCursor = newCursor
		m.filter = m.searchQuery
		m.applyFilter()
	}
}

// StartContentSearch enters content search mode
func (m *ListModel) StartContentSearch() {
	m.contentSearching = true
	m.contentSearchQuery = ""
	m.contentSearchCursor = 0
	m.contentSearchResults = nil
}

// IsContentSearching returns true if in content search mode
func (m *ListModel) IsContentSearching() bool {
	return m.contentSearching
}

// CancelContentSearch exits content search mode
func (m *ListModel) CancelContentSearch() {
	m.contentSearching = false
	m.contentSearchQuery = ""
	m.contentSearchResults = nil
	m.applyFilter()
}

// ConfirmContentSearch selects the current result
// Does NOT rebuild filter - keeps selection intact for handleOpen
func (m *ListModel) ConfirmContentSearch() {
	m.contentSearching = false
	m.contentSearchResults = nil
	// Don't call applyFilter here - we want to keep the current selection
}

// GetSearchSnippet returns the search snippet for the currently selected item
func (m *ListModel) GetSearchSnippet() string {
	if !m.contentSearching || len(m.contentSearchResults) == 0 {
		return ""
	}
	item := m.SelectedItem()
	if item == nil || item.IsGroup() {
		return ""
	}
	// Find matching search result
	for _, r := range m.contentSearchResults {
		if r.Session.ID == item.Session.ID {
			return r.Snippet
		}
	}
	return ""
}

// HandleContentSearchKey processes a key during content search mode
// Returns true if search should be triggered
func (m *ListModel) HandleContentSearchKey(key string) bool {
	switch key {
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}
		return false
	case "down", "ctrl+n":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.ensureVisible()
		}
		return false
	}
	// Handle text input
	oldQuery := m.contentSearchQuery
	newText, newCursor, handled := handleTextInputKey(m.contentSearchQuery, m.contentSearchCursor, key)
	if handled {
		m.contentSearchQuery = newText
		m.contentSearchCursor = newCursor
		return newText != oldQuery // Trigger search only if query changed
	}
	return false
}

// ContentSearchResultsMsg carries async search results
type ContentSearchResultsMsg struct {
	Query   string
	Results []session.SearchResult
}

// RunContentSearch returns a command to execute the search async
func (m *ListModel) RunContentSearch() tea.Cmd {
	if len(m.contentSearchQuery) < 3 {
		m.contentSearchResults = nil
		m.applyFilter()
		return nil
	}
	query := m.contentSearchQuery
	sessions := m.manager.Sessions
	return func() tea.Msg {
		results := session.SearchContent(sessions, query)
		return ContentSearchResultsMsg{Query: query, Results: results}
	}
}

// HandleContentSearchResults processes async search results
func (m *ListModel) HandleContentSearchResults(msg ContentSearchResultsMsg) {
	// Only apply if query still matches
	if msg.Query == m.contentSearchQuery {
		m.contentSearchResults = msg.Results
		m.cursor = 0
		m.applyFilter()
	}
}

// View renders the list as a table
func (m *ListModel) View() string {
	visibleHeight := m.height
	if visibleHeight <= 0 {
		visibleHeight = 20
	}

	// Reserve 1 line for header, 1 for search if active
	dataHeight := visibleHeight - 1
	if m.searching || m.contentSearching {
		dataHeight--
	}

	// Calculate column widths - be conservative to prevent overflow
	// Name takes most space, date is fixed at 12 chars
	dateW := 12
	// Account for: prefix(3) + separator(" │ " = 3) + date(12) + margin(4)
	nameW := m.width - dateW - 3 - 3 - 4
	if nameW < 20 {
		nameW = 20
	}
	if nameW > 60 {
		nameW = 60 // Cap name width for readability
	}

	var lines []string

	// Header row - same format as data rows (pin + status + space = 3 chars prefix)
	headerText := padStr("Name", nameW) + " │ " + padStr("Date", dateW)
	header := "   " + helpStyle.Render(headerText)
	lines = append(lines, header)

	if len(m.filtered) == 0 && !m.creatingGroup {
		msg := "No sessions found"
		if m.filter != "" {
			msg = "No matching sessions"
		}
		lines = append(lines, "   "+itemStyle.Render(msg))
		for i := 1; i < dataHeight; i++ {
			lines = append(lines, "")
		}
	} else {
		for i := 0; i < dataHeight; i++ {
			// Show new group input line at position 0 when creating
			if m.creatingGroup && i == 0 {
				inputText := m.newGroupInput
				if m.newGroupCursor <= len(inputText) {
					inputText = inputText[:m.newGroupCursor] + "_" + inputText[m.newGroupCursor:]
				}
				lines = append(lines, "  + "+groupStyle.Render(padStr(inputText, nameW)))
				continue
			}

			// Adjust index when creating group (shift everything down by 1)
			listIdx := m.offset + i
			if m.creatingGroup {
				listIdx = m.offset + i - 1
			}

			if listIdx >= 0 && listIdx < len(m.filtered) {
				idx := m.filtered[listIdx]
				item := m.items[idx]
				isSelected := listIdx == m.cursor
				isHovered := listIdx == m.hoverIndex && !isSelected
				lines = append(lines, m.renderRow(item, isSelected, isHovered, nameW, dateW))
			} else {
				lines = append(lines, "") // Empty line to fill space
			}
		}
	}

	// Add search line at bottom (always at same position)
	if m.searching || m.contentSearching {
		var prefix, query string
		var cursor int
		if m.searching {
			prefix = "/"
			query = m.searchQuery
			cursor = m.searchCursor
		} else {
			prefix = "?"
			query = m.contentSearchQuery
			cursor = m.contentSearchCursor
		}
		searchText := query
		if cursor <= len(searchText) {
			searchText = searchText[:cursor] + "_" + searchText[cursor:]
		}
		lines = append(lines, " "+prefix+searchText)
	}

	return strings.Join(lines, "\n")
}

// padStr pads or truncates a string to EXACT width (never more, never less)
func padStr(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) > width {
		if width <= 2 {
			return s[:width]
		}
		return s[:width-2] + ".."
	}
	return s + strings.Repeat(" ", width-len(s))
}

// renderRow renders a single row in table format - MUST NOT exceed width
func (m *ListModel) renderRow(item ListItem, selected, hovered bool, nameW, dateW int) string {
	totalWidth := nameW + dateW + 6 // prefix(~4) + separator(3)

	if item.IsGroup() {
		arrow := "▶"
		if item.Group.Expanded {
			arrow = "▼"
		}
		name := truncate(item.Group.Name, nameW)

		var row string
		// Show rename input if renaming this group
		if selected && m.renaming {
			nameWithCursor := m.renameInput[:m.renameCursor] + "_" + m.renameInput[m.renameCursor:]
			row = " " + arrow + " " + padStr(nameWithCursor, nameW)
		} else if selected && m.deleting {
			// Show delete confirmation for group - X replaces arrow
			prompt := "Delete group? (y/n)"
			row = " X " + padStr(prompt, nameW)
		} else {
			row = " " + arrow + " " + name
		}

		// Apply style to entire row
		if selected {
			return selectedGroupStyle.Width(totalWidth).Render(row)
		}
		return groupStyle.Render(row)
	}

	// Session row
	s := item.Session

	// Tree prefix for items in groups
	treePrefix := ""
	prefixExtra := 0
	if item.Indent > 0 {
		if item.IsLastInTree {
			treePrefix = "└"
		} else {
			treePrefix = "├"
		}
		prefixExtra = 2
	}

	// Effective name width (smaller when tree prefix present)
	effectiveNameW := nameW - prefixExtra

	// Build name/date content
	var nameText string
	showDeleteX := false
	if selected && m.renaming {
		nameWithCursor := m.renameInput[:m.renameCursor] + "_" + m.renameInput[m.renameCursor:]
		nameText = padStr(nameWithCursor, effectiveNameW)
	} else if selected && m.deleting {
		nameText = padStr("Delete? (y/n)", effectiveNameW)
		showDeleteX = true
	} else {
		name := s.Name
		if name == "" {
			name = s.FolderName()
		}
		nameText = padStr(name, effectiveNameW)
	}
	dateText := padStr(s.LastAccessedAt.Format("Jan 2 15:04"), dateW)
	content := nameText + " │ " + dateText

	// Get status style - preserve color even when selected by adding background
	statusStyle := StatusStyle(s.Status.String())
	if selected {
		statusStyle = statusStyle.Background(surfaceColor)
	}
	statusSym := statusStyle.Render(s.Status.Symbol())

	// Build styled prefix parts
	var prefix string
	if item.Indent > 0 {
		if selected {
			prefix = selectedItemStyle.Render(" "+treePrefix)
		} else {
			prefix = " " + treePrefix
		}
	}

	// Pin symbol
	if s.Pinned {
		if selected {
			prefix += selectedItemStyle.Render("✦")
		} else {
			prefix += selectedItemStyle.Render("✦")
		}
	} else {
		if selected {
			prefix += selectedItemStyle.Render(" ")
		} else {
			prefix += " "
		}
	}

	// Status or X for delete
	if showDeleteX {
		if selected {
			prefix += selectedItemStyle.Render("X ")
		} else {
			prefix += "X "
		}
	} else {
		prefix += statusSym
		if selected {
			prefix += selectedItemStyle.Render(" ")
		} else {
			prefix += " "
		}
	}

	// Apply style to content
	if selected {
		// Calculate remaining width for content
		contentWidth := totalWidth - 4 - prefixExtra // rough prefix width
		if contentWidth < len(content) {
			contentWidth = len(content)
		}
		return prefix + selectedItemStyle.Width(contentWidth).Render(content)
	} else if hovered {
		return prefix + hoverItemStyle.Render(content)
	}
	return prefix + itemStyle.Render(content)
}

// truncateRow truncates a row to maxWidth, accounting for ANSI codes
func truncateRow(s string, maxWidth int) string {
	return lipgloss.NewStyle().MaxWidth(maxWidth).Render(s)
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// ListKeyMap defines the key bindings for the list
type ListKeyMap struct {
	Up            key.Binding
	Down          key.Binding
	ShiftUp       key.Binding
	ShiftDown     key.Binding
	Left          key.Binding
	Right         key.Binding
	Enter         key.Binding
	Search        key.Binding
	ContentSearch key.Binding
	Rename        key.Binding
	Delete        key.Binding
	Kill          key.Binding
	Move          key.Binding
	NewGroup      key.Binding
	NewSession    key.Binding
	Pin           key.Binding
	Layout        key.Binding
	Theme         key.Binding
	Resume        key.Binding
}

// DefaultListKeyMap returns the default key bindings
func DefaultListKeyMap() ListKeyMap {
	return ListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
		),
		ShiftUp: key.NewBinding(
			key.WithKeys("shift+up"),
			key.WithHelp("⇧↑", "up fast"),
		),
		ShiftDown: key.NewBinding(
			key.WithKeys("shift+down"),
			key.WithHelp("⇧↓", "down fast"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "collapse"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "expand"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		ContentSearch: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "search content"),
		),
		Rename: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "rename"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete group"),
		),
		Kill: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "kill session"),
		),
		Move: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "move"),
		),
		NewGroup: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "new group"),
		),
		NewSession: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "new session"),
		),
		Pin: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "pin"),
		),
		Layout: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "layout"),
		),
		Theme: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "theme"),
		),
		Resume: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "resume"),
		),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isWordChar returns true if the character is part of a word
func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// findWordBoundaryLeft finds the position of the previous word boundary
func findWordBoundaryLeft(s string, pos int) int {
	if pos <= 0 {
		return 0
	}
	// Skip any non-word chars first
	i := pos - 1
	for i > 0 && !isWordChar(s[i]) {
		i--
	}
	// Then skip word chars
	for i > 0 && isWordChar(s[i-1]) {
		i--
	}
	return i
}

// findWordBoundaryRight finds the position of the next word boundary
func findWordBoundaryRight(s string, pos int) int {
	if pos >= len(s) {
		return len(s)
	}
	// Skip current word chars
	i := pos
	for i < len(s) && isWordChar(s[i]) {
		i++
	}
	// Skip non-word chars
	for i < len(s) && !isWordChar(s[i]) {
		i++
	}
	return i
}

// handleTextInputKey processes a key for text input fields with word navigation support
// Returns: newText, newCursor, handled
func handleTextInputKey(text string, cursor int, key string) (string, int, bool) {
	switch key {
	case "left":
		if cursor > 0 {
			return text, cursor - 1, true
		}
	case "right":
		if cursor < len(text) {
			return text, cursor + 1, true
		}
	case "ctrl+a", "home": // Beginning of line
		return text, 0, true
	case "ctrl+e", "end": // End of line
		return text, len(text), true
	case "alt+left", "ctrl+left", "alt+b": // Word left
		return text, findWordBoundaryLeft(text, cursor), true
	case "alt+right", "ctrl+right", "alt+f": // Word right
		return text, findWordBoundaryRight(text, cursor), true
	case "backspace":
		if cursor > 0 {
			return text[:cursor-1] + text[cursor:], cursor - 1, true
		}
	case "alt+backspace", "ctrl+w": // Delete word backward
		if cursor > 0 {
			newPos := findWordBoundaryLeft(text, cursor)
			return text[:newPos] + text[cursor:], newPos, true
		}
	case "delete":
		if cursor < len(text) {
			return text[:cursor] + text[cursor+1:], cursor, true
		}
	case "alt+delete", "alt+d": // Delete word forward
		if cursor < len(text) {
			newPos := findWordBoundaryRight(text, cursor)
			return text[:cursor] + text[newPos:], cursor, true
		}
	case "ctrl+u": // Delete to beginning of line
		return text[cursor:], 0, true
	case "ctrl+k": // Delete to end of line
		return text[:cursor], cursor, true
	default:
		// Insert printable character
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			return text[:cursor] + key + text[cursor:], cursor + 1, true
		}
	}
	return text, cursor, false
}
