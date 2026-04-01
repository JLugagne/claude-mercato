# Creating a market

A market is a Git repository with a specific directory structure. `mct` indexes it to let users discover, install, and update Claude agent and skill definitions.

`mct` supports two repository formats: the **mct market format** (hierarchical, with profiles) and the **skills-only format** (flat, for pure skill collections).

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
- Agent files must be `.md` and live under an `agents/` directory
- Skill definitions use a directory structure: `skills/<name>/SKILL.md`
- The first two path segments form the **profile** (e.g. `profile-a/category-x`)
- A `README.md` at the profile level (`profile/category/README.md`) provides metadata for all entries in that profile
- Everything else is ignored by `mct`

### Entry type

The entry type (agent or skill) is **inferred from the path**, not declared in frontmatter. A file under `agents/` is an agent; a `SKILL.md` under `skills/<name>/` is a skill.

### Entry frontmatter

Every agent or skill file must start with YAML frontmatter:

```yaml
---
description: Short description of what this does
author: Your Name
---

Your agent/skill prompt content here...
```

#### Required fields

| Field | Description |
|-------|-------------|
| `description` | Short description, used in search results |

#### Optional fields

| Field | Type | Description |
|-------|------|-------------|
| `author` | string | Author name |
| `breaking_change` | bool | Flags a breaking change in this version |
| `deprecated` | bool | Marks entry as deprecated |
| `requires_skills` | list | Skills this entry depends on (see below) |

### Skill dependencies

An agent can declare that it requires specific skills:

```yaml
---
description: Go developer agent
requires_skills:
  - file: skills/go-test/SKILL.md
  - file: skills/go-lint/SKILL.md
    pin: abc1234
---
```

When a user installs this agent, `mct` auto-installs the required skills. The `file` path is relative to the market root. An optional `pin` locks the dependency to a specific commit SHA.

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

When a `market` URL is specified, `mct` will auto-register that market if it isn't already configured, then install the dependency from it.

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
| `description` | string | Shown in the TUI profile list |
| `tags` | list of strings | Used for search ranking |

The README body is also indexed for full-text search.

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

This is the format used by Anthropic's skills repository and other community skill collections. `mct` auto-detects this layout when you register a market.

### Adding a skills-only repo

```bash
# Direct URL
mct market add skillsrepo https://github.com/org/skills-repo.git

# GitHub /tree/ URL -- auto-extracts branch and subpath
mct market add https://github.com/org/repo/tree/main/src/skills
```

When using a `/tree/` URL, `mct` parses the branch and subdirectory path automatically. The clone is pruned to keep only the skills folder, reducing disk usage.

### Installation

Skills are installed as **symlinked directories** into `.claude/skills/`:

```
.claude/
  skills/
    my-skill/  -->  ~/.cache/mct/skillsrepo/skills/my-skill/
```

This means the entire skill directory (including any supporting files alongside `SKILL.md`) is available locally.

## Fields injected by mct

When a user installs an entry, `mct` injects tracking fields into the frontmatter. **Do not include these in your market files** -- `mct` will reject files that already contain them:

- `mct_ref` -- Full reference (e.g. `mymarket@dev/go/agents/go-developer.md`)
- `mct_version` -- Short SHA and date of the last commit that touched the file
- `mct_market` -- Market name
- `mct_installed_at` -- Installation timestamp

## Linting

Validate your market structure before publishing:

```bash
mct lint [dir]
```

This checks for:
- Valid frontmatter in all entry files
- Correct directory structure (agents/, skills/)
- Missing profile READMEs
- Broken `requires_skills` references
- Profiles with no agents or skills

Returns exit code 1 if errors are found.

## Publishing

Push your repo to any Git host (GitHub, GitLab, self-hosted). Users register it with:

```bash
# SSH
mct market add mymarket git@github.com:org/my-market.git

# HTTPS
mct market add mymarket https://github.com/org/my-market.git
```

SSH requires `mct config set ssh_enabled true` or `MCT_SSH_ENABLED=true`.

Markets track a branch (default: `main`). Users can change it with:

```bash
mct market set mymarket branch develop
```
