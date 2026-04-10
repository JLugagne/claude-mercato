# mct — A package manager for Claude Code agents and skills

Think **npm**, but for your `.claude/` directory.

Install, share, version, and sync Claude Code agents and skills across your team, your machines, and your CI — from any Git repository.

---

## The problem

Claude Code agents and skills are becoming a critical part of how teams build software. They define how your AI reviews code, writes tests, refactors, enforces conventions.

But right now, they are:

- scattered across personal `.claude/` directories
- copy-pasted between projects and teammates
- drifting silently when someone edits a file locally
- impossible to discover or reuse across teams
- never in sync between your laptop and CI

Agents and skills are code-adjacent artifacts. They deserve the same tooling as code: versioning, dependencies, distribution, reproducibility.

That's what `mct` does.

## The solution

`mct` treats Git repositories as **markets** — sources of agent and skill definitions — and installs them into your local `.claude/` directory like packages.

- **Markets** are Git repos you register as sources
- **Entries** (agents, skills) are installed from markets into `.claude/`
- **Dependencies** between skills are resolved automatically
- **Drift detection** knows when you've modified an installed file locally
- **Sync** pulls upstream updates and flags conflicts
- **Search** works across all your markets, fully offline
- **Save / Restore** makes your whole setup portable in one file

No central registry. No server. Just Git.

## Quick example

Register a market and install an agent:

```bash
mct market add team git@github.com:my-org/claude-agents.git
mct add team/profile/agents/reviewer
```

Save your setup so the rest of the team (and your CI) can reproduce it:

```bash
mct save
git add .mct.json
git commit -m "add mct setup"
git push
```

Restore on another machine:

```bash
git pull
mct restore
```

Use in CI:

```yaml
- name: Restore Claude agents and skills
  run: mct restore

- name: Run Claude with restored skills
  run: claude -p "Review this PR" --skills ./.claude/skills
```

Your agents and skills are consistent everywhere — laptop, teammate's laptop, CI runner.

## Features

Real features, all implemented today:

- **Dependency resolution** — skills can declare `requires_skills`, and `mct` installs the full graph
- **Drift detection** — MD5 checksums detect local edits to installed files, so updates never silently overwrite your work
- **Conflict handling** — when a local edit meets an upstream change, `mct` tells you and lets you resolve it
- **Offline BM25 search** with fuzzy matching, across all registered markets
- **Interactive TUI** for browsing markets and managing installations
- **Prune** to handle entries deleted upstream (keep or remove)
- **SSH support** for private repositories (system SSH agent, `~/.ssh/config`, `known_hosts`)
- **JSON output** (`--json`) on most commands for scripting and CI
- **Save / Restore** to a portable `.mct.json` file — share setups across machines and teammates
- **Tracking metadata** injected into frontmatter (`mct_ref`, `mct_version`, `mct_market`, `mct_installed_at`) so installed files are self-describing

## Install

```bash
go install github.com/JLugagne/claude-mercato/cmd/mct@latest
```

## Core commands

```bash
# Markets (sources)
mct market add <name> <git-url>   # register a Git repo as a market
mct market list                   # list registered markets
mct market info <name>            # inspect a market
mct market remove <name>

# Install / remove entries
mct add <market>/<path>           # install an agent or skill (with deps)
mct add <market>/<path> --dry-run
mct remove --ref <market>/<path>

# Sync
mct refresh                       # fetch updates from all markets
mct update                        # apply pending changes locally
mct sync                          # refresh + update
mct check                         # show status of installed entries
mct prune                         # handle entries deleted upstream

# Search
mct search "security review"
mct search "cli" --type skill --installed

# Portable setup
mct save                          # write .mct.json in current dir
mct restore                       # reinstall everything from .mct.json
mct export setup.json             # export full setup (markets + entries)
mct import setup.json

# TUI
mct tui
```

See `mct --help` for the full command list, including `lint`, `diff`, `conflicts`, `index`, and more.

## Why not just use Git?

Git gives you versioned files. It doesn't give you:

- **Dependency resolution** — when installing a skill, its required skills come along automatically
- **Drift detection** — Git can't tell you that you edited `code-reviewer.md` locally after installing it from a market, and that the upstream has also changed
- **Multi-repo composition** — using agents from three different repos in one project, with consistent updates
- **Offline search across sources** — BM25 + fuzzy, no network calls, no data leaving your machine
- **Reproducible CI** — a single `.mct.json` that pins your markets and installed entries for the whole team

`mct` is built on top of Git. It just makes Git usable for a use case Git wasn't designed for.

## Use cases

**Team workflows** — share reviewers, conventions, and skills across a team via a single private Git repo.

**CI automation** — pin your Claude setup in `.mct.json`, run `mct restore` in CI, get reproducible AI-assisted pipelines.

**Multi-machine setups** — keep your personal `.claude/` in sync across laptop, desktop, and remote dev boxes.

**Enterprise environments** — SSH support and fully local search mean internal agents and skills can be shared without any data leaving the company network.

## Creating a market

A market is just a Git repository with a specific layout. See [MARKET.md](./MARKET.md) for the full spec and examples.

## Roadmap

- Public and private registries
- Semantic versioning and compatibility checks
- Signed entries
- Richer conflict resolution UI in the TUI
- Deeper integration with Claude Code

## Scope

`mct` is currently focused on the **Claude Code ecosystem** — agents and skills as markdown files with YAML frontmatter in `.claude/`. Support for other AI runtimes may come later if there's demand, but the goal today is to be the best possible package manager for one ecosystem, not a mediocre one for all of them.

## Feedback

Open questions I'd love input on:

- How should conflicts between multiple markets exposing the same entry be handled?
- What belongs in a future `mct` standard vs. what should stay market-specific?
- What's missing for your team to adopt it?

Issues and discussions welcome.

## License

MIT
