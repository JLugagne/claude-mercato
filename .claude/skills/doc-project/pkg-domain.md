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
| `install.go` | `InstallDatabase`, `InstalledMarket`, `InstalledPackage`, `InstalledFiles` | Install DB model with `FindPackage()`, `AddOrUpdatePackage()`, `RemoveLocation()`, `CleanStaleLocations()` |
| `diff.go` | `DiffAction`, `FileDiff` | Git diff representation (insert/modify/delete) |

## Key Types

### MctRef
Canonical reference: `"market@profile/category/type/name"`. Methods: `Parse()` splits into market + path.

### Entry
Full representation of an agent or skill with metadata, version, state, dependencies, and profile context.

### EntryState
Enum: `Clean`, `UpdateAvailable`, `Drift`, `UpdateAndDrift`, `Deleted`, `NewInRegistry`, `Orphaned`, `Unknown`.

### Frontmatter
Parsed from YAML header in .md files. Fields: name, description, author, version, tags, deprecated, breaking_change, requires_skills. Mct-injected fields: mct_ref, mct_version, mct_market, mct_profile, mct_installed_at, mct_checksum.

### InstallDatabase
Tracks all installed packages across all projects. Keyed by market name, then by profile. Each package records its files (skills + agents) and locations (project paths).

## Error Codes

Grouped by category:
- **Market**: MARKET_NOT_FOUND, MARKET_ALREADY_EXISTS, MARKET_URL_EXISTS, MARKET_UNREACHABLE
- **Entry**: ENTRY_NOT_FOUND, ENTRY_ALREADY_INSTALLED, ENTRY_ORPHANED
- **Validation**: INVALID_FRONTMATTER, INVALID_ENTRY_TYPE, MCT_FIELDS_IN_REPO
- **Skill**: SKILL_NOT_FOUND, SKILL_TYPE_MISMATCH, PIN_MISMATCH
- **Sync**: SYNC_DIRTY, CACHE_STALE, OFFLINE_MODE
- **Conflict**: CONFLICT_REF_COLLISION, CONFLICT_DEP_VERSION, CONFLICT_DEP_DELETED
- **Infrastructure**: CLONE_EXISTS, SSH_DISABLED, LOCK_CONTENTION, STALE_LOCK, DRIFT_DETECTED, DEPENDENCY_CYCLE, PATH_TRAVERSAL
