# mct (claude-mercato)

A package manager for Claude agent and skill definitions, distributed through Git repositories.

## Why

Claude agent and skill definitions are just markdown files with YAML frontmatter. Teams copy them between repos, lose track of versions, and have no way to discover what's available. `mct` treats this as a package management problem: markets are Git repos, definitions are packages, and the tool handles installation, versioning, updates, and search.

No central registry. No server. Just Git.

## What it does

- **Markets** -- Register Git repositories as sources of agent/skill definitions
- **Skills repos** -- First-class support for skills-only repositories (e.g. Anthropic's `skills/` directory format)
- **Install/Remove** -- Add definitions to your local `.claude/` directory with dependency resolution (skills can require other skills, even from other markets)
- **Sync** -- Fetch upstream changes and update installed definitions while detecting local drift
- **Search** -- Full-text BM25 search with fuzzy matching across all registered markets
- **Drift detection** -- Knows when you've modified an installed file locally, and handles conflicts on update
- **Prune** -- Handle entries deleted upstream with keep/remove decisions
- **TUI** -- Interactive terminal UI for browsing markets and managing installations

## Supported formats

`mct` supports two repository formats:

### mct market format

A hierarchical structure with profiles containing agents and skills:

```
market-repo/
  dev/
    go/
      README.md
      agents/
        go-developer.md
      skills/
        go-test/
          SKILL.md
    python/
      README.md
      agents/
        python-developer.md
```

See [MARKET.md](MARKET.md) for the full specification.

### Skills-only repos

A flat structure containing only skill directories. This is the format used by Anthropic's `skills/` repos and other community skill collections:

```
skills-repo/
  skills/
    my-skill/
      SKILL.md
    another-skill/
      SKILL.md
```

`mct` auto-detects skills-only repos when adding them. You can also point to a subdirectory using GitHub `/tree/` URLs:

```bash
# GitHub /tree/ URL -- branch and subpath extracted automatically
mct market add https://github.com/org/repo/tree/main/src/skills

# Or specify the skills path as a second argument
mct market add https://github.com/org/skills-repo.git src/skills
```

## How it works

1. Register a market: `mct market add mymarket git@github.com:org/agents-repo.git`
2. `mct` clones it (shallow) to `~/.cache/mct/mymarket/`
3. Install a definition: `mct add mymarket@dev/go/agents/go-developer.md`
4. `mct` reads the file from the git tree, injects tracking fields (`mct_ref`, `mct_version`, `mct_market`, `mct_installed_at`) into the frontmatter, and symlinks it into `.claude/`
5. If the definition declares `requires_skills`, those are auto-installed as managed dependencies (including cross-market dependencies)
6. Later, `mct sync` fetches the market, diffs against your last sync point, and updates installed files -- but only after checking whether you've modified them locally

Config lives in `~/.config/mct/config.yml`. State (sync points, checksums) lives in `~/.cache/mct/`.

## Install

```bash
go install github.com/JLugagne/claude-mercato/cmd/mct@latest
```

## Usage

Refs use the format `market@path` (e.g. `mymarket@dev/go/agents/go-developer.md`).

```bash
# Project setup
mct init                     # Initialize mct in current project

# Market management
mct market add mymarket git@github.com:org/agents-repo.git
mct market add https://github.com/org/skills-repo/tree/main/skills
mct market add https://github.com/org/skills-repo.git src/skills
mct market list
mct market info mymarket
mct market remove mymarket
mct market rename mymarket newname
mct market set mymarket <property> <value>
mct markets                  # alias for market list
mct readme mymarket          # Show market README
mct readme mymarket skills/README.md
mct readme mymarket --list   # List all READMEs in market

# Install / remove
mct add mymarket@dev/go/agents/foo.md
mct add mymarket@dev/go/agents/foo.md --dry-run
mct add mymarket@dev/go/agents/foo.md --no-deps
mct add mymarket@dev/go/agents/foo.md --accept-breaking
mct install mymarket@dev/go/agents/foo.md  # alias for add
mct remove --ref mymarket@dev/go/agents/foo.md
mct remove --all          # Remove all installed entries (prompts for confirmation)
mct remove --all --yes    # Remove all without prompt

# Sync
mct refresh          # Fetch updates from all markets
mct update           # Apply pending changes to local files
mct sync             # refresh + update in one step
mct check            # Show status of all installed entries
mct status           # alias for check
mct prune            # Process deleted entries
mct prune --ref mymarket@dev/go/agents/foo.md
mct prune --all-keep
mct prune --all-remove

# Search
mct search "cli automation"
mct search "cli automation" --type skill
mct search "cli automation" --market mymarket
mct search "cli automation" --installed
mct search "cli automation" --not-installed
mct search "cli automation" --limit 20

# Config
mct config set ssh_enabled true
mct config get

# Import/Export
mct export settings.json     # Export all markets + entries to file
mct import settings.json     # Import from file (skips existing URLs)
mct export                   # Export to stdout
mct import settings.json --dry-run  # Preview without changes
mct save                     # Save current setup to .mct.json
mct restore                  # Restore setup from .mct.json

# Other
mct conflicts
mct list              # List installed profiles and their entry refs
mct sync-state        # Print sync state
mct index --bench
mct index --dump      # Dump index as JSON
mct lint [dir]        # Check a local directory as a market
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

# Shorthand: save/restore to .mct.json in current directory
mct save       # equivalent to: mct export .mct.json
mct restore    # equivalent to: mct import .mct.json
```

The export format is a portable JSON file containing:
- All registered markets (name, URL, branch, flags)
- All installed profiles, grouped by profile path

Duplicate detection is by URL, not by name -- if you renamed a market locally, `mct import` will still recognize it as the same repository and skip re-registering.

## Creating a market

See [MARKET.md](MARKET.md) for the market repository structure and how to create your own.

## Privacy

`mct` does not collect any telemetry, analytics, or usage data. It never phones home. All operations happen locally on your machine and communicate only with the Git repositories you explicitly register. Your workflow, your data, your business.

## Why mct

Stop copy-pasting markdown files between repos and hoping everyone is on the same version. `mct` gives your team a single command to install, update, and discover Claude agent and skill definitions -- with version tracking, dependency resolution, and drift detection built in. It works with any Git host, public or private, and requires zero infrastructure: no registry, no server, no account. Just `mct market add` and you're up and running.

Teams already commit `.eslintrc`, `.editorconfig`, and `pyproject.toml` so every contributor shares the same linting and formatting rules. Agent and skill definitions are the next layer of that idea -- they encode *how the AI works on your project*: which patterns to follow, which tests to write, which architectural rules to enforce. With `mct save`, you check a `.mct.json` into your repo, and anyone who runs `mct restore` gets the exact same set of agents and skills. Same context, same standards, same results -- no setup wiki, no onboarding checklist.

## License

See [LICENSE](LICENSE).
