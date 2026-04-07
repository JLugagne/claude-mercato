# mct — The Package Manager for AI Context & Intelligence

**Stop copy-pasting prompts. Start versioning your AI's brain.**

`mct` (Claude Mercato) is a lightweight CLI that treats AI agent definitions, rules, and skills as **versioned software dependencies**. It synchronizes your team's best practices across every IDE (Cursor, Windsurf, Claude Code, Gemini) and every CI/CD pipeline, automatically.

No central registry. No hosted service. No account required. Just Git.

---

## ⚡ Why mct?

In the AI-native development era, **prompts and skills are the new Linters**. If your team uses different versions of a prompt, they get inconsistent code. If your CI/CD uses an outdated audit skill, you miss vulnerabilities.

### The Problem: "Context Drift"
- **The Copy-Paste Trap:** You fix a prompt in one repo, but 10 other projects still use the old, buggy version.
- **IDE Fragmentation:** Maintaining `.cursorrules`, `.clauderules`, `.windsurfrules`, and `GEMINI.md` manually is a maintenance nightmare.
- **The "Invisible" CI:** Your pipelines run AI reviews with static prompts that never evolve without a manual rebuild of your Docker images or CI config.

### The Solution: `mct`
`mct` turns any Git repository into an "Intelligence Market." It manages the full lifecycle of your AI context: **Discovery, Installation, Transformation, and Global Synchronization.**

---

## 🌟 Core Value Pillars

### 1. Multi-IDE Source of Truth
Write your rules once in Markdown. `mct` automatically transforms and deploys them into the native formats of all major AI coding tools.
- **Claude Code** (`.claude/`)
- **Cursor** (`.cursor/rules/*.mdc` with YAML frontmatter)
- **Windsurf** (`.windsurf/rules/` with trigger support)
- **Gemini & Codex** (`.gemini/`, `.codex/`)
- **OpenCode** (`.opencode/` with full model/tool mapping)

### 2. Continuous Intelligence (CI/CD)
Update your central "Market" repo, and **100+ pipelines are instantly updated**. 
By running `mct restore` in your CI, your AI code reviewers always use the latest security skills and architectural rules, without ever touching a Dockerfile or a config string. This ensures your "Reviewer Agent" is always as smart as your "Developer Agent."

### 3. Team Alignment (The `.mct.json` Lockfile)
Just like `package.json`, `.mct.json` ensures every developer on the project works with the exact same AI context.
- `mct save`: Freeze your current AI setup (markets and installed profiles).
- `mct restore`: A new teammate clones the repo and gets the same "AI brain" instantly.
- **Custom Configs**: Use `--file` (or `-f`) to manage different setups (e.g. `mct restore -f .mct.review.json` for CI).
- **Git Hooks**: Auto-restore after `git pull` and auto-save before `git push` to eliminate manual sync.

### 4. Smart Dependency Resolver
Skills can depend on other skills, even across different Git repositories. `mct` resolves the dependency graph and installs everything needed to make your agents functional, ensuring you never have a "missing skill" error during a complex AI task.

---

## 🚀 Quick Start

### 1. Install
```bash
go install github.com/JLugagne/claude-mercato/cmd/mct@latest
```

### 2. Initialize and Add a Market
`mct` comes pre-configured with standard markets (Anthropic, etc.), but you can point it at any Git repository (public or private):
```bash
mct init
mct market add github.com/your-org/ai-standards
```

### 3. Discover and Install
Refs use the format `market@path`. Search across all markets with full-text BM25 ranking:
```bash
mct search "go testing"
mct add your-org/ai-standards@dev/go/testing
```

### 4. Global Sync
Update **every skill in every project** on your machine to the latest upstream version in one command:
```bash
mct sync
```

---

## 🛠 Features for Power Users

### Smart Drift Detection
`mct` uses **xxHash checksums** to detect if you've modified a local rule. It will never silently overwrite your custom tweaks. When syncing, it flags conflicts and lets you decide: keep your version, overwrite with upstream, or handle entries individually.

### Multi-Project State Management
`mct` maintains a global state of where every skill is installed. When you run `mct sync`, it doesn't just update the current directory; it updates the "AI logic" across your entire workstation.

### Professional Search Engine
Forget `grep`. `mct` features a built-in search engine with **Snowball stemming** and **Levenshtein distance** fuzzy matching (≤ 2). It indexes your cached markets locally for instant, offline-first discovery.

### SSH & Private Repos
First-class support for private repositories via system SSH agents and `~/.ssh/config`. Perfect for internal company "Agent Libraries."

---

## 📊 Supported Tools

| Tool | Agents | Skills | Output format |
|------|--------|--------|---------------|
| **Claude Code** | Yes | Yes | `.claude/agents/*.md`, `.claude/skills/*/SKILL.md` |
| **Cursor** | — | Yes | `.cursor/rules/*.mdc` (Native YAML frontmatter) |
| **Windsurf** | — | Yes | `.windsurf/rules/*.md` (Native YAML frontmatter) |
| **Codex** | — | Yes | `.codex/skills/*.md` (Plain Markdown) |
| **Gemini CLI** | — | Yes | `.gemini/rules/*.md` (Stripped Frontmatter) |
| **OpenCode** | Yes | Yes | Full YAML frontmatter with model/tool mapping |

---

## 🔒 Privacy & Infrastructure

- **Zero Infrastructure:** No central server, no account, no telemetry. It never "phones home."
- **Git as a Backend:** Works with GitHub, GitLab, Bitbucket, Gitea, or self-hosted Git.
- **Offline First:** Once a market is cached, search and installation work without an internet connection.

---

## 📦 Creating Your Own Market
See [MARKET.md](MARKET.md) for the full specification. In short: organize your repo with `agents/` and `skills/` directories. Use YAML frontmatter to describe capabilities and list dependencies via `requires_skills`.

## License
MIT — See [LICENSE](LICENSE).
