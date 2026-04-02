# outbound/cfgadapter — Config, State & InstallDB Adapter

**Location**: `internal/mercato/outbound/cfgadapter/`

## Purpose

Implements three repository interfaces for persisting configuration (YAML), sync state (JSON), and the install database (JSON with file locking).

## Files

### configstore.go — ConfigStore Implementation

Persists `~/.config/mct/config.yml`.

| Method | Description |
|--------|-------------|
| `Load(path)` | Read and parse YAML config, apply defaults |
| `Save(path, cfg)` | Write YAML config (atomic, 0600 perms) |
| `Exists(path)` | Check if config file exists |
| `AddMarket(path, market)` | Append market to config |
| `RemoveMarket(path, name)` | Remove market from config |
| `SetMarketProperty(path, name, key, value)` | Update a market field |
| `SetConfigField(path, key, value)` | Update a top-level config field |

### statestore.go — StateStore Implementation

Persists `~/.cache/mct/sync-state.json`.

| Method | Description |
|--------|-------------|
| `LoadSyncState(cacheDir)` | Read JSON sync state |
| `SaveSyncState(cacheDir, state)` | Write JSON sync state |
| `SetMarketSyncDirty(cacheDir, market)` | Mark market sync as incomplete |
| `SetMarketSyncClean(cacheDir, market, sha, branch)` | Record successful sync with SHA and branch |

### installdb.go — InstallDB Implementation

Persists `~/.cache/mct/installed.json` with file-level locking.

| Method | Description |
|--------|-------------|
| `Load(cacheDir)` | Read installed.json |
| `Save(cacheDir, db)` | Write installed.json (atomic, 0600 perms) |
| `Lock(cacheDir)` | Acquire `.lock` file (5s timeout, stale PID detection) |
| `Unlock(cacheDir)` | Release `.lock` file |

### Locking Strategy

1. Create `installed.json.lock` containing current PID
2. If lock exists, read PID and check if process is alive
3. If stale (process dead), remove and retry
4. If live, wait up to 5 seconds with polling
5. Returns `LOCK_CONTENTION` or `STALE_LOCK` errors on failure
