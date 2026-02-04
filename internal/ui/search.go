package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SearchModel manages the search overlay
type SearchModel struct {
	input   textinput.Model
	active  bool
	onClose func(query string)
}

// NewSearchModel creates a new search model
func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search sessions..."
	ti.CharLimit = 100
	ti.Width = 40

	return &SearchModel{
		input: ti,
	}
}

// Open activates the search overlay
func (m *SearchModel) Open() {
	m.active = true
	m.input.SetValue("")
	m.input.Focus()
}

// Close deactivates the search overlay
func (m *SearchModel) Close() {
	m.active = false
	m.input.Blur()
}

// IsActive returns whether search is active
func (m *SearchModel) IsActive() bool {
	return m.active
}

// Query returns the current search query
func (m *SearchModel) Query() string {
	return m.input.Value()
}

// SetWidth sets the input width
func (m *SearchModel) SetWidth(width int) {
	m.input.Width = width - 10
}

// Update handles messages for the search input
func (m *SearchModel) Update(msg tea.Msg) (*SearchModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the search input
func (m *SearchModel) View() string {
	if !m.active {
		return ""
	}

	prompt := searchPromptStyle.Render("/")
	return searchStyle.Render(prompt + m.input.View())
}
