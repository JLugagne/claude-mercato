# mct — The Package Manager for Claude Skills & Agents

**One command to install. One command to sync. Zero infrastructure.**

`mct` is a lightweight CLI that turns any Git repository into a distribution channel for Claude agent and skill definitions. Install curated prompts, keep them up to date across every project, and share your team's best practices — all through the Git workflows you already use.

No central registry. No hosted service. No account required. Just Git.

---

## Why mct?

Claude agent and skill definitions are just markdown files — yet teams copy-paste them across repos, lose track of versions, and have no way to discover what's available. Bug fixes never propagate. Onboarding means a wiki page with a dozen manual steps. Everyone ends up running a slightly different version of the same prompt.

`mct` fixes this by treating definitions as packages. Markets are Git repos. Definitions are packages. The CLI handles installation, versioning, updates, dependency resolution, and search — so your team stays in sync without the overhead. It works with any Git host, public or private, and requires zero infrastructure: no registry, no server, no account. Just `mct market add` and you're up and running.

### What you get

- **Install & remove** definitions with a single command, including automatic dependency resolution — even across repositories
- **Sync** upstream changes to every project, with drift detection that respects your local edits
- **Search** across all registered markets with full-text BM25 ranking, Snowball stemming, and fuzzy matching (Levenshtein distance ≤ 2)
- **Git hooks** for automated sync — auto-restore after pull and auto-save before push
- **Prune** definitions deleted upstream, with interactive keep-or-remove decisions
- **Lint** your own market before publishing to catch structural issues early
- **Browse** everything in an interactive terminal UI with keyboard-driven navigation
- **Script** everything with `--json` output on every command for CI/CD pipelines
- **Work offline** with `--offline` mode when you don't need network access

---

## Quick Start

### 1. Install

```bash
go install github.com/JLugagne/claude-mercato/cmd/mct@latest
```

### 2. Initialize your project

```bash
cd your-project
mct init
```

This creates the configuration at `~/.config/mct/config.yml` and sets up the local `.claude/` directory where definitions will live.

### 3. Add a market

Point `mct` at any Git repository containing agent or skill definitions. The market name is auto-derived from the URL (e.g. `https://github.com/acme/agents.git` becomes `acme/agents`). You can use full URLs or bare hostnames — `mct` auto-prepends `https://` when no protocol is specified:

```bash
# Public repo (registered as "acme/agents")
mct market add github.com/acme/agents

# Full URL works too
mct market add https://github.com/acme/agents.git

# Skills-only repo with /tree/ URL (registered as "acme/skills-repo")
mct market add https://github.com/acme/skills-repo/tree/main/skills

# Private repo via SSH (registered as "my-org/private-agents")
mct config set ssh_enabled true
mct market add git@github.com:my-org/private-agents.git
```

`mct` shallow-clones the repository to `~/.cache/mct/<n>/` and indexes all agents and skills it finds.

### 4. Discover and install

Refs use the format `market-name@path`, where the market name is the auto-derived name:

```bash
# Search across all markets
mct search "go testing"

# Install a single agent (dependencies resolved automatically)
mct add acme/agents@dev/go/agents/go-developer.md

# Install an entire profile at once
mct add acme/agents@dev/go

# Or browse interactively
mct tui
```

### 5. Stay in sync

```bash
# Pull upstream changes and update local files in one step
mct sync

# Or do it in two stages: fetch first, then apply
mct refresh
mct update

# Check what's changed
mct status
```

That's it. Your `.claude/` directory now has versioned, trackable definitions that update with a single command.

---

## How Synchronization Works

`mct` keeps your local definitions aligned with upstream markets across all your projects, without ever silently overwriting your work.

### Installing

When you install a definition, `mct` reads the file from the Git tree, copies it into your project's `.claude/` directory, and records everything in a local state database (`~/.cache/mct/installed.json`) — including the market name, profile path, commit SHA, file list, and install location. For directory-based skills, every file in the skill directory is copied, not just `SKILL.md`. If a definition declares skill dependencies (via the `requires_skills` frontmatter field), those are automatically installed too — even if they live in a different market. Cross-market dependencies are resolved by auto-registering the required market with your confirmation.

### Syncing

When you run `mct sync`, the tool fetches each registered market, diffs the latest commit against your last sync point, and applies updates to affected installed files. Before overwriting anything, it computes xxHash checksums to detect whether you've modified a file locally. If you have, `mct` flags the conflict and gives you control: keep your local version (`--all-keep`), overwrite with upstream (`--all-merge`), or handle entries individually. You can also filter updates to only agents (`--agents-only`) or only skills (`--skills-only`).

### Pruning

When a definition is deleted upstream, `mct prune` walks you through the cleanup — keep the local copy or remove it, your call. Batch decisions are available with `--all-keep` and `--all-remove`.

### Project isolation

Files are always **copied, not symlinked**, so every project is fully self-contained and works without access to the cache directory. The same definition can be installed across multiple projects, and each installation is tracked separately with automatic stale-location cleanup.

State lives in `~/.cache/mct/`. Configuration lives in `~/.config/mct/config.yml`.

---

## Share Your Setup with `.mct.json`

Teams already commit `.eslintrc`, `.editorconfig`, and `pyproject.toml` so every contributor shares the same tooling rules. `.mct.json` extends that idea to AI definitions — it encodes which markets your project uses and which agents and skills are installed, so everyone works with the same context, the same standards, and the same results.

```bash
# Save your current market registrations and installed definitions
mct save

# Commit .mct.json alongside your code
git add .mct.json && git commit -m "Add mct configuration"
```

When a teammate clones the repo, they run one command:

```bash
mct restore
```

Every market is registered, every definition is installed, and the project is ready. No setup doc, no onboarding checklist, no "which version of the prompt are you using?" conversations.

The export format is a portable JSON file (version 1) containing all registered markets (name, URL, branch, trusted/read-only flags) and every installed profile grouped by path. Duplicate detection works by normalized URL — protocol prefixes, trailing `.git`, and casing are all stripped — so renamed markets are still recognized and won't be duplicated.

For machine-to-machine transfers or CI pipelines:

```bash
mct export setup.json             # export to a named file
mct export                        # export to stdout
mct import setup.json --dry-run   # preview what would happen
mct import setup.json --yes       # apply without prompts
```

---

## Git Hooks

`mct` can install Git hooks to keep your `.mct.json` in sync automatically, so the team never drifts:

```bash
# Auto-restore after every pull (runs mct restore in post-merge)
mct hook install post-pull

# Auto-save before every push (runs mct save + git add + git commit in pre-push)
mct hook install pre-push
```

With both hooks installed, the workflow is fully hands-free: pulling automatically installs any new definitions your teammates added, and pushing automatically commits your `.mct.json` changes so they propagate to the rest of the team.

Hooks are managed as tagged snippets inside standard Git hook files — they won't clobber your existing hooks. Remove them cleanly with:

```bash
mct hook uninstall post-pull
mct hook uninstall pre-push
```

If the hook file becomes empty after removing the `mct` snippet, it's deleted entirely.

---

## Use Any Git Repository — Public or Private

`mct` works with any Git host: GitHub, GitLab, Bitbucket, Gitea, self-hosted instances, or even bare repos on a shared drive. No vendor lock-in, no platform dependency. Two repository formats are supported, and `mct` auto-detects which one you're using.

### Market format (hierarchical)

A structured repository with profiles containing agents and skills:

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

Profiles are defined by the first two path segments (e.g. `dev/go`). Each profile can have a `README.md` with YAML frontmatter for description and tags, which feeds into search ranking.

### Skills-only format (flat)

A flat structure containing only skill directories — the format used by Anthropic's official skills repository and community skill collections:

```
skills-repo/
  skills/
    my-skill/
      SKILL.md
    another-skill/
      SKILL.md
```

`mct` auto-detects this layout. You can also point to a subdirectory using GitHub `/tree/` URLs, and `mct` will parse the branch and subpath automatically:

```bash
mct market add https://github.com/org/repo/tree/main/src/skills
```

Or specify the skills path as a second argument:

```bash
mct market add https://github.com/org/repo.git src/skills
```

See [MARKET.md](MARKET.md) for the full specification on creating your own market.

---

## Privacy First

`mct` does not collect any telemetry, analytics, or usage data. It never phones home. All operations happen locally on your machine and communicate only with the Git repositories you explicitly register. There is no central server, no third-party service, and no tracking of any kind. Your workflow, your data, your business.

---

## SSH and Private Repositories

`mct` has first-class SSH support, making it straightforward to use private repositories as markets. If your team maintains internal agent and skill definitions in a private repo, SSH is the simplest way to manage access — anyone with SSH keys configured for the Git host can register and sync without extra credential setup.

```bash
# Enable SSH globally
mct config set ssh_enabled true

# Or via environment variable
export MCT_SSH_ENABLED=true

# Register a private market (registered as "my-org/private-agents")
mct market add git@github.com:my-org/private-agents.git
```

Under the hood, `mct` uses your system's SSH agent (`SSH_AUTH_SOCK`) for authentication, falls back to key files in `~/.ssh/` (ed25519, ecdsa, rsa), reads `~/.ssh/config` for per-host `IdentityFile` settings, and validates host keys against `~/.ssh/known_hosts`. HTTPS is used by default when SSH is not enabled.

---

## Configuration

`mct` exposes four configuration keys that control its behavior:

```bash
mct config set ssh_enabled true       # Enable SSH for git operations
mct config set local_path .claude/    # Where definitions are installed locally
mct config set conflict_policy block  # How to handle ref collisions (block/skip)
mct config set drift_policy prompt    # How to handle local modifications (prompt/force/skip)

mct config get                        # Show all current values
mct config get --json                 # Machine-readable output
```

Configuration can also be set via environment variables (e.g. `MCT_SSH_ENABLED=true`), which take precedence over the config file.

---

## Full Command Reference

### Project setup

```bash
mct init                          # Initialize mct in current project
```

### Market management

Market names are auto-derived from URLs (e.g. `github.com/acme/agents` → `acme/agents`). URLs without a protocol get `https://` prepended automatically. Use `mct market list` to see your registered names.

```bash
mct market add <url> [skills-path]  # Register a new market (name auto-derived from URL)
mct market add <url> --branch dev   # Track a specific branch
mct market add <url> --trusted      # Skip breaking change confirmations
mct market add <url> --read-only    # Index only, never install from it
mct market add <url> --no-clone     # Register without cloning
mct market list                     # List configured markets with their derived names
mct market info <n>              # Show market details
mct market remove <n>            # Unregister a market
mct market remove <n> --force    # Skip installed entries check
mct market remove <n> --keep-cache  # Keep local clone directory
mct market set <n> <key> <value> # Update a market property
mct markets                         # Alias for market list
mct readme <n>                   # Show market README
mct readme <n> <path>            # Show a specific README
mct readme <n> --list            # List all READMEs in market
```

### Install / remove

Refs use the format `name@path` (e.g. `acme/agents@dev/go/agents/go-developer.md`):

```bash
mct add <ref>                       # Install an entry
mct add <ref> --dry-run             # Preview install
mct add <ref> --no-deps             # Skip dependency resolution
mct add <ref> --accept-breaking     # Accept breaking changes
mct install <ref>                   # Alias for add
mct remove --ref <ref>              # Remove an installed entry
mct remove --ref <ref> --all-locations  # Remove from all projects
mct remove --all                    # Remove all (prompts for confirmation)
mct remove --all --yes              # Remove all without prompt
```

### Sync

```bash
mct refresh                         # Fetch updates from all markets
mct update                          # Apply pending changes
mct update --agents-only            # Only update agents
mct update --skills-only            # Only update skills
mct update --all-keep               # Keep all local changes
mct update --all-merge              # Overwrite with upstream
mct update --all-delete             # Delete all local changes
mct sync                            # Refresh + update in one step
mct sync --dry-run                  # Preview without applying
mct check                           # Show status of all installed entries
mct status                          # Alias for check
mct prune                           # Process entries deleted upstream
mct prune --ref <ref>               # Process a specific entry
mct prune --all-keep                # Keep all deleted entries locally
mct prune --all-remove              # Remove all deleted entries
```

### Search

```bash
mct search "cli automation"                          # Full-text search
mct search "cli automation" --type skill             # Filter by type
mct search "cli automation" --market acme/agents     # Filter by market
mct search "cli automation" --category dev/go        # Filter by category
mct search "cli automation" --installed              # Only installed
mct search "cli automation" --not-installed          # Only not installed
mct search "cli automation" --include-deleted        # Include deleted entries
mct search "cli automation" --limit 20               # Max results
```

### Import / export

```bash
mct export settings.json            # Export to file
mct export                          # Export to stdout
mct import settings.json            # Import from file
mct import settings.json --dry-run  # Preview without changes
mct import settings.json --yes      # Auto-confirm new markets
mct save                            # Save to .mct.json
mct restore                         # Restore from .mct.json
```

### Git hooks

```bash
mct hook install post-pull          # Auto-restore after git pull
mct hook install pre-push           # Auto-save before git push
mct hook uninstall post-pull        # Remove the post-pull hook
mct hook uninstall pre-push         # Remove the pre-push hook
```

### Other

```bash
mct conflicts                       # Show all conflicts
mct list                            # List installed profiles and refs
mct sync-state                      # Print sync state per market
mct index --bench                   # Measure indexing performance
mct index --dump                    # Dump index as JSON
mct lint [dir]                      # Validate a market structure
mct tui                             # Launch interactive terminal UI
```

### Global flags

Every command accepts these flags:

```bash
--config <path>     # Path to config file (default: ~/.config/mct/config.yml)
--cache <path>      # Cache directory (default: ~/.cache/mct)
--offline           # Disable network operations
--verbose           # Detailed operation log
--quiet             # Suppress all output except errors
--no-color          # Disable ANSI colors
--ci                # Non-interactive mode (for CI/CD pipelines)
--version           # Print version
```

### JSON output

Most commands support `--json` for machine-readable output, making `mct` easy to script:

```bash
mct market list --json
mct check --json
mct search "automation" --json
mct list --json
mct sync --json
mct config get --json
```

---

## Lint Your Market Before Publishing

Validate your market structure before pushing:

```bash
mct lint [dir]
```

This checks for valid frontmatter in all entry files, correct directory structure (`agents/`, `skills/`), missing profile READMEs, broken `requires_skills` references, and profiles with no agents or skills. Returns exit code 1 if errors are found, making it easy to integrate into CI.

---

## Creating a Market

See [MARKET.md](MARKET.md) for the full specification. In short: every agent or skill file uses YAML frontmatter with a `description` field, and can optionally declare dependencies on other skills — including cross-market dependencies via a `market` URL — using the `requires_skills` field. The entry type is inferred from the path: files under `agents/` are agents, `SKILL.md` files under `skills/<n>/` are skills. When installed, `mct` injects tracking fields (`mct_ref`, `mct_version`, `mct_market`, `mct_profile`, `mct_installed_at`, `mct_checksum`) into the frontmatter for version tracking and integrity verification.

## License

MIT — See [LICENSE](LICENSE).
