# domain/repositories — Adapter Interfaces

**Location**: `internal/mercato/domain/repositories/`

## Purpose

Defines the outbound port interfaces that adapters must implement. These abstractions decouple the app layer from infrastructure concerns (filesystem, git, config persistence).

## Interfaces

### filesystem/filesystem.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `ReadFS` | Composed from `fs.FS`, `fs.ReadFileFS`, `fs.ReadDirFS`, `fs.StatFS` | Read-only FS operations |
| `Filesystem` | Embeds `ReadFS` + `WriteFile()`, `DeleteFile()`, `MkdirAll()`, `RemoveAll()`, `MD5Checksum()`, `Symlink()`, `Readlink()`, `IsSymlink()`, `ListDir()` | Full FS with writes, checksums, symlinks |

### gitrepo/gitrepo.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `GitRepo` | `Clone()`, `DefaultBranch()`, `Fetch()`, `DiffSinceCommit()`, `ReadFileAtRef()`, `FileVersion()`, `RemoteHEAD()`, `ListFiles()`, `IsValidRepo()`, `ValidateRemote()`, `ReadMarketFiles()`, `ListDirFiles()` | Git operations against market repos |

Supporting type: `MarketFile` — file content + version SHA tuple

### configstore/configstore.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `ConfigStore` | `Load()`, `Save()`, `Exists()`, `AddMarket()`, `RemoveMarket()`, `SetMarketProperty()`, `SetConfigField()` | YAML config persistence |

### statestore/statestore.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `StateStore` | `LoadSyncState()`, `SaveSyncState()`, `SetMarketSyncDirty()`, `SetMarketSyncClean()` | Sync state persistence (sync-state.json) |

### installdb/installdb.go

| Interface | Methods | Description |
|-----------|---------|-------------|
| `InstallDB` | `Load()`, `Save()`, `Lock()`, `Unlock()` | Install database with file locking (5s timeout, stale PID detection) |
