# domain/service — Port Interfaces

**Location**: `internal/mercato/domain/service/`

## Purpose

Defines the contracts (ports) between the application layer and consumers. Each file declares query and command interfaces that the `app` package implements.

## Files & Interfaces

### market.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `MarketQueries` | `ListMarkets()`, `GetMarket(name)`, `MarketInfo(name)` | Read-only market access |
| `MarketCommands` | Embeds `MarketQueries` + `AddMarket()`, `RemoveMarket()`, `SetMarketProperty()`, `LintMarket()` | Market lifecycle management |

Result types: `AddMarketResult`, `RemoveMarketOpts`, `MarketInfoResult`, `LintResult`

### entry.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `EntryQueries` | `List(opts)`, `GetEntry(ref)`, `ReadEntryContent()`, `ListSkillDirFiles()`, `Conflicts()` | Read entries and detect conflicts |
| `EntryCommands` | Embeds `EntryQueries` + `Add(ref, opts)`, `Remove(ref, opts)`, `Prune(opts)`, `Init(opts)` | Install, remove, prune, init |

Options: `ListOpts`, `AddOpts` (Location, Force, AcceptBreaking), `RemoveOpts`, `InitOpts`, `PruneOpts`

### sync.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `SyncQueries` | `Check(opts)`, `SyncState()` | Status check, sync state |
| `SyncCommands` | `Refresh(opts)`, `Update(opts)`, `Sync(opts)` | Fetch, apply, or both |

Options: `CheckOpts`, `RefreshOpts`, `UpdateOpts`, `SyncOpts`
Results: `RefreshResult`, `UpdateResult`, `SyncResult`

### search.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `SearchQueries` | `Search(query, opts)`, `DumpIndex()`, `BenchIndex()` | Full-text search and indexing |

`SearchOpts`: Type, Market, Category, Installed filters
`SearchResult`: Entry + BM25 score

### readme.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `ReadmeQueries` | `Readme(market, path)`, `ListReadmes(market)` | Profile README access |

### config.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `ConfigCommands` | `SetConfigField()`, `GetConfig()` | Configuration management |
