package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DialogType represents different dialog types
type DialogType int

const (
	DialogNone DialogType = iota
	DialogRename
	DialogNewGroup
	DialogRenameGroup
	DialogDelete
	DialogMove
	DialogTerminal
)

// TerminalOption represents a terminal choice
type TerminalOption struct {
	Name   string
	Active bool // true if this is the currently selected terminal
}

// DialogModel manages modal dialogs
type DialogModel struct {
	dialogType     DialogType
	input          textinput.Model
	title          string
	message        string
	targetID       string
	targetName     string
	confirmFocus   bool
	groups         []GroupOption
	groupCursor    int
	terminals      []TerminalOption
	terminalCursor int
}

// GroupOption represents a group choice in the move dialog
type GroupOption struct {
	ID   string
	Name string
	Path string
}

// NewDialogModel creates a new dialog model
func NewDialogModel() *DialogModel {
	ti := textinput.New()
	ti.CharLimit = 100
	ti.Width = 30

	return &DialogModel{
		input: ti,
	}
}

// OpenRename opens the rename dialog
func (m *DialogModel) OpenRename(id, currentName string, isGroup bool) {
	if isGroup {
		m.dialogType = DialogRenameGroup
		m.title = "Rename Group"
	} else {
		m.dialogType = DialogRename
		m.title = "Rename Session"
	}
	m.targetID = id
	m.targetName = currentName
	m.input.SetValue(currentName)
	m.input.Focus()
	m.input.CursorEnd()
}

// OpenNewGroup opens the new group dialog
func (m *DialogModel) OpenNewGroup() {
	m.dialogType = DialogNewGroup
	m.title = "New Group"
	m.input.SetValue("")
	m.input.Placeholder = "Group name"
	m.input.Focus()
}

// OpenDelete opens the delete confirmation dialog
func (m *DialogModel) OpenDelete(id, name string, isGroup bool) {
	m.dialogType = DialogDelete
	if isGroup {
		m.title = "Delete Group"
		m.message = "Delete group \"" + name + "\"?\nSessions will be moved to root."
	} else {
		m.title = "Delete Session"
		m.message = "Remove \"" + name + "\" from list?\n(Claude's data will not be deleted)"
	}
	m.targetID = id
	m.targetName = name
	m.confirmFocus = false
}

// OpenMove opens the move to group dialog
func (m *DialogModel) OpenMove(id, name string, groups []GroupOption) {
	m.dialogType = DialogMove
	m.title = "Move Session"
	m.targetID = id
	m.targetName = name
	m.groups = append([]GroupOption{{ID: "", Name: "(No Group)", Path: ""}}, groups...)
	m.groupCursor = 0
}

// OpenTerminal opens the terminal selection dialog
func (m *DialogModel) OpenTerminal(terminals []TerminalOption) {
	m.dialogType = DialogTerminal
	m.title = "Select Terminal"
	m.terminals = terminals
	m.terminalCursor = 0
	// Set cursor to currently active terminal
	for i, t := range terminals {
		if t.Active {
			m.terminalCursor = i
			break
		}
	}
}

// SelectedTerminal returns the selected terminal name
func (m *DialogModel) SelectedTerminal() string {
	if m.dialogType != DialogTerminal || len(m.terminals) == 0 {
		return ""
	}
	return m.terminals[m.terminalCursor].Name
}

// Close closes the dialog
func (m *DialogModel) Close() {
	m.dialogType = DialogNone
	m.input.Blur()
}

// IsOpen returns whether a dialog is open
func (m *DialogModel) IsOpen() bool {
	return m.dialogType != DialogNone
}

// Type returns the current dialog type
func (m *DialogModel) Type() DialogType {
	return m.dialogType
}

// TargetID returns the target item ID
func (m *DialogModel) TargetID() string {
	return m.targetID
}

// Value returns the input value
func (m *DialogModel) Value() string {
	return m.input.Value()
}

// SelectedGroup returns the selected group for move dialog
func (m *DialogModel) SelectedGroup() *GroupOption {
	if m.dialogType != DialogMove || len(m.groups) == 0 {
		return nil
	}
	return &m.groups[m.groupCursor]
}

// Confirm returns whether user confirmed (for delete dialog)
func (m *DialogModel) Confirm() bool {
	return m.confirmFocus
}

// Update handles messages for the dialog
func (m *DialogModel) Update(msg tea.Msg) (*DialogModel, tea.Cmd) {
	if !m.IsOpen() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Close()
			return m, nil

		case "enter":
			// Don't close here - let the app handle the result
			return m, nil

		case "tab", "left", "right":
			if m.dialogType == DialogDelete {
				m.confirmFocus = !m.confirmFocus
			}
			return m, nil

		case "up":
			if m.dialogType == DialogMove && m.groupCursor > 0 {
				m.groupCursor--
			}
			if m.dialogType == DialogTerminal && m.terminalCursor > 0 {
				m.terminalCursor--
			}
			return m, nil

		case "down":
			if m.dialogType == DialogMove && m.groupCursor < len(m.groups)-1 {
				m.groupCursor++
			}
			if m.dialogType == DialogTerminal && m.terminalCursor < len(m.terminals)-1 {
				m.terminalCursor++
			}
			return m, nil
		}
	}

	// Forward to text input for rename/new group dialogs
	if m.dialogType == DialogRename || m.dialogType == DialogNewGroup || m.dialogType == DialogRenameGroup {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the dialog
func (m *DialogModel) View() string {
	if !m.IsOpen() {
		return ""
	}

	var content string

	switch m.dialogType {
	case DialogRename, DialogNewGroup, DialogRenameGroup:
		content = m.renderInputDialog()
	case DialogDelete:
		content = m.renderDeleteDialog()
	case DialogMove:
		content = m.renderMoveDialog()
	case DialogTerminal:
		content = m.renderTerminalDialog()
	}

	return dialogStyle.Render(content)
}

// renderInputDialog renders rename/new group dialog
func (m *DialogModel) renderInputDialog() string {
	title := dialogTitleStyle.Render(m.title)
	input := inputStyle.Render(m.input.View())
	hint := helpStyle.Render("Enter to confirm • Esc to cancel")

	return lipgloss.JoinVertical(lipgloss.Center, title, "", input, "", hint)
}

// renderDeleteDialog renders the delete confirmation dialog
func (m *DialogModel) renderDeleteDialog() string {
	title := dialogTitleStyle.Render(m.title)
	message := m.message

	var cancelBtn, confirmBtn string
	if m.confirmFocus {
		cancelBtn = buttonStyle.Render(" Cancel ")
		confirmBtn = activeButtonStyle.Render(" Delete ")
	} else {
		cancelBtn = activeButtonStyle.Render(" Cancel ")
		confirmBtn = buttonStyle.Render(" Delete ")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, cancelBtn, "  ", confirmBtn)
	hint := helpStyle.Render("Tab to switch • Enter to confirm")

	return lipgloss.JoinVertical(lipgloss.Center, title, "", message, "", buttons, "", hint)
}

// renderMoveDialog renders the move to group dialog
func (m *DialogModel) renderMoveDialog() string {
	title := dialogTitleStyle.Render(m.title + ": " + m.targetName)

	var groupLines []string
	for i, g := range m.groups {
		cursor := "  "
		style := itemStyle
		if i == m.groupCursor {
			cursor = "> "
			style = selectedItemStyle
		}
		groupLines = append(groupLines, cursor+style.Render(g.Name))
	}

	groupList := lipgloss.JoinVertical(lipgloss.Left, groupLines...)
	hint := helpStyle.Render("j/k to navigate • Enter to move • Esc to cancel")

	return lipgloss.JoinVertical(lipgloss.Center, title, "", groupList, "", hint)
}

// renderTerminalDialog renders the terminal selection dialog
func (m *DialogModel) renderTerminalDialog() string {
	title := dialogTitleStyle.Render(m.title)

	var terminalLines []string
	for i, t := range m.terminals {
		cursor := "  "
		style := itemStyle
		if i == m.terminalCursor {
			cursor = "> "
			style = selectedItemStyle
		}
		name := t.Name
		if t.Active {
			name += " (current)"
		}
		terminalLines = append(terminalLines, cursor+style.Render(name))
	}

	terminalList := lipgloss.JoinVertical(lipgloss.Left, terminalLines...)
	hint := helpStyle.Render("j/k to navigate • Enter to select • Esc to cancel")

	return lipgloss.JoinVertical(lipgloss.Center, title, "", terminalList, "", hint)
}
