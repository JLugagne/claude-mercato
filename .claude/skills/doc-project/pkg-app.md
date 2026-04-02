# app — Application Logic Layer

**Location**: `internal/mercato/app/`

## Purpose

The central business logic layer. The `App` struct implements all service interfaces from `domain/service/` and orchestrates domain types with repository adapters.

## App Struct

```go
type App struct {
    git        gitrepo.GitRepo
    fs         filesystem.Filesystem
    cfg        configstore.ConfigStore
    state      statestore.StateStore
    idb        installdb.InstallDB
    configPath string
    cacheDir   string
}
```

## Files

### market.go — Market Management

Implements `MarketQueries` + `MarketCommands`.

| Method | Description |
|--------|-------------|
| `ListMarkets()` | Load config, convert MarketConfig to Market types |
| `GetMarket(name)` | Find specific market by name |
| `MarketInfo(name)` | Market details with entry counts and sync status |
| `AddMarket(url, opts)` | Clone repo (shallow), validate structure, auto-detect format, add to config |
| `RemoveMarket(name, opts)` | Remove from config, optionally delete cached clone |
| `SetMarketProperty()` | Update a market field in config |

Helpers: `marketNameFromURL()`, `isSkillPath()`, `marketDirName()` (converts `a/b` to `a--b`), `projectPath()`, `findMarketConfig()`

### entry.go — Entry Install/Remove

Implements `EntryQueries` + `EntryCommands`.

| Method | Description |
|--------|-------------|
| `List(opts)` | List installed entries at current project location |
| `GetEntry(ref)` | Get specific entry by MctRef |
| `ReadEntryContent()` | Read raw file from market repo |
| `ListSkillDirFiles()` | List all files in a skill directory |
| `Add(ref, opts)` | Install entry + resolve dependencies recursively, inject mct fields, write to disk, update installdb |
| `Remove(ref, opts)` | Uninstall from current location, update installdb |
| `Prune(opts)` | Clean up deleted upstream entries |
| `Init(opts)` | Create `.claude/` directory, initialize config |
| `Conflicts()` | Detect filename collisions across markets |

### sync.go — Synchronization

Implements `SyncQueries` + `SyncCommands`.

| Method | Description |
|--------|-------------|
| `Check(opts)` | Compute EntryState for all installed entries (clean/update/drift/deleted/orphaned) |
| `Refresh(opts)` | Fetch from all markets, build diff list |
| `Update(opts)` | Apply pending changes with drift/conflict handling |
| `Sync(opts)` | Combined Refresh + Update |
| `detectDrift()` | Compare local file xxhash vs upstream content at recorded version |

### search.go — Full-Text Search

Implements `SearchQueries`.

| Method | Description |
|--------|-------------|
| `Search(query, opts)` | BM25 search with Snowball stemming, fuzzy matching (Levenshtein <= 2), tag boosting (3x) |
| `DumpIndex()` | Export all entries |
| `BenchIndex()` | Performance metrics |

Helpers: `buildCorpus()`, `filterEntries()`, `buildProfileDoc()`, `tokenize()`, `expandFuzzy()`

### lint.go — Market Validation

| Method | Description |
|--------|-------------|
| `LintMarket(name)` | Validate frontmatter, path structure, README presence, skill deps |

### conflict.go — Collision Detection

| Method | Description |
|--------|-------------|
| `Conflicts()` | Find files with same name across markets |

### readme.go — README Queries

| Method | Description |
|--------|-------------|
| `Readme(market, path)` | Read specific profile README |
| `ListReadmes(market)` | List all READMEs in market |
