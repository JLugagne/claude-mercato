# outbound/fsadapter — Filesystem Adapter

**Location**: `internal/mercato/outbound/fsadapter/`

## Purpose

Implements the `Filesystem` interface using the standard `os` package and `fs.FS`. Provides read/write/symlink operations and MD5 checksums.

## Constructor

```go
New() *FSAdapter         // Rooted at current working directory
NewAt(root string) *FSAdapter  // Rooted at specific path
```

## Files

### fsadapter.go — Core Implementation

| Method | Description |
|--------|-------------|
| `Open()`, `ReadFile()`, `ReadDir()`, `Stat()` | Delegates to `os.DirFS` (implements `ReadFS`) |
| `WriteFile(path, data, perm)` | Write with automatic `MkdirAll` for parent dirs |
| `DeleteFile(path)` | Remove single file |
| `MkdirAll(path, perm)` | Create directory tree |
| `RemoveAll(path)` | Delete directory tree |
| `MD5Checksum(content)` | Hash content for drift detection |
| `Symlink(target, link)` | Create symbolic link |
| `Readlink(path)` | Read symlink target |
| `IsSymlink(path)` | Check if path is a symlink |
| `ListDir(path)` | List directory contents |

### fsutil.go — Path Utilities

Path normalization helpers for consistent path handling across the adapter.
