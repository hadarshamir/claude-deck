package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Terminal represents a terminal emulator type
type Terminal int

const (
	TerminalUnknown Terminal = iota
	TerminalITerm2
	TerminalGhostty
	TerminalApple
	TerminalKitty
	TerminalAuto // Auto-detect
)

var terminalNames = map[Terminal]string{
	TerminalITerm2:  "iTerm2",
	TerminalGhostty: "Ghostty",
	TerminalApple:   "Terminal",
	TerminalKitty:   "Kitty",
	TerminalAuto:    "Auto",
}

func (t Terminal) String() string {
	if name, ok := terminalNames[t]; ok {
		return name
	}
	return "Unknown"
}

// AllTerminals returns all available terminal options (for settings UI)
func AllTerminals() []Terminal {
	return []Terminal{TerminalAuto, TerminalITerm2, TerminalGhostty, TerminalKitty, TerminalApple}
}

// ParseTerminal converts a string to Terminal type
func ParseTerminal(s string) Terminal {
	for t, name := range terminalNames {
		if strings.EqualFold(name, s) {
			return t
		}
	}
	return TerminalAuto
}

// preferredTerminal stores the user's terminal preference (default: auto-detect)
var preferredTerminal = TerminalAuto

// SetPreferredTerminal sets the preferred terminal
func SetPreferredTerminal(t Terminal) {
	preferredTerminal = t
}

// GetPreferredTerminal returns the current preferred terminal
func GetPreferredTerminal() Terminal {
	return preferredTerminal
}

// DetectTerminal determines which terminal emulator is running
func DetectTerminal() Terminal {
	// Check TERM_PROGRAM environment variable
	termProgram := os.Getenv("TERM_PROGRAM")

	switch strings.ToLower(termProgram) {
	case "iterm.app":
		return TerminalITerm2
	case "ghostty":
		return TerminalGhostty
	case "apple_terminal":
		return TerminalApple
	case "kitty":
		return TerminalKitty
	}

	// Check if iTerm2 is running
	if isAppRunning("iTerm") {
		return TerminalITerm2
	}

	// Check if Ghostty is running
	if isAppRunning("Ghostty") {
		return TerminalGhostty
	}

	// Check if Kitty is running
	if isAppRunning("kitty") {
		return TerminalKitty
	}

	// Default to Apple Terminal
	return TerminalApple
}

// GetEffectiveTerminal returns the terminal to use (preferred or auto-detected)
func GetEffectiveTerminal() Terminal {
	if preferredTerminal != TerminalAuto {
		return preferredTerminal
	}
	return DetectTerminal()
}

// isAppRunning checks if an application is running
func isAppRunning(appName string) bool {
	cmd := exec.Command("pgrep", "-x", appName)
	return cmd.Run() == nil
}

// OpenSession opens a Claude session in a new terminal tab, or focuses existing tab
// Returns the kitty window ID if opened in kitty (0 otherwise)
// tabTitle is optional - if provided, sets the kitty tab title
func OpenSession(projectPath, sessionID string, activeWindowID int, tabTitle string) (int, error) {
	terminal := GetEffectiveTerminal()

	// If session already has an active tab, focus it instead of opening new one
	if activeWindowID > 0 && terminal == TerminalKitty {
		if err := focusKittyWindow(activeWindowID); err == nil {
			return activeWindowID, nil
		}
		// Fall through to open new tab if focus fails
	}

	// Build the command to run
	claudeCmd := fmt.Sprintf("cd %q && claude --resume %s", projectPath, sessionID)

	switch terminal {
	case TerminalITerm2:
		return 0, openInITerm2(claudeCmd)
	case TerminalGhostty:
		return 0, openInGhostty(claudeCmd, projectPath)
	case TerminalKitty:
		return openInKittyWithID(claudeCmd, projectPath, tabTitle)
	default:
		return 0, openInAppleTerminal(claudeCmd)
	}
}

// focusKittyWindow focuses an existing kitty window by ID
func focusKittyWindow(windowID int) error {
	cmd := exec.Command("kitty", "@", "focus-window", "--match", fmt.Sprintf("id:%d", windowID))
	return cmd.Run()
}

// CloseKittyWindow closes a kitty window by ID
func CloseKittyWindow(windowID int) error {
	if windowID <= 0 {
		return nil
	}
	cmd := exec.Command("kitty", "@", "close-window", "--match", fmt.Sprintf("id:%d", windowID))
	return cmd.Run()
}

// openInKittyWithID opens a new tab in Kitty and returns the window ID
func openInKittyWithID(command string, workDir string, tabTitle string) (int, error) {
	wrappedCmd := fmt.Sprintf("%s; exec zsh", command)
	args := []string{"@", "launch", "--type=tab", "--cwd", workDir}
	if tabTitle != "" {
		args = append(args, "--tab-title", tabTitle)
	}
	args = append(args, "zsh", "-i", "-c", wrappedCmd)
	cmd := exec.Command("kitty", args...)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to AppleScript (no window ID available)
		return 0, openInKittyAppleScript(command)
	}
	// kitty @ launch returns the window ID
	var windowID int
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &windowID)
	return windowID, nil
}

// SetKittyTabTitle renames a kitty tab by window ID
func SetKittyTabTitle(windowID int, title string) error {
	if windowID <= 0 || title == "" {
		return nil
	}
	cmd := exec.Command("kitty", "@", "set-tab-title", "--match", fmt.Sprintf("id:%d", windowID), title)
	return cmd.Run()
}

// openInITerm2 opens a new tab in iTerm2
func openInITerm2(command string) error {
	script := fmt.Sprintf(`
tell application "iTerm"
    activate
    tell current window
        create tab with default profile
        tell current session
            write text %q
        end tell
    end tell
end tell
`, command)

	return runAppleScript(script)
}

// openInGhostty opens a new tab in Ghostty
func openInGhostty(command string, workDir string) error {
	// Copy command to clipboard first (keystroke has issues with special chars)
	copyCmd := exec.Command("sh", "-c", fmt.Sprintf("echo %q | pbcopy", command))
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy command to clipboard: %v", err)
	}

	// Use AppleScript to open new tab and paste
	script := `
tell application "Ghostty"
    activate
end tell
delay 0.1
tell application "System Events"
    tell process "Ghostty"
        keystroke "t" using command down
        delay 0.3
        keystroke "v" using command down
        delay 0.1
        keystroke return
    end tell
end tell
`
	return runAppleScript(script)
}

// openInKittyAppleScript opens a new tab in Kitty using AppleScript (fallback)
func openInKittyAppleScript(command string) error {
	// Wrap command to keep tab open: command; exec zsh
	wrappedCmd := fmt.Sprintf("%s; exec zsh", command)

	// Copy command to clipboard first
	copyCmd := exec.Command("sh", "-c", fmt.Sprintf("echo %q | pbcopy", wrappedCmd))
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy command to clipboard: %v", err)
	}

	script := `
tell application "kitty"
    activate
end tell
delay 0.1
tell application "System Events"
    tell process "kitty"
        keystroke "t" using command down
        delay 0.3
        keystroke "v" using command down
        delay 0.1
        keystroke return
    end tell
end tell
`
	return runAppleScript(script)
}

// openInAppleTerminal opens a new tab in Apple Terminal
func openInAppleTerminal(command string) error {
	script := fmt.Sprintf(`
tell application "Terminal"
    activate
    tell application "System Events"
        keystroke "t" using command down
    end tell
    delay 0.2
    do script %q in front window
end tell
`, command)

	return runAppleScript(script)
}

// runAppleScript executes an AppleScript
func runAppleScript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript failed: %v, output: %s", err, output)
	}
	return nil
}

// NewSession opens a new Claude session in a new terminal tab
// tabTitle is optional - if provided, sets the kitty tab title
// Returns the kitty window ID if opened in kitty (0 otherwise)
func NewSession(projectPath string, tabTitle string) (int, error) {
	terminal := GetEffectiveTerminal()

	// Build the command to run
	claudeCmd := fmt.Sprintf("cd %q && claude", projectPath)

	switch terminal {
	case TerminalITerm2:
		return 0, openInITerm2(claudeCmd)
	case TerminalGhostty:
		return 0, openInGhostty(claudeCmd, projectPath)
	case TerminalKitty:
		return openInKittyWithID(claudeCmd, projectPath, tabTitle)
	default:
		return 0, openInAppleTerminal(claudeCmd)
	}
}
