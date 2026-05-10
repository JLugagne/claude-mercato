# domain — Core Domain Types & Logic

**Location**: `internal/mercato/domain/`

## Purpose

Defines all core types, error codes, frontmatter parsing, sync state, and the install database data model. No external dependencies — pure domain logic.

## Files

| File | Key Types / Functions | Description |
|------|----------------------|-------------|
| `types.go` | `MctRef`, `MctVersion`, `EntryType`, `Market`, `Entry`, `SkillDep`, `Tombstone`, `EntryState`, `Conflict` | Core type definitions |
| `config.go` | `Config`, `MarketConfig`, `NormalizeURL()` | Application configuration model |
| `errors.go` | `DomainError`, error codes (MARKET_NOT_FOUND, ENTRY_NOT_FOUND, INVALID_FRONTMATTER, etc.) | ~20 typed domain errors |
| `frontmatter.go` | `Frontmatter`, `ReadmeFrontmatter`, `ParseFrontmatter()`, `InjectMctFields()`, `PatchMctVersion()`, `PatchMctChecksum()` | YAML frontmatter extraction, injection, patching |
| `state.go` | `SyncState`, `MarketSyncState` | Per-market sync tracking (SHA, timestamp, branch, status) |
| `install.go` | `InstallDatabase`, `InstalledMarket`, `InstalledPackage`, `InstalledLocation`, `InstalledFile`, `InstalledFiles`, `InstallSchemaVersion` | Install DB model (v2 schema) with `FindPackage()`, `FindLocation()`, `AddOrUpdatePackage()` (replace-wholesale), `MergeLocationFiles()`, `MergePackageFiles()` (caller-side composition for incremental adds), `RemoveLocation()`, `CleanStaleLocations()` |
| `runtime_type.go` | `RuntimeTypeClaudeCode`, `RuntimeTypeForDotDir()` | Runtime-type label constants and folder→type lookup used by v1→v2 migration |
| `diff.go` | `DiffAction`, `FileDiff` | Git diff representation (insert/modify/delete) |

## Key Types

### MctRef
Canonical reference: `"market@profile/category/type/name"`. Methods: `Parse()` splits into market + path.

### Entry
Full representation of an agent, skill, or command with metadata, version, state, dependencies, and profile context. `EntryType` constants: `EntryTypeAgent`, `EntryTypeSkill`, `EntryTypeCommand`.

### EntryState
Enum: `Clean`, `UpdateAvailable`, `Drift`, `UpdateAndDrift`, `Deleted`, `NewInRegistry`, `Orphaned`, `Unknown`.

### Frontmatter
Parsed from YAML header in .md files. Fields: name, description, author, version, tags, deprecated, breaking_change, requires_skills. Mct-injected fields: mct_ref, mct_version, mct_market, mct_profile, mct_installed_at, mct_checksum.

### InstallDatabase (schema v2)
Tracks all installed packages across all projects. Keyed by market name, then by profile. Each package carries:
- `Files` — package-wide skill/agent/command leaf names (drives sync diffs and ref enumeration). `InstalledFiles` has `Skills`, `Agents`, and `Commands []string` fields.
- `Locations []InstalledLocation` — one entry per (project path, runtime type). The same project path appears multiple times when a package was installed for several runtimes (e.g. claude-code + cursor).

`InstalledLocation { Path, Type, Files []InstalledFile }` — `Type` is `claude-code`, `cursor`, `windsurf`, etc. (taken from `Transformer.ToolName()` for non-claude tools, the constant `RuntimeTypeClaudeCode` for the built-in path). `Files []InstalledFile { Path, XXH }` records every file actually written, with its xxhash64 at install/update time. This is the source of truth for drift detection AND for prune-on-update: each `AddOrUpdatePackage` call replaces the (Path, Type) entry's `Files` wholesale, and the sync flow diffs old vs. new to delete files dropped upstream. For single-entry adds the caller composes the full set with `MergeLocationFiles` / `MergePackageFiles` before calling.

`InstallSchemaVersion = 2`. The cfgadapter auto-migrates v1 (`Locations []string` + flat `ToolChecksums` map) on load by walking each location's on-disk `.claude/` tree, hashing every file present, and classifying by the leading dot-folder via `RuntimeTypeForDotDir`. The migrated DB is persisted back so subsequent loads are fast.

## Error Codes

Grouped by category:
- **Market**: MARKET_NOT_FOUND, MARKET_ALREADY_EXISTS, MARKET_URL_EXISTS, MARKET_UNREACHABLE
- **Entry**: ENTRY_NOT_FOUND, ENTRY_ALREADY_INSTALLED, ENTRY_ORPHANED
- **Validation**: INVALID_FRONTMATTER, INVALID_ENTRY_TYPE, MCT_FIELDS_IN_REPO
- **Skill**: SKILL_NOT_FOUND, SKILL_TYPE_MISMATCH, PIN_MISMATCH
- **Sync**: SYNC_DIRTY, CACHE_STALE, OFFLINE_MODE
- **Conflict**: CONFLICT_REF_COLLISION, CONFLICT_DEP_VERSION, CONFLICT_DEP_DELETED
- **Infrastructure**: CLONE_EXISTS, SSH_DISABLED, LOCK_CONTENTION, STALE_LOCK, DRIFT_DETECTED, DEPENDENCY_CYCLE, PATH_TRAVERSAL
