# Phantom

Phantom is a keyboard-first terminal dashboard for everyday developer operations: system health, logs, processes, ports, HTTP workflows, and tool launchers.

## What is implemented

- `Dashboard` tab
  - CPU, memory, disk usage
  - top memory processes
- `Logs` tab
  - multi-source merged logs (`file`, `journald_unit`, `command`)
  - source badges and per-source toggles
  - follow mode, filter, error-only mode
- `Processes` tab
  - live process table (PID, CPU, RSS, name)
  - filtering/navigation
  - action modal: signals, env inspect, open fds, PID yank
- `Ports` tab
  - listening/all inet socket view
  - PID and owning process lookup
  - action modal: kill owner, jump to process tab, yank host:port
- `HTTP` tab
  - collections from `config.lua`
  - environment variable substitution (`{{var}}`)
  - native Go HTTP transport (no `curl` dependency)
  - response views: pretty/raw/headers
- Launcher tabs
  - `Git` (`lazygit`)
  - `Docker` (`lazydocker`)
  - `Kind` cluster operations
  - `Nvim` (`nvim`)
- Global UX
  - tab switching (`Tab`/`Shift+Tab`)
  - direct tab jump (`1`-`9`)
  - command palette (`:`)
  - shell escape in palette (`:!cmd`, `:!!`)
  - config reload from palette
  - status toasts for cross-tab actions/yank

## Project layout

```text
phantom/
├── cmd/phantom/main.go
├── internal/
│   ├── app/
│   ├── config/
│   ├── ui/
│   │   ├── model.go
│   │   ├── components/
│   │   └── tabs/
│   │       ├── dashboard/
│   │       ├── logs/
│   │       ├── processes/
│   │       ├── ports/
│   │       ├── http/
│   │       ├── git/
│   │       ├── docker/
│   │       ├── kind/
│   │       └── nvim/
│   └── utils/
├── config.lua
└── debug.log
```

## Requirements

- Go 1.21+
- Optional binaries:
  - `lazygit`
  - `lazydocker`
  - `kind`
  - `kubectl` (for parts of Kind tab)
  - `nvim`

Missing optional binaries do not crash Phantom. Their tabs remain visible and show install guidance.

## Build and run

```bash
go build -o phantom ./cmd/phantom
./phantom
```

## Test

```bash
go test ./...
```

## Configuration

Phantom searches config in this order:

1. `./config.lua`
2. `~/.config/phantom/config.lua`

If no config is found, defaults are used.

### Minimal config example

```lua
Config = {
  logs = {
    file = "debug.log",
  },

  http = {
    environment = {
      base_url = "https://jsonplaceholder.typicode.com",
      token = "Bearer replace-me",
    },

    templates = {
      {
        name = "List posts",
        method = "GET",
        url = "{{base_url}}/posts",
        headers = "",
        body = "",
      },
      {
        name = "Create post",
        method = "POST",
        url = "{{base_url}}/posts",
        headers = "Content-Type: application/json",
        body = [[
{"title":"hello","body":"from phantom","userId":1}
]],
      },
    },
  },
}
```

## Key bindings

### Global

- `q` or `Ctrl+C`: quit
- `Tab` / `Shift+Tab`: next/previous tab
- `1`-`9`: jump to tab index
- `:`: command palette

### Logs

- `r`: refresh
- `f`: toggle follow
- `l`: toggle error-only
- `S`: toggle source selector
- `[` / `]`: cycle and toggle source on/off (when selector open)
- `/`: filter
- `j`/`k`: move
- `g`/`G`: top/bottom
- `y`: yank selected log line

### Processes

- `r`: refresh
- `/`: filter by process name
- `Enter`: open action modal
- `e`: env view
- `f`: file-descriptor view
- `k`: SIGTERM (inside modal)
- `K`: SIGKILL confirm (inside modal)
- `s`: custom signal picker (inside modal)
- `c`/`y`: copy PID
- `j`/`k`: move
- `g`/`G`: top/bottom

### Ports

- `r`: refresh
- `a`: toggle listening-only vs all sockets
- `Enter`: open action modal
- `k`: SIGTERM owner (inside modal)
- `K`: SIGKILL owner confirm (inside modal)
- `p`: jump to owning process in Processes tab
- `j`/`k`: move
- `g`/`G`: top/bottom
- `c`/`y`: copy host:port

### HTTP

- `Ctrl+S`: send request
- `Ctrl+L`: cycle focus pane
- `Tab` / `Shift+Tab`: cycle request fields
- `H` / `L`: cycle response view
- `y`: yank response body (response pane)

### Palette

- `:!<cmd>` then `Enter`: run shell command and show output overlay
- `:!!`: re-run last shell command
- `j`/`k`: scroll shell output
- `y`: yank shell output
- `Esc`: close overlay/palette

### Kind

- `n`: create cluster
- `d`: delete selected cluster
- `x`: delete all clusters
- `v`: describe
- `l`: list nodes
- `s`: switch kube context
- `i`: load docker image into cluster
- `k`: export kubeconfig
- `e`: export logs

## Notes

- Logs multi-source polling is snapshot-based; it is optimized for practical debugging workflows rather than perfect stream replay semantics.
- HTTP headers support one header per line (`Key: Value`), with backward-compatible parsing for semicolon-separated entries.
- Universal yank fallback order: OSC 52 -> `pbcopy`/`xclip`/`xsel` -> `/tmp/phantom_yank`.
