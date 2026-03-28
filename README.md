# mct (claude-mercato)

A package manager for Claude agent and skill definitions, distributed through Git repositories.

## Why

Claude agent and skill definitions are just markdown files with YAML frontmatter. Teams copy them between repos, lose track of versions, and have no way to discover what's available. `mct` treats this as a package management problem: markets are Git repos, definitions are packages, and the tool handles installation, versioning, updates, and search.

No central registry. No server. Just Git.

## What it does

- **Markets** -- Register Git repositories as sources of agent/skill definitions
- **Install/Remove** -- Add definitions to your local `.claude/` directory with dependency resolution (skills can require other skills)
- **Sync** -- Fetch upstream changes and update installed definitions while detecting local drift
- **Search** -- Full-text BM25 search across all registered markets
- **Pin** -- Lock definitions to a specific commit SHA
- **Drift detection** -- Knows when you've modified an installed file locally, and handles conflicts on update
- **TUI** -- Interactive terminal UI for browsing markets and managing installations

## How it works

```
Markets (Git repos)          mct               Local .claude/
┌──────────────────┐    ┌──────────────┐    ┌──────────────────┐
│ agents/foo.md    │───>│  clone/fetch  │───>│ agents/foo.md    │
│ skills/bar.md    │    │  diff/merge   │    │ skills/bar.md    │
│ skills/baz.md    │    │  checksum     │    │   (with mct_*    │
└──────────────────┘    │  frontmatter  │    │    fields)       │
                        └──────────────┘    └──────────────────┘
```

1. You register a market: `mct market add mymarket https://github.com/org/agents-repo`
2. `mct` clones it to `~/.cache/mct/mymarket/`
3. You install a definition: `mct add mymarket/profile/agents/foo`
4. `mct` reads the file from the git tree, injects tracking fields (`mct_ref`, `mct_version`, `mct_market`, `mct_installed_at`) into the frontmatter, writes it to your local `.claude/` directory, and records an MD5 checksum
5. If the definition declares `requires_skills`, those are auto-installed as managed dependencies
6. Later, `mct sync` fetches the market, diffs against your last sync point, and updates installed files -- but only after checking whether you've modified them locally

Config lives in `~/.config/mct/config.yml`. State (sync points, checksums) lives in `~/.cache/mct/`.

## Architecture

Clean architecture with no database and no HTTP -- everything is local Git + filesystem:

```
cmd/mct/                          Entry point
internal/mercato/
  domain/                         Pure types, errors, config, frontmatter parsing
    service/                      Service interfaces (commands + queries)
    repositories/                 Repository interfaces (git, filesystem, config, state)
  app/                            Business logic (market, sync, entry, search, conflict)
  outbound/                       Adapters (go-git, filesystem, config/state stores)
  inbound/
    commands/                     Cobra CLI
    queries/tui/                  Bubble Tea TUI
internal/pkg/bm25/                BM25 full-text search engine
```

All Git operations use [go-git](https://github.com/go-git/go-git) -- no `git` binary required.

## Install

```bash
go install github.com/JLugagne/claude-mercato/cmd/mct@latest
```

## Usage

```bash
# Market management
mct market add mymarket https://github.com/org/agents-repo
mct market list
mct market info mymarket

# Install / remove
mct add mymarket/profile/agents/foo
mct remove mymarket/profile/agents/foo

# Sync
mct refresh          # Fetch updates from all markets
mct update           # Apply updates to installed entries
mct sync             # refresh + update in one step
mct check            # Show status of all installed entries

# Search
mct search "cli automation"

# Other
mct pin mymarket/profile/agents/foo    # Lock to current version
mct diff mymarket/profile/agents/foo   # Compare local vs market
mct conflicts                          # Detect reference collisions
mct list                               # List all entries
mct tui                                # Interactive UI
```

## Dependencies

- [go-git](https://github.com/go-git/go-git) -- Git operations
- [cobra](https://github.com/spf13/cobra) -- CLI
- [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss) -- TUI
- [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) -- Config and frontmatter

## License

See [LICENSE](LICENSE).
