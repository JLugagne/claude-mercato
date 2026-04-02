# inbound/commands — CLI Commands

**Location**: `internal/mercato/inbound/commands/`

## Purpose

All Cobra CLI command definitions. Each file registers one or more subcommands on the root command. Commands delegate to `Services` (the app layer) and format output.

## Root Command

`NewRootCmd()` builds the root `mct` command and attaches all subcommands.

### Global Flags (`GlobalOpts`)

| Flag | Description |
|------|-------------|
| `--config` | Config file path override |
| `--cache` | Cache directory override |
| `--offline` | Disable network access |
| `--verbose` | Verbose output |
| `--quiet` | Suppress output |
| `--no-color` | Disable color |
| `--ci` | CI mode (no interactive prompts) |

### Services Struct

Dependency injection container passed to all commands:

```go
type Services struct {
    MarketCommands
    EntryCommands
    SyncCommands
    SearchQueries
    ReadmeQueries
    ConfigCommands
}
```

## Command Files

| File | Commands | Aliases | Description |
|------|----------|---------|-------------|
| `market_cmd.go` | `market add`, `market remove`, `market list`, `market info`, `market set` | `markets` → `market list` | Market lifecycle |
| `add_cmd.go` | `add <ref>` | `install` | Install entry + deps |
| `remove_cmd.go` | `remove <ref>` | — | Uninstall entry |
| `list_cmd.go` | `list` | — | List installed entries |
| `search_cmd.go` | `search <query>` | — | Full-text search |
| `sync_cmd.go` | `sync` | — | Refresh + update |
| `refresh_cmd.go` | `refresh` | — | Fetch updates |
| `update_cmd.go` | `update` | — | Apply changes |
| `check_cmd.go` | `check` | `status` | Show entry states |
| `prune_cmd.go` | `prune` | — | Clean deleted entries |
| `conflicts_cmd.go` | `conflicts` | — | Show ref collisions |
| `lint_cmd.go` | `lint <market>` | — | Validate market |
| `readme_cmd.go` | `readme` | — | Show/list READMEs |
| `config_cmd.go` | `config get`, `config set` | — | Config management |
| `export_cmd.go` | `export` | `save` | Save .mct.json |
| `import_cmd.go` | `import` | `restore` | Restore from .mct.json |
| `hook_cmd.go` | `hook install`, `hook uninstall` | — | Git hook management |
| `sync_state_cmd.go` | `sync-state` | — | Per-market sync state |
| `index_cmd.go` | `index dump`, `index bench` | — | Index debugging |

## Utility Files

| File | Description |
|------|-------------|
| `json.go` | `printJSON()` helper for JSON output formatting |
| `stubs.go` | Helper command constructors |
