# Phantom Developer Dashboard

Phantom is a TUI (Terminal User Interface) developer dashboard for managing your workspace, running system tools, sending HTTP requests, and more—all from your terminal.

## Directory Structure

```
.
├── config.lua                # User configuration (panels, HTTP templates, commands)
├── debug.log                 # Log file (created at runtime)
├── go.mod, go.sum            # Go module files
├── cmd/
│   └── phantom/
│       └── main.go           # Application entry point
├── internal/
│   ├── app/                  # App-level utilities (binary checks, etc.)
│   │   └── app.go
│   ├── config/               # Loads and parses config.lua
│   │   └── config.go
│   ├── ui/
│   │   ├── model.go          # Main TUI model (tab management, layout)
│   │   ├── components/
│   │   │   ├── launcher/
│   │   │   │   └── launcher.go   # Launcher for external tools (lazygit, lazydocker, etc.)
│   │   │   └── styles/
│   │   │       └── styles.go     # Lipgloss styles for UI
│   │   └── tabs/
│   │       ├── dashboard/
│   │       │   └── dashboard.go  # System stats, process list
│   │       ├── docker/
│   │       │   └── docker.go     # Docker panel (lazydocker)
│   │       ├── git/
│   │       │   └── git.go        # Git panel (lazygit)
│   │       ├── http/
│   │       │   └── http.go       # HTTP client panel
│   │       ├── kind/
│   │       │   └── kind.go       # Kubernetes Kind cluster management
│   │       └── nvim/
│   │           └── nvim.go       # Neovim launcher
│   └── utils/
│       └── utils.go              # Utility functions (formatting, JSON pretty print)
```

## Features

- **Dashboard:** View CPU, memory, disk usage, and running processes.
- **HTTP Client:** Send HTTP requests, manage collections, view responses.
- **Git & Docker:** Launch [lazygit](https://github.com/jesseduffield/lazygit) and [lazydocker](https://github.com/jesseduffield/lazydocker) from the dashboard.
- **Kind:** Manage local Kubernetes clusters with [kind](https://kind.sigs.k8s.io/).
- **Neovim:** Launch Neovim directly from the dashboard.
- **Custom Panels:** Add your own panels via `config.lua` (e.g., clock, Docker logs).

## Installation

### Prerequisites

- [Go 1.21+](https://golang.org/dl/)
- [lazygit](https://github.com/jesseduffield/lazygit) (optional, for Git panel)
- [lazydocker](https://github.com/jesseduffield/lazydocker) (optional, for Docker panel)
- [kind](https://kind.sigs.k8s.io/) (optional, for Kubernetes panel)
- [neovim](https://neovim.io/) (optional, for Nvim panel)
- [docker](https://www.docker.com/) (optional, for Docker logs panel)
- [Lua](https://www.lua.org/) (for custom panels in `config.lua`)

### Build

Clone the repository and build:

```sh
git clone <your-repo-url>
cd phantom
go build -o phantom ./cmd/phantom
```

### Run

Make sure you have a `config.lua` in the project root (see the provided example).

```sh
./phantom
```

## Configuration

Edit `config.lua` to customize:

- Panel layout (dashboard, http, system, custom panels)
- HTTP request templates and environments
- Custom shell commands (future feature)
- Custom panels (Lua functions)

See comments in `config.lua` for details and examples.

## Key Bindings

- `Tab` / `Shift+Tab`: Switch panels
- `q` or `Ctrl+C`: Quit
- **HTTP Panel:**
  - `Ctrl+S`: Send request
  - `Ctrl+L`: Switch pane
  - `Tab`/`Shift+Tab`: Move between input fields
  - `H`/`L` or `Left`/`Right`: Switch response view

## Logging

Logs are written to `debug.log` in the project root.
