# Claude Deck

A TUI application for managing Claude Code sessions. Discover, organize, and quickly resume your Claude Code conversations.

## Features

- **Session Discovery**: Automatically finds sessions from `~/.claude/projects`
- **Organization**: Group sessions into folders, rename, and reorder
- **Quick Resume**: Open sessions in new terminal tabs (iTerm2, Ghostty, Terminal.app)
- **Live Preview**: See recent conversation messages with live updates
- **Status Detection**: Shows if sessions are running, waiting, or idle
- **Fuzzy Search**: Quickly find sessions with `/`

## Installation

```bash
# Install Go if not installed
brew install go

# Clone and build
cd ~/Projects/claude-deck
make build

# Install to ~/bin
make install
```

## Usage

```bash
claude-deck
```

### Key Bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` | Go to top |
| `G` | Go to bottom |
| `h` / `←` | Collapse group |
| `l` / `→` | Expand group |
| `Enter` | Open session in new tab |
| `n` | New session (same project) |
| `/` | Search sessions |
| `r` | Rename session/group |
| `d` | Delete session/group |
| `m` | Move session to group |
| `N` | Create new group |
| `R` | Refresh list |
| `?` | Show help |
| `q` | Quit |

## How It Works

### Session Discovery

Sessions are discovered from Claude Code's data directory:
- Location: `~/.claude/projects/<encoded-path>/*.jsonl`
- Directory names are encoded paths (e.g., `-Users-hadar-project`)
- Each `.jsonl` file is a session with UUID filename

### Metadata Storage

Custom metadata (names, groups, order) is stored separately:
- Location: `~/.claude-sessions/sessions.json`
- Claude's original data is never modified

### Status Detection

Session status is determined by:
1. Process detection: `pgrep -f "claude.*<project-path>"`
2. CPU usage: High CPU = running, low CPU = waiting
3. Fallback: File modification time

### Terminal Integration

Sessions open in new terminal tabs via AppleScript:
- Supports iTerm2, Ghostty, and Terminal.app
- Auto-detects current terminal
- Runs: `cd <project> && claude --resume <session-id>`

## Development

```bash
# Run during development
make run

# Format code
make fmt

# Run tests
make test

# Build optimized release binary
make release
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Styling
- [fsnotify](https://github.com/fsnotify/fsnotify) - File watching
