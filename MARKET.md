# Creating a Market

A market is a Git repository with a specific directory structure. `mct` indexes it to let users discover, install, and update Claude agent and skill definitions.

`mct` supports two repository formats: the **mct market format** (hierarchical, with profiles) and the **skills-only format** (flat, for pure skill collections). When you register a repository, `mct` auto-detects which format it uses.

---

## mct market format

### Repository structure

```
your-market/
  profile-a/
    category-x/
      README.md              # Profile description and tags
      agents/
        my-agent.md          # Agent definition
      skills/
        helper-skill/
          SKILL.md            # Skill definition
          other-file.txt      # Additional files (bundled with the skill)
  profile-b/
    category-y/
      README.md
      agents/
        another-agent.md
      skills/
        another-skill/
          SKILL.md
```

Key rules:

- Agent files must be `.md` and live under an `agents/` directory.
- Skill definitions use a directory structure: `skills/<name>/SKILL.md`. All files in the skill directory are bundled — not just `SKILL.md`.
- The first two path segments form the **profile** (e.g. `profile-a/category-x`).
- A `README.md` at the profile level (`profile/category/README.md`) provides metadata for all entries in that profile.
- Everything else is ignored by `mct`.

### Entry type inference

The entry type (agent or skill) is **inferred from the path**, not declared in frontmatter. Any `.md` file under an `agents/` directory is an agent. A `SKILL.md` file under `skills/<name>/` is a skill. If `mct` cannot determine the type from the path, the file is ignored (and `mct lint` will flag it as an error).

### Entry frontmatter

Every agent or skill file must start with YAML frontmatter delimited by `---`:

```yaml
---
description: Short description of what this does
author: Your Name
---

Your agent/skill prompt content here...
```

#### Required fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Short description, used in search results and the TUI |

#### Optional fields

| Field | Type | Description |
|-------|------|-------------|
| `author` | string | Author name |
| `breaking_change` | bool | Flags a breaking change in this version. Users who haven't passed `--accept-breaking` will be prompted before updating. |
| `deprecated` | bool | Marks entry as deprecated |
| `requires_skills` | list | Skills this entry depends on (see [Skill dependencies](#skill-dependencies) below) |

### Skill dependencies

An agent or skill can declare that it requires other skills. When a user installs this entry, `mct` auto-installs all required skills — with cycle detection to prevent infinite loops.

```yaml
---
description: Go developer agent
requires_skills:
  - file: skills/go-test/SKILL.md
  - file: skills/go-lint/SKILL.md
    pin: abc1234
---
```

Each dependency entry supports these fields:

| Field | Required | Description |
|-------|----------|-------------|
| `file` | yes | Path to the skill file, relative to the market root. Must be a safe relative path — no absolute paths, no `..` traversal, and only alphanumerics, hyphens, underscores, dots, and forward slashes. |
| `pin` | no | Locks the dependency to a specific commit SHA |
| `market` | no | URL of an external market containing the skill (see below) |

The `file` path can point to either a `SKILL.md` file (e.g. `skills/go-test/SKILL.md`) or a skill directory (e.g. `skills/go-test`), which `mct` normalizes to `skills/go-test/SKILL.md` automatically.

#### Cross-market dependencies

Skills can depend on skills from other markets:

```yaml
---
description: Full-stack developer agent
requires_skills:
  - file: skills/go-test/SKILL.md
  - file: skills/python-lint/SKILL.md
    market: https://github.com/acme/python-skills
    pin: def5678
---
```

When a `market` URL is specified and the market isn't already registered, `mct` prompts the user for confirmation and then auto-registers it. In non-interactive mode (`--ci`), unregistered cross-market dependencies are rejected.

### Profile README

A `README.md` at the profile level (`profile/category/README.md`) can have its own frontmatter:

```yaml
---
description: Tools for Go development
tags:
  - golang
  - testing
  - development
---

# Go Development Profile

This profile contains agents and skills for Go development workflows...
```

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Shown in the TUI profile list and used as a fallback description in search results |
| `tags` | list of strings | Boosted in search ranking (repeated 3× internally for BM25 term frequency) |

The README body text is also indexed for full-text search, so any context you write there improves discoverability.

### Profile-level install

Users can install an entire profile in one command:

```bash
mct add acme/agents@dev/go
```

This installs all agents and skills under the `dev/go` profile. Already-installed entries are skipped.

### Example market

```
acme-market/
  dev/
    go/
      README.md
      agents/
        go-developer.md
        go-reviewer.md
      skills/
        go-test/
          SKILL.md
        go-lint/
          SKILL.md
    python/
      README.md
      agents/
        python-developer.md
      skills/
        pytest/
          SKILL.md
  ops/
    docker/
      README.md
      agents/
        dockerfile-writer.md
```

This market has 3 profiles (`dev/go`, `dev/python`, `ops/docker`) containing 5 agents and 3 skills.

---

## Skills-only format

For repositories that contain only skills (no agents, no profile hierarchy), `mct` supports a flat structure:

```
skills-repo/
  skills/
    my-skill/
      SKILL.md
    another-skill/
      SKILL.md
```

This is the format used by Anthropic's official skills repository and other community skill collections. `mct` auto-detects this layout when you register a market by scanning for `<name>/SKILL.md` files under a `skills/` or `skills-catalog/` directory.

For skills-only repos, the profile is derived from the skill path itself (e.g. `skills/my-skill`), and the skill's own description is used as the profile description if no README is present.

### Adding a skills-only repo

```bash
# Direct URL — mct auto-detects the skills-only format (registered as "org/skills-repo")
mct market add https://github.com/org/skills-repo.git

# GitHub /tree/ URL — branch and subpath extracted automatically
mct market add https://github.com/org/repo/tree/main/src/skills

# Explicit skills path as a second argument
mct market add https://github.com/org/repo.git src/skills
```

When using a `/tree/` URL, `mct` parses the branch and subdirectory path automatically. The clone is pruned to keep only the skills folder and `.git`, reducing disk usage.

### Installation

Skills from any format are installed as **copied directories** into `.claude/skills/`:

```
.claude/
  skills/
    my-skill/
      SKILL.md
      helper.txt
```

All files in the skill directory are copied, so supporting files alongside `SKILL.md` are available locally. Files are copied (not symlinked), keeping your project fully self-contained and independent of the cache directory.

---

## Fields injected by mct

When a user installs an entry, `mct` injects tracking fields into the YAML frontmatter. **Do not include these in your market files** — `mct` will reject files that already contain them:

| Field | Description | Example |
|-------|-------------|---------|
| `mct_ref` | Full reference | `acme/agents@dev/go/agents/go-developer.md` |
| `mct_version` | Short SHA and date of the last commit that touched the file | `a1b2c3d·2026-03-15` |
| `mct_market` | Market name | `acme/agents` |
| `mct_profile` | Profile path within the market | `dev/go` |
| `mct_installed_at` | Installation timestamp (RFC 3339 UTC) | `2026-03-20T14:30:00Z` |
| `mct_checksum` | Integrity checksum of the original content | `e5f6a7b8...` |

These fields are placed at the top of the frontmatter block, above your original fields, separated by a blank line.

---

## Market naming

When you register a market, `mct` derives the name from the repository URL. The host is stripped, and the remaining path becomes the name:

| URL | Derived name |
|-----|-------------|
| `https://github.com/acme/agents-repo.git` | `acme/agents-repo` |
| `git@gitlab.com:team/skills.git` | `team/skills` |
| `https://github.com/org/mono/tree/main/skills` | `org/mono` |

URL normalization strips protocol prefixes, `git@host:` SSH prefixes, trailing `.git`, and trailing slashes, then lowercases the result. This means the following URLs all resolve to the same identity:

```
https://github.com/Acme/Repo.git
git@github.com:Acme/Repo.git
https://github.com/acme/repo
```

On disk, the clone directory replaces `/` with `--` (e.g. `~/.cache/mct/acme--agents-repo/`).

---

## Linting

Validate your market structure before publishing:

```bash
mct lint [dir]
```

This reports a summary of profiles, agents, and skills found, then lists any issues. Issues are classified as **errors** (exit code 1) or **warnings**:

**Errors:**
- Invalid or missing frontmatter in an entry file
- Cannot determine entry type from the path (not under `agents/` or `skills/`)
- `requires_skills` references a file that doesn't exist in the market

**Warnings:**
- Profile is missing a `README.md`
- Profile README has no `tags` in its frontmatter
- Profile has no agents or skills

Example output:

```
  profiles: 3  agents: 5  skills: 3

  ~  [ops/docker] missing README.md
  ~  [dev/python] README.md has no tags

  0 error(s)  2 warning(s)
```

---

## Publishing

Push your repo to any Git host (GitHub, GitLab, Bitbucket, Gitea, self-hosted). Users register it with:

```bash
# HTTPS (default) — registered as "org/my-market"
mct market add https://github.com/org/my-market.git

# SSH (requires ssh_enabled) — registered as "org/my-market"
mct config set ssh_enabled true
mct market add git@github.com:org/my-market.git
```

The market name is auto-derived from the URL (host is stripped, e.g. `org/my-market`). URLs without a protocol get `https://` prepended automatically, so `github.com/org/my-market` works too.

### Market add options

| Flag | Description |
|------|-------------|
| `--branch <name>` | Track a specific branch (default: `main`) |
| `--trusted` | Skip breaking change confirmations for this market |
| `--read-only` | Index the market for search but never install from it |
| `--no-clone` | Register the market in config without cloning the repo |
| `--json` | Output result as JSON |

Users can change the tracked branch later:

```bash
mct market set acme/agents branch develop
```

### Market info

Users can inspect a registered market:

```bash
mct market info acme/agents
```

This shows the market name, URL, branch, trusted/read-only/skills-only flags, skills path (if applicable), total entry count, installed count, sync status, and last sync timestamp.

---

## Default markets

When a user runs `mct init` for the first time, `mct` auto-registers a set of community markets from an embedded registry. These include Anthropic's official skills repository and several popular community collections. Users can remove any of them with `mct market remove`.

---

## Versioning

`mct` uses Git commit SHAs for version tracking — there is no separate version numbering scheme to maintain. Each installed entry records the commit SHA at the time of installation. When `mct sync` runs, it diffs the current HEAD against the recorded SHA and applies only the changes that affect installed files.

The version displayed to users is a compact format: the first 7 characters of the SHA followed by the commit date (e.g. `a1b2c3d·2026-03-15`).

---

## Search indexing

All registered markets are indexed for search. `mct` builds one BM25 document per profile by combining the profile path tokens, README content, README description, tags, and all entry descriptions and filenames. Tags and descriptions are boosted (repeated 3× for term frequency). English stemming (Snowball) is applied to all tokens, and fuzzy matching (Levenshtein distance ≤ 2) expands query terms that have no exact match in the vocabulary.

Search supports filtering by type (`agent` or `skill`), market, category, installed status, and result limit. Deleted entries are excluded by default but can be included with `--include-deleted`.

---

## Conflict and drift policies

Two global configuration settings control how `mct` handles edge cases:

- **`conflict_policy`** (`block` or `skip`): What to do when two markets provide an entry with the same ref. `block` prevents installation; `skip` silently ignores the conflict.
- **`drift_policy`** (`prompt`, `force`, or `skip`): What to do when an installed file has been modified locally and an upstream update arrives. `prompt` asks the user interactively; `force` overwrites; `skip` keeps the local version.

Set them with:

```bash
mct config set conflict_policy block
mct config set drift_policy prompt
```
