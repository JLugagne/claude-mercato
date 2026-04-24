---
name: doc-project
description: >
  ALWAYS LOAD THIS SKILL when working on agents-mercato. Comprehensive project documentation:
  purpose, how it works end-to-end, architecture, all packages, data models, and key workflows.
  Required context for any code change, review, or question about this codebase.
---

# agents-mercato (mct) — Project Documentation

## What is agents-mercato?

agents-mercato (`mct`) is a **decentralized package manager for Claude Code agents and skills**. It solves the problem of distributing, versioning, and synchronizing Claude agent/skill definitions across teams and projects — without requiring any central registry or infrastructure.

### The Problem

Claude Code agents and skills are markdown files (`.md` / `SKILL.md`) that live in a project's `.claude/` directory. Teams need to:
- Share curated sets of agents and skills across multiple projects
- Keep those definitions in sync when upstream authors publish updates
- Detect when someone locally modifies an installed skill (drift detection)
- Manage dependencies between skills (skill A requires skill B)
- Avoid conflicts when multiple sources provide overlapping definitions

### The Solution

mct uses **Git repositories as distribution channels** (called "markets"). A market is just a Git repo with a conventional directory structure containing agent and skill files. mct clones markets locally, indexes their contents, and lets users install entries into their projects. It then tracks versions, detects drift, resolves dependencies, and synchronizes updates — all through Git.

### How It Works End-to-End

1. **Register a market**: `mct market add <git-url>` — mct shallow-clones the repo into `~/.cache/mct/<name>/`, auto-detects its format (hierarchical or flat), and saves it to `~/.config/mct/config.yml`

2. **Browse & search**: `mct search <query>` or `mct tui` — mct builds a BM25 full-text index from all market entries, supports fuzzy matching, filtering by type/market/category, and an interactive TUI with three-panel browsing

3. **Install an entry**: `mct add <market>@<path>` — mct reads the file from the cached clone, validates its YAML frontmatter, recursively resolves skill dependencies (with cycle detection), injects tracking metadata (`mct_ref`, `mct_version`, `mct_checksum`, etc.), writes files to `.claude/`, and records the installation in `~/.cache/mct/installed.json`

4. **Check status**: `mct check` — for every installed entry, mct computes its state by comparing the local file hash (xxhash) against both the recorded version and the latest upstream version. States: clean, update_available, drift, update_and_drift, deleted, orphaned

5. **Sync updates**: `mct sync` (or `mct refresh` + `mct update` separately) — fetches from all markets, computes diffs since last sync, and applies changes. Handles drift conflicts interactively (keep local / accept upstream / delete) or via policy flags (`--all-keep`, `--all-merge`)

6. **Export/restore**: `mct save` writes a `.mct.json` manifest; `mct restore` re-installs from it. Combined with optional Git hooks (`mct hook install`), this enables fully automatic save-on-push and restore-on-pull workflows

### Market Format

**Hierarchical** (full market):
```
market-repo/
  profile-a/category-x/
    README.md              ← profile metadata (tags, description)
    agents/agent1.md       ← agent definition
    skills/skill1/SKILL.md ← skill definition (+ supporting files)
  profile-b/category-y/
    README.md
    agents/agent2.md
```

**Flat** (skills-only repo):
```
repo/
  skills/
    skill1/SKILL.md
    skill2/SKILL.md
```

Format is auto-detected on `market add`. The first two path segments form a "profile" that groups related entries.

### Entry Type Inference

Determined by path convention, not declaration:
- `.md` file under `agents/` → Agent
- `SKILL.md` under `skills/<name>/` → Skill
- Everything else → Ignored by mct

### Dependency Resolution

Skills can declare dependencies in their frontmatter:
```yaml
requires_skills:
  - file: skills/foo/SKILL.md              # same-market dep
  - file: skills/bar/SKILL.md
    market: https://github.com/org/other.git  # cross-market dep
    pin: abc123                               # optional version pin
```

mct resolves dependencies recursively with cycle detection. Cross-market deps auto-register the foreign market if not already known (with user confirmation in interactive mode).

### Drift Detection

After installation, mct tracks each file's content hash (xxhash64). On `mct check`, it compares:
- Local file content vs. original content at the recorded git version
- If they differ → the user has locally modified the file → marked as "drift"

This lets users customize installed skills while still being warned about upstream updates that may conflict.

### Conflict Detection

Two types:
1. **Ref collision** — Same filename exists in multiple markets (ambiguous install)
2. **Version mismatch** — Same skill required at different versions by different agents

### Project Isolation

Files are **copied, not symlinked**. Each project gets its own independent copy in `.claude/`. The install database (`installed.json`) tracks which projects have which entries, enabling multi-project cleanup via `CleanStaleLocations()`.

## Tech Stack

- **Go 1.26** with **Cobra** (CLI), **Charmbracelet** (TUI), **go-git** (Git ops)
- **BM25 + Snowball** for full-text search, **xxhash** for drift detection, **YAML** for config

## Architecture

Hexagonal (Ports & Adapters):

```
cmd/mct/main.go          → entry point, wires DI
inbound/commands/         → Cobra CLI handlers
inbound/queries/tui/      → Bubble Tea interactive UI
app/                      → business logic (implements all service interfaces)
domain/                   → types, errors, frontmatter parsing, state
domain/service/           → port interfaces (queries + commands)
domain/repositories/      → adapter interfaces (git, fs, config, state, installdb)
outbound/gitadapter/      → go-git implementation
outbound/fsadapter/       → OS filesystem implementation
outbound/cfgadapter/      → YAML config, JSON state, install DB with file locking
```

## Package Index

| Package | Role | Details |
|---------|------|---------|
| [cmd/mct](./pkg-cmd-mct.md) | Entry point | Bootstrap, DI wiring, root command |
| [domain](./pkg-domain.md) | Core types & logic | Config, types, errors, frontmatter, state, install DB, diffs |
| [domain/service](./pkg-domain-service.md) | Port interfaces | Market, Entry, Sync, Search, Readme, Config contracts |
| [domain/repositories](./pkg-domain-repositories.md) | Adapter interfaces | Filesystem, GitRepo, ConfigStore, StateStore, InstallDB |
| [app](./pkg-app.md) | Application layer | Market mgmt, entry install/remove, sync, search, lint, conflicts |
| [inbound/commands](./pkg-inbound-commands.md) | CLI commands | All Cobra subcommands, JSON output, global flags |
| [inbound/queries/tui](./pkg-inbound-tui.md) | Terminal UI | Bubble Tea app with panels, search, command mode |
| [outbound/gitadapter](./pkg-outbound-gitadapter.md) | Git adapter | go-git clone/fetch/diff/read, SSH auth |
| [outbound/fsadapter](./pkg-outbound-fsadapter.md) | Filesystem adapter | Read/write/symlink, MD5 checksums |
| [outbound/cfgadapter](./pkg-outbound-cfgadapter.md) | Config/state adapter | YAML config, JSON state/installdb, file locking |

## Key Concepts

- **Market** — A Git repo containing agent/skill definitions in a hierarchical or flat layout
- **Entry** — An agent (.md file under `agents/`) or skill (`SKILL.md` under `skills/<name>/`)
- **Profile** — First two path segments group entries (e.g., `dev/go`)
- **MctRef** — Canonical reference in `market@path` format
- **Drift** — Local modifications detected via xxhash comparison against upstream
- **Sync** — Fetch upstream changes, compute state (clean/update/drift/deleted), apply updates
- **Dependency resolution** — Skills can require other skills, even cross-market, with cycle detection
- **Conflict detection** — Filename collisions across markets, version mismatches on shared deps
- **Git hooks** — Optional post-merge (`mct restore`) and pre-push (`mct save`) automation

## Data Flow

```
User runs `mct add market@dev/go/skills/foo`
  → CLI parses ref → App.Add()
    → GitRepo.ReadFileAtRef() reads from cached clone
    → Frontmatter.Parse() validates YAML header
    → Dependency resolution (recursive, cycle-safe)
    → InjectMctFields() adds tracking metadata
    → Filesystem.WriteFile() writes to .claude/
    → InstallDB.Save() records in installed.json
```

## Configuration

- **Config**: `~/.config/mct/config.yml` — markets, policies (conflict, drift), local path, SSH
- **Cache**: `~/.cache/mct/` — shallow clones, `installed.json`, `sync-state.json`
- **Env vars**: `MCT_SSH_ENABLED`, `MCT_LOCAL_PATH`, `MCT_CONFLICT_POLICY`, `MCT_DRIFT_POLICY`
