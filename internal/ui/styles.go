package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines a color theme
type Theme struct {
	Name string

	// Base colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Muted     lipgloss.Color
	Text      lipgloss.Color
	Subtext   lipgloss.Color
	Overlay   lipgloss.Color
	Surface   lipgloss.Color
	Base      lipgloss.Color

	// Role colors
	UserRole      lipgloss.Color
	AssistantRole lipgloss.Color
}

// Available themes
var Themes = map[string]Theme{
	"Catppuccin Mocha": {
		Name:          "Catppuccin Mocha",
		Primary:       lipgloss.Color("#f5c2e7"), // pink
		Secondary:     lipgloss.Color("#74c7ec"), // sapphire
		Success:       lipgloss.Color("#a6e3a1"), // green
		Warning:       lipgloss.Color("#fab387"), // peach
		Error:         lipgloss.Color("#f38ba8"), // red
		Muted:         lipgloss.Color("#7f849c"), // overlay1
		Text:          lipgloss.Color("#cdd6f4"), // text
		Subtext:       lipgloss.Color("#bac2de"), // subtext1
		Overlay:       lipgloss.Color("#6c7086"), // overlay0
		Surface:       lipgloss.Color("#313244"), // surface0
		Base:          lipgloss.Color("#1e1e2e"), // base
		UserRole:      lipgloss.Color("#fab387"), // peach
		AssistantRole: lipgloss.Color("#b4befe"), // lavender
	},
	"Catppuccin Latte": {
		Name:          "Catppuccin Latte",
		Primary:       lipgloss.Color("#ea76cb"), // pink
		Secondary:     lipgloss.Color("#209fb5"), // sapphire
		Success:       lipgloss.Color("#40a02b"), // green
		Warning:       lipgloss.Color("#fe640b"), // peach
		Error:         lipgloss.Color("#d20f39"), // red
		Muted:         lipgloss.Color("#8c8fa1"), // overlay1
		Text:          lipgloss.Color("#4c4f69"), // text
		Subtext:       lipgloss.Color("#5c5f77"), // subtext1
		Overlay:       lipgloss.Color("#9ca0b0"), // overlay0
		Surface:       lipgloss.Color("#ccd0da"), // surface0
		Base:          lipgloss.Color("#eff1f5"), // base
		UserRole:      lipgloss.Color("#fe640b"), // peach
		AssistantRole: lipgloss.Color("#7287fd"), // lavender
	},
	"Catppuccin Frappe": {
		Name:          "Catppuccin Frappe",
		Primary:       lipgloss.Color("#f4b8e4"), // pink
		Secondary:     lipgloss.Color("#85c1dc"), // sapphire
		Success:       lipgloss.Color("#a6d189"), // green
		Warning:       lipgloss.Color("#ef9f76"), // peach
		Error:         lipgloss.Color("#e78284"), // red
		Muted:         lipgloss.Color("#838ba7"), // overlay1
		Text:          lipgloss.Color("#c6d0f5"), // text
		Subtext:       lipgloss.Color("#b5bfe2"), // subtext1
		Overlay:       lipgloss.Color("#737994"), // overlay0
		Surface:       lipgloss.Color("#414559"), // surface0
		Base:          lipgloss.Color("#303446"), // base
		UserRole:      lipgloss.Color("#ef9f76"), // peach
		AssistantRole: lipgloss.Color("#babbf1"), // lavender
	},
	"Dracula": {
		Name:          "Dracula",
		Primary:       lipgloss.Color("#ff79c6"), // pink
		Secondary:     lipgloss.Color("#8be9fd"), // cyan
		Success:       lipgloss.Color("#50fa7b"), // green
		Warning:       lipgloss.Color("#ffb86c"), // orange
		Error:         lipgloss.Color("#ff5555"), // red
		Muted:         lipgloss.Color("#6272a4"), // comment
		Text:          lipgloss.Color("#f8f8f2"), // foreground
		Subtext:       lipgloss.Color("#f8f8f2"),
		Overlay:       lipgloss.Color("#44475a"), // current line
		Surface:       lipgloss.Color("#44475a"),
		Base:          lipgloss.Color("#282a36"), // background
		UserRole:      lipgloss.Color("#ffb86c"), // orange
		AssistantRole: lipgloss.Color("#bd93f9"), // purple
	},
	"Nord": {
		Name:          "Nord",
		Primary:       lipgloss.Color("#88c0d0"), // nord8
		Secondary:     lipgloss.Color("#81a1c1"), // nord9
		Success:       lipgloss.Color("#a3be8c"), // nord14
		Warning:       lipgloss.Color("#ebcb8b"), // nord13
		Error:         lipgloss.Color("#bf616a"), // nord11
		Muted:         lipgloss.Color("#4c566a"), // nord3
		Text:          lipgloss.Color("#eceff4"), // nord6
		Subtext:       lipgloss.Color("#e5e9f0"), // nord5
		Overlay:       lipgloss.Color("#434c5e"), // nord2
		Surface:       lipgloss.Color("#3b4252"), // nord1
		Base:          lipgloss.Color("#2e3440"), // nord0
		UserRole:      lipgloss.Color("#ebcb8b"), // nord13
		AssistantRole: lipgloss.Color("#b48ead"), // nord15
	},
	"Tokyo Night": {
		Name:          "Tokyo Night",
		Primary:       lipgloss.Color("#bb9af7"), // purple
		Secondary:     lipgloss.Color("#7dcfff"), // cyan
		Success:       lipgloss.Color("#9ece6a"), // green
		Warning:       lipgloss.Color("#e0af68"), // yellow
		Error:         lipgloss.Color("#f7768e"), // red
		Muted:         lipgloss.Color("#565f89"), // comment
		Text:          lipgloss.Color("#c0caf5"), // foreground
		Subtext:       lipgloss.Color("#a9b1d6"),
		Overlay:       lipgloss.Color("#414868"),
		Surface:       lipgloss.Color("#24283b"),
		Base:          lipgloss.Color("#1a1b26"), // background
		UserRole:      lipgloss.Color("#e0af68"), // yellow
		AssistantRole: lipgloss.Color("#7aa2f7"), // blue
	},
	"Gruvbox Dark": {
		Name:          "Gruvbox Dark",
		Primary:       lipgloss.Color("#d3869b"), // purple
		Secondary:     lipgloss.Color("#83a598"), // aqua
		Success:       lipgloss.Color("#b8bb26"), // green
		Warning:       lipgloss.Color("#fe8019"), // orange
		Error:         lipgloss.Color("#fb4934"), // red
		Muted:         lipgloss.Color("#928374"), // gray
		Text:          lipgloss.Color("#ebdbb2"), // fg
		Subtext:       lipgloss.Color("#d5c4a1"),
		Overlay:       lipgloss.Color("#504945"), // bg2
		Surface:       lipgloss.Color("#3c3836"), // bg1
		Base:          lipgloss.Color("#282828"), // bg
		UserRole:      lipgloss.Color("#fe8019"), // orange
		AssistantRole: lipgloss.Color("#83a598"), // aqua
	},
	"Solarized Dark": {
		Name:          "Solarized Dark",
		Primary:       lipgloss.Color("#d33682"), // magenta
		Secondary:     lipgloss.Color("#2aa198"), // cyan
		Success:       lipgloss.Color("#859900"), // green
		Warning:       lipgloss.Color("#cb4b16"), // orange
		Error:         lipgloss.Color("#dc322f"), // red
		Muted:         lipgloss.Color("#586e75"), // base01
		Text:          lipgloss.Color("#839496"), // base0
		Subtext:       lipgloss.Color("#93a1a1"), // base1
		Overlay:       lipgloss.Color("#073642"), // base02
		Surface:       lipgloss.Color("#073642"),
		Base:          lipgloss.Color("#002b36"), // base03
		UserRole:      lipgloss.Color("#cb4b16"), // orange
		AssistantRole: lipgloss.Color("#6c71c4"), // violet
	},
}

// ThemeNames returns theme names in display order
var ThemeNames = []string{
	"Catppuccin Mocha",
	"Catppuccin Latte",
	"Catppuccin Frappe",
	"Dracula",
	"Nord",
	"Tokyo Night",
	"Gruvbox Dark",
	"Solarized Dark",
}

// Current theme colors (set by ApplyTheme)
var (
	primaryColor   lipgloss.Color
	secondaryColor lipgloss.Color
	successColor   lipgloss.Color
	warningColor   lipgloss.Color
	errorColor     lipgloss.Color
	mutedColor     lipgloss.Color
	textColor      lipgloss.Color
	subtextColor   lipgloss.Color
	overlayColor   lipgloss.Color
	surfaceColor   lipgloss.Color
	baseColor      lipgloss.Color
	userRoleColor  lipgloss.Color
	assistantRoleColor lipgloss.Color
)

// Styles (updated by ApplyTheme)
var (
	baseStyle             lipgloss.Style
	titleStyle            lipgloss.Style
	panelStyle            lipgloss.Style
	activePanelStyle      lipgloss.Style
	itemStyle             lipgloss.Style
	selectedItemStyle     lipgloss.Style
	hoverItemStyle        lipgloss.Style
	groupStyle            lipgloss.Style
	selectedGroupStyle    lipgloss.Style
	statusRunningStyle    lipgloss.Style
	statusWaitingStyle    lipgloss.Style
	statusIdleStyle       lipgloss.Style
	previewTitleStyle     lipgloss.Style
	previewMetaStyle      lipgloss.Style
	userMessageStyle      lipgloss.Style
	assistantMessageStyle lipgloss.Style
	userRoleStyle         lipgloss.Style
	assistantRoleStyle    lipgloss.Style
	inputStyle            lipgloss.Style
	helpStyle             lipgloss.Style
	helpKeyStyle          lipgloss.Style
	dialogStyle           lipgloss.Style
	dialogTitleStyle      lipgloss.Style
	buttonStyle           lipgloss.Style
	activeButtonStyle     lipgloss.Style
	searchStyle           lipgloss.Style
	searchPromptStyle     lipgloss.Style
	matchHighlightStyle   lipgloss.Style
)

// CurrentThemeName tracks the active theme
var CurrentThemeName = "Catppuccin Mocha"

func init() {
	ApplyTheme("Catppuccin Mocha")
}

// ApplyTheme applies a theme by name
func ApplyTheme(name string) {
	theme, ok := Themes[name]
	if !ok {
		theme = Themes["Catppuccin Mocha"]
		name = "Catppuccin Mocha"
	}
	CurrentThemeName = name

	// Set colors
	primaryColor = theme.Primary
	secondaryColor = theme.Secondary
	successColor = theme.Success
	warningColor = theme.Warning
	errorColor = theme.Error
	mutedColor = theme.Muted
	textColor = theme.Text
	subtextColor = theme.Subtext
	overlayColor = theme.Overlay
	surfaceColor = theme.Surface
	baseColor = theme.Base
	userRoleColor = theme.UserRole
	assistantRoleColor = theme.AssistantRole

	// Update all styles
	baseStyle = lipgloss.NewStyle()

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(mutedColor).
		Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
		Foreground(textColor)

	selectedItemStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	hoverItemStyle = lipgloss.NewStyle().
		Foreground(secondaryColor)

	groupStyle = lipgloss.NewStyle().
		Foreground(secondaryColor).
		Bold(true)

	selectedGroupStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	statusRunningStyle = lipgloss.NewStyle().
		Foreground(successColor)

	statusWaitingStyle = lipgloss.NewStyle().
		Foreground(warningColor)

	statusIdleStyle = lipgloss.NewStyle().
		Foreground(overlayColor)

	previewTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor)

	previewMetaStyle = lipgloss.NewStyle().
		Foreground(mutedColor).
		Italic(true)

	userMessageStyle = lipgloss.NewStyle().
		Foreground(userRoleColor)

	assistantMessageStyle = lipgloss.NewStyle().
		Foreground(subtextColor)

	userRoleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(userRoleColor)

	assistantRoleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(assistantRoleColor)

	inputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
		Foreground(mutedColor)

	helpKeyStyle = lipgloss.NewStyle().
		Foreground(secondaryColor)

	dialogStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Align(lipgloss.Center)

	dialogTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(1)

	buttonStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Background(mutedColor).
		Foreground(textColor)

	activeButtonStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Background(primaryColor).
		Foreground(lipgloss.Color("#FFFFFF"))

	searchStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(secondaryColor).
		Padding(0, 1)

	searchPromptStyle = lipgloss.NewStyle().
		Foreground(secondaryColor)

	matchHighlightStyle = lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true)
}

// StatusStyle returns the appropriate style for a status
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return statusRunningStyle
	case "waiting":
		return statusWaitingStyle
	default:
		return statusIdleStyle
	}
}

// RoleStyle returns the style for a message role
func RoleStyle(role string) lipgloss.Style {
	if role == "user" {
		return userMessageStyle
	}
	return assistantMessageStyle
}
