# outbound/gitadapter — Git Adapter

**Location**: `internal/mercato/outbound/gitadapter/`

## Purpose

Implements the `GitRepo` interface using go-git. Handles all Git operations: clone, fetch, diff, file reading, and remote validation.

## Constructor

```go
New(opts ...Option) *GitAdapter
```

### Options
- `WithSSHEnabled()` — Enable SSH authentication
- `WithDepth(n)` — Set shallow clone depth (default: 1)

## Files

### gitadapter.go — Core Implementation

| Method | Description |
|--------|-------------|
| `Clone(url, path, branch)` | Shallow clone with optional SSH auth |
| `DefaultBranch(path)` | Detect default branch from repo |
| `Fetch(path, branch)` | Fetch updates from remote |
| `DiffSinceCommit(path, sha)` | Compute file changes since a commit |
| `ReadFileAtRef(path, ref, file)` | Read file content at specific commit/branch |
| `FileVersion(path, file)` | Get last-modified commit SHA for a file |
| `RemoteHEAD(url)` | Get remote HEAD SHA without cloning |
| `ListFiles(path)` | List all .md files in repo |
| `IsValidRepo(path)` | Check if path contains a valid git repo |
| `ValidateRemote(url)` | Validate URL is reachable before cloning |
| `ReadMarketFiles(path, branch)` | Read all market entries in one pass |
| `ListDirFiles(path, dir, ref)` | List files in a specific directory at a ref |

### helpers.go — SSH & Git Utilities

| Function | Description |
|----------|-------------|
| `isSSHURL(url)` | Detect SSH URL format |
| `resolveAuth(url)` | Build SSH auth for clone operations |
| `resolveAuthFromRepo(path)` | Build SSH auth from existing repo remote |
| SSH key loading | Tries `~/.ssh/` keys: ed25519, ecdsa, rsa |
| SSH config parsing | Reads `~/.ssh/config` for per-host IdentityFile |
| SSH agent support | Loads from `SSH_AUTH_SOCK` |
| Known hosts | Validates against `~/.ssh/known_hosts` |
| Tree walking | Helpers for traversing git trees and computing diffs |
