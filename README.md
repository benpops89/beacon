<h1 align="center">Beacon</h1>

<p align="center">
  <strong>Lightweight status tracker for opencode sessions running in tmux</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/TypeScript-Bun-FF6B6B?style=flat-square&logo=typescript&logoColor=white" alt="TypeScript">
  <img src="https://img.shields.io/badge/tmux-Integration-1DB954?style=flat-square&logo=tmux&logoColor=white" alt="tmux">
</p>

<p align="center">
  🧠 Plan &nbsp;|&nbsp; 🔨 Build &nbsp;|&nbsp; ⏳ Unknown &nbsp;|&nbsp; 🔔 Input Required
</p>

---

## Overview

Beacon is a lightweight, inbox-style status tracker designed for developers who run opencode sessions within tmux. It provides a minimal status bar indicator showing which agents are running and which sessions need your attention.

### Features

- **Agent-aware icons**: Instantly see whether the plan or build agent is running
- **Input alerts**: Get notified when a session requires your input
- **4-hour stale protection**: Automatically ignores outdated status files
- **Tmux integration**: Seamless status bar updates every 2 seconds
- **Television integration**: Fuzzy-search session switching with `tv beacon`
- **Minimal footprint**: Two simple components - a TypeScript plugin and a Go binary

## Status States

| State | Icon | Description |
|-------|------|-------------|
| Plan mode | 🧠 | Plan agent is running |
| Build mode | 🔨 | Build agent is running |
| Unknown | ⏳ | Agent running, mode unidentified |
| Input required | 🔔 | Session is waiting for your input |
| Finished | ✅ | Session marked as complete |
| Idle | 💤 | Session stale (>4h without update) |

## Installation

### 1. Build the Go Binary

```bash
cd consumer
go build -o beacon .
cp beacon ~/.local/bin/
```

### 2. Register the opencode Plugin

Add the plugin path to your opencode configuration:

```json
// ~/.config/opencode/opencode.json
{
  "plugin": [
    "/path/to/beacon/writer"
  ]
}
```

### 3. Configure tmux

Add the following to your `~/.tmux.conf`:

```tmux
# Refresh status bar every 2 seconds
set -g status-interval 2

# Display beacon status alongside gitmux
set -g status-right "#(beacon status) #[fg=white,nobold]#(gitmux -cfg $HOME/.config/tmux/gitmux.yml)"
```

Reload tmux configuration:

```bash
tmux source ~/.config/tmux/tmux.conf
```

## Usage

Beacon operates automatically once installed. The `beacon_finish` tool is available in all opencode sessions:

```
Use beacon_finish when you've completed a task to mark it as finished
```

### Status Files

Status data is stored in `~/.local/share/beacon/`:

```
~/.local/share/beacon/{session_name}.json
```

Example content:

```json
{
  "status": "running",
  "agent": "plan",
  "session_name": "github/beacon",
  "updated_at": "2026-03-20T12:00:00Z"
}
```

### Command Line

```bash
beacon status              # Output tmux status bar format
beacon list               # List sessions (plain names)
beacon list --icons       # List sessions with icons
beacon switch <session>   # Switch to session and mark as idle
```

## Interactive Session Switching (Television)

Beacon integrates with [Television](https://github.com/alexpasmantier/television) for fuzzy-search session switching.

### Installation

```bash
# Install Television
curl -fsSL https://alexpasmantier.github.io/television/install.sh | bash
```

### Setup

Create `~/.config/television/cable/beacon.toml`:

```toml
[metadata]
name = "beacon"
description = "Switch to opencode sessions"

[source]
command = "beacon list --icons"

[keybindings]
enter = "actions:open"

[actions.open]
command = "beacon switch '{strip_ansi|split: :1..|join: }'"
mode = "execute"
```

### Usage

```bash
tv beacon
```

Or bind to a key in your shell for quick access:

```bash
# Zsh - add to ~/.zshrc
echo 'eval "$(tv init zsh)"' >> ~/.zshrc
bindkey -s '^b' 'tv beacon\n'
```

Now press `Ctrl+b` to fuzzy-search and switch sessions.

When you select a session, beacon will:
1. Switch to that tmux session
2. Mark the session as "idle" (removing the alert 🔔 from the status bar)

## Architecture

```
beacon/
├── writer/              # opencode plugin (TypeScript/Bun)
│   ├── index.ts        # Main plugin logic
│   └── index.test.ts   # Unit tests
└── consumer/           # Go CLI binary
    ├── main.go         # Entry point, bar formatting, list mode
    └── main_test.go    # Unit tests
```

### Writer (TypeScript Plugin)

The opencode plugin hooks into lifecycle events to track session state:

- **Message events**: Captures the current agent mode (plan/build)
- **Session status**: Updates status to `running` when busy, `input_required` when idle
- **Permission updates**: Sets `input_required` when user input is needed
- **beacon_finish tool**: Allows the agent to explicitly mark tasks complete

### Consumer (Go Binary)

The Go binary scans status files and formats output:

- **status**: Outputs icons for tmux status bar (ignores stale files)
- **list**: Lists all sessions for Television integration (includes stale)
- **list --icons**: Lists sessions with status icons
- **switch <session>**: Switches to a session and marks it as idle
- **Stale filtering**: Ignores files older than 4 hours (status mode only)
- **Icon mapping**: Translates agent types to appropriate icons
- **tmux formatting**: Outputs status-agnostic color codes

## Configuration

### Stale Duration

Modify the `staleDuration` constant in `consumer/main.go`:

```go
const staleDuration = 4 * time.Hour
```

### Icons and Colors

Icons and colors are defined as structs for easy customization:

```go
var icon = Icons{
    Plan:          "🧠",
    Build:         "🔨",
    Unknown:       "⏳",
    InputRequired: "🔔",
}

var color = Colors{
    Running: "#[fg=cyan]",
    Alert:   "#[fg=red,bold]",
}
```

## Testing

### Writer Tests

```bash
cd writer
bun test
```

### Consumer Tests

```bash
cd consumer
go test -v
```

## License

MIT
