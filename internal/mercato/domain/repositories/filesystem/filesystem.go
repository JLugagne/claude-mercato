package filesystem

import "io/fs"

// ReadFS is the read-only view of the filesystem. testing/fstest.MapFS satisfies this directly.
type ReadFS interface {
	fs.FS
	fs.ReadFileFS
	fs.ReadDirFS
	fs.StatFS
}

// Filesystem is the full abstraction used by the app layer.
// Read methods follow fs.FS path semantics (no leading slash, forward slashes only).
type Filesystem interface {
	ReadFS
	WriteFile(path string, content []byte) error
	DeleteFile(path string) error
	MkdirAll(path string) error
	RemoveAll(path string) error
	MD5Checksum(content []byte) string
	Symlink(target, link string) error
	Readlink(path string) (string, error)
	IsSymlink(path string) bool
	ListDir(path string) ([]string, error)
}
