# Claude Deck

A TUI session manager for Claude Code. Discover, organize, and quickly resume your Claude Code conversations.

![Claude Deck](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)

## Features

- **Session Discovery** - Automatically finds all sessions from `~/.claude/projects`
- **Live Status** - Shows running/waiting/idle status via Kitty window tracking
- **Tab Name Sync** - Session names sync from Claude's tab titles automatically
- **Organization** - Groups, pinning, renaming, and custom ordering
- **Quick Resume** - Open sessions in new terminal tabs with `--resume`
- **Live Preview** - See conversation messages with real-time updates
- **Search** - Fuzzy search by name (`/`) or search within content (`?`)
- **Multi-Terminal** - Supports Kitty, iTerm2, Ghostty, and Terminal.app

## Installation

```bash
go install github.com/hadarshamir/claude-deck/cmd/claude-deck@latest
```

Or build from source:

```bash
git clone https://github.com/hadarshamir/claude-deck.git
cd claude-deck
make install  # Installs to ~/bin/
```

## Usage

```bash
deck
```

### Key Bindings

**Navigation**
| Key | Action |
|-----|--------|
| `↑` / `↓` | Move up/down |
| `Shift+↑` / `Shift+↓` | Move up/down fast |
| `←` / `→` | Collapse/expand group |
| `Tab` | Switch panel focus |

**Actions** (Shift + key)
| Key | Action |
|-----|--------|
| `Enter` | Open session in terminal |
| `N` | New session (pick folder) |
| `G` | Create new group |
| `R` | Rename session/group |
| `K` | Kill session (close tab) |
| `D` | Delete group |
| `M` | Move session to group |
| `P` | Pin/unpin session |

**Search**
| Key | Action |
|-----|--------|
| `/` | Search by name |
| `?` | Search in content |

**Settings**
| Key | Action |
|-----|--------|
| `L` | Toggle layout (side-by-side / stacked) |
| `T` | Select terminal emulator |
| `C` | Select color theme |
| `S` | Toggle resume on startup |

**Other**
| Key | Action |
|-----|--------|
| `Ctrl+R` | Refresh status and names |
| `H` | Show help |
| `Q` | Quit |

## How It Works

### Session Discovery

Sessions are discovered from Claude Code's data directory:
- Location: `~/.claude/projects/<encoded-path>/*.jsonl`
- Each `.jsonl` file is a session with UUID filename
- Project path is read from the `cwd` field in JSONL

### Status Detection

Session status is event-driven (no polling):
- **fsnotify** watches for JSONL file changes
- **Kitty** window IDs track which tab belongs to which session
- **Spinner detection** - Claude's tab title spinner indicates active work

Status states:
- 🟢 **Running** - Claude is actively working (spinner in tab title)
- 🟡 **Waiting** - Tab is open, waiting for input
- ⚫ **Idle** - No open tab

### Tab Name Sync

Session names automatically sync from Claude's tab titles:
- Strong matches via `--resume` flag or stored window ID
- Names sync on file changes and on startup
- User renames are preserved (won't be overwritten)

### Metadata Storage

Custom metadata is stored separately from Claude's data:
- Location: `~/.claude-sessions/sessions.json`
- Stores: names, groups, pins, window IDs, settings
- Claude's original data is never modified

## Development

```bash
make build    # Build to ./bin/claude-deck
make run      # Build and run
make test     # Run tests
make fmt      # Format code
make lint     # Run golangci-lint
make release  # Optimized release build
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Styling
- [fsnotify](https://github.com/fsnotify/fsnotify) - File watching
