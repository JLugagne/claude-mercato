# Creating a market

A market is a Git repository with a specific directory structure. `mct` indexes it to let users discover, install, and update Claude agent and skill definitions.

## Repository structure

```
your-market/
  profile-a/
    category-x/
      README.md              # Profile description and tags
      agents/
        my-agent.md          # Agent definition
      skills/
        helper-skill.md      # Skill definition
  profile-b/
    category-y/
      README.md
      agents/
        another-agent.md
      skills/
        another-skill.md
```

Key rules:
- Files must be `.md` and live under an `agents/` or `skills/` directory
- The first two path segments form the **profile** (e.g. `profile-a/category-x`)
- A `README.md` at the profile level (`profile/category/README.md`) provides metadata for all entries in that profile
- Everything else is ignored by `mct`

## Entry frontmatter

Every agent or skill file must start with YAML frontmatter:

```yaml
---
type: agent          # "agent" or "skill"
description: Short description of what this does
author: Your Name
---

Your agent/skill prompt content here...
```

### Required fields

| Field | Values | Description |
|-------|--------|-------------|
| `type` | `agent` or `skill` | Must match the parent directory |
| `description` | string | Short description, used in search results |

### Optional fields

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
type: agent
description: Go developer agent
requires_skills:
  - file: skills/go-test.md
  - file: skills/go-lint.md
    pin: abc1234
---
```

When a user installs this agent, `mct` auto-installs the required skills. The `file` path is relative to the market root. An optional `pin` locks the dependency to a specific commit SHA.

## Profile README

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

## Example market

```
acme-market/
  dev/
    go/
      README.md
      agents/
        go-developer.md
        go-reviewer.md
      skills/
        go-test.md
        go-lint.md
    python/
      README.md
      agents/
        python-developer.md
      skills/
        pytest.md
  ops/
    docker/
      README.md
      agents/
        dockerfile-writer.md
```

This market has 3 profiles (`dev/go`, `dev/python`, `ops/docker`) containing 5 agents and 3 skills.

## Fields injected by mct

When a user installs an entry, `mct` injects tracking fields into the frontmatter. **Do not include these in your market files** -- `mct` will reject files that already contain them:

- `mct_ref` -- Full reference (e.g. `mymarket/dev/go/agents/go-developer.md`)
- `mct_version` -- Short SHA and date of the last commit that touched the file
- `mct_market` -- Market name
- `mct_installed_at` -- Installation timestamp

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
