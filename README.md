# mct (claude-mercato)

A package manager for Claude agent and skill definitions, distributed through Git repositories.

## Why

Claude agent and skill definitions are just markdown files with YAML frontmatter. Teams copy them between repos, lose track of versions, and have no way to discover what's available. `mct` treats this as a package management problem: markets are Git repos, definitions are packages, and the tool handles installation, versioning, updates, and search.

No central registry. No server. Just Git.

## What it does

- **Markets** -- Register Git repositories as sources of agent/skill definitions
- **Install/Remove** -- Add definitions to your local `.claude/` directory with dependency resolution (skills can require other skills)
- **Sync** -- Fetch upstream changes and update installed definitions while detecting local drift
- **Search** -- Full-text BM25 search with fuzzy matching across all registered markets
- **Pin** -- Lock definitions to a specific commit SHA
- **Drift detection** -- Knows when you've modified an installed file locally, and handles conflicts on update
- **TUI** -- Interactive terminal UI for browsing markets and managing installations

## How it works

1. Register a market: `mct market add mymarket git@github.com:org/agents-repo.git`
2. `mct` clones it to `~/.cache/mct/mymarket/`
3. Install a definition: `mct add mymarket/profile/agents/foo`
4. `mct` reads the file from the git tree, injects tracking fields (`mct_ref`, `mct_version`, `mct_market`, `mct_installed_at`) into the frontmatter, writes it to `.claude/`, and records an MD5 checksum
5. If the definition declares `requires_skills`, those are auto-installed as managed dependencies
6. Later, `mct sync` fetches the market, diffs against your last sync point, and updates installed files -- but only after checking whether you've modified them locally

Config lives in `~/.config/mct/config.yml`. State (sync points, checksums) lives in `~/.cache/mct/`.

## Install

```bash
go install github.com/JLugagne/claude-mercato/cmd/mct@latest
```

## Usage

```bash
# Market management
mct market add mymarket git@github.com:org/agents-repo.git
mct market list
mct market info mymarket

# Install / remove
mct add mymarket/profile/agents/foo
mct remove --ref mymarket/profile/agents/foo

# Sync
mct refresh          # Fetch updates from all markets
mct update           # Apply updates to installed entries
mct sync             # refresh + update in one step
mct check            # Show status of all installed entries

# Search
mct search "cli automation"

# Config
mct config set ssh_enabled true
mct config get

# Import/Export
mct export settings.json     # Export all markets + entries to file
mct import settings.json     # Import from file (skips existing URLs)
mct export                   # Export to stdout
mct import settings.json --dry-run  # Preview without changes

# Other
mct pin --ref mymarket/profile/agents/foo --sha abc1234
mct diff --ref mymarket/profile/agents/foo
mct conflicts
mct list
mct index --bench
mct tui
```

## JSON output

Most commands support `--json` for machine-readable output, making `mct` easy to use in scripts and pipelines:

```bash
mct market list --json
mct check --json
mct search "automation" --json
mct list --json
mct export                    # Always JSON (it's the export format)
mct config get --json
mct sync --json
```

## SSH and private repositories

`mct` supports SSH for Git operations, which makes it straightforward to use private repositories as markets. If your team maintains internal agent/skill definitions in a private repo, SSH is the easiest way to manage access -- anyone with SSH keys configured for the Git host can register and sync the market without extra credential setup.

### Setup

```bash
# Enable SSH globally
mct config set ssh_enabled true

# Or via environment variable
export MCT_SSH_ENABLED=true

# Register a private market using SSH URL
mct market add internal git@github.com:my-org/private-agents.git
```

### How it works

- `mct` uses the system SSH agent (`SSH_AUTH_SOCK`) for authentication
- Falls back to key files in `~/.ssh/` (supports ed25519, ecdsa, rsa)
- Reads `~/.ssh/config` for per-host `IdentityFile` settings
- Validates host keys against `~/.ssh/known_hosts`

### When to use SSH

- **Private markets** -- teams sharing internal agent/skill definitions in private repos
- **GitHub/GitLab with SSH keys** -- if you already authenticate via SSH, no extra setup needed
- **Air-gapped or VPN-only hosts** -- SSH works where HTTPS may require proxy configuration

HTTPS is used by default when SSH is not enabled.

## Import / Export

Share your `mct` setup across machines or with teammates:

```bash
# Export everything: markets, installed entries, and config
mct export setup.json

# Import on another machine (markets with the same URL are skipped)
mct import setup.json

# Preview what would happen
mct import setup.json --dry-run
```

The export format is a portable JSON file containing:
- All registered markets (name, URL, branch, flags)
- All installed profiles, grouped by profile path

Duplicate detection is by URL, not by name -- if you renamed a market locally, `mct import` will still recognize it as the same repository and skip re-registering.

## Creating a market

See [MARKET.md](MARKET.md) for the market repository structure and how to create your own.

## License

See [LICENSE](LICENSE).
