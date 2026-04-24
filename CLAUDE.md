# agents-mercato

ALWAYS load doc-project skill, NEVER explore the code first. Explore the code only if you don't find the information in doc-project skill.

## Documentation maintenance

After any architecture change, technical decision, or package restructuring, update the relevant `doc-project` skill files in `.claude/skills/doc-project/`:

- **SKILL.md** — General overview, architecture diagram, package index, key concepts
- **pkg-*.md** — Per-package detail files (types, interfaces, methods, helpers)

This includes but is not limited to:
- Adding, removing, or renaming packages
- Changing interfaces or port/adapter contracts
- Adding new CLI commands or TUI features
- Modifying domain types, error codes, or data models
- Changing the DI wiring or bootstrap flow
- Altering sync, search, or dependency resolution logic
