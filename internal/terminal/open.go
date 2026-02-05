package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

// OpenSession opens a Claude session in a new kitty tab, or focuses existing tab
// Returns the kitty window ID
func OpenSession(projectPath, sessionID string, activeWindowID int, tabTitle string) (int, error) {
	// If session already has an active tab, focus it instead of opening new one
	if activeWindowID > 0 {
		if err := focusKittyWindow(activeWindowID); err == nil {
			return activeWindowID, nil
		}
		// Fall through to open new tab if focus fails
	}

	claudeCmd := fmt.Sprintf("cd %q && claude --resume %s", projectPath, sessionID)
	return openInKitty(claudeCmd, projectPath, tabTitle)
}

// NewSession opens a new Claude session in a new kitty tab
// Returns the kitty window ID
func NewSession(projectPath string, tabTitle string) (int, error) {
	claudeCmd := fmt.Sprintf("cd %q && claude", projectPath)
	return openInKitty(claudeCmd, projectPath, tabTitle)
}

// openInKitty opens a new tab in Kitty and returns the window ID
func openInKitty(command string, workDir string, tabTitle string) (int, error) {
	wrappedCmd := fmt.Sprintf("%s; exec zsh", command)
	args := []string{"@", "launch", "--type=tab", "--cwd", workDir}
	if tabTitle != "" {
		args = append(args, "--tab-title", tabTitle)
	}
	args = append(args, "zsh", "-i", "-c", wrappedCmd)
	cmd := exec.Command("kitty", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("kitty launch failed: %v", err)
	}
	var windowID int
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &windowID)
	return windowID, nil
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

// SetKittyTabTitle renames a kitty tab by window ID
func SetKittyTabTitle(windowID int, title string) error {
	if windowID <= 0 || title == "" {
		return nil
	}
	cmd := exec.Command("kitty", "@", "set-tab-title", "--match", fmt.Sprintf("window_id:%d", windowID), title)
	return cmd.Run()
}

// ResetKittyTabTitle resets the tab title to use the window's dynamic title
func ResetKittyTabTitle(windowID int) error {
	if windowID <= 0 {
		return nil
	}
	cmd := exec.Command("kitty", "@", "set-tab-title", "--match", fmt.Sprintf("window_id:%d", windowID))
	return cmd.Run()
}
