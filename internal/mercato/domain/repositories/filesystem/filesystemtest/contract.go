package filesystemtest

import (
	"io/fs"
	"testing/fstest"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem"
)

var _ filesystem.Filesystem = (*MockFilesystem)(nil)

// MockFilesystem implements filesystem.Filesystem for tests.
// Populate FS with a fstest.MapFS for read operations.
// Override write operations with function fields as needed.
type MockFilesystem struct {
	// FS backs all read operations (Open, ReadFile, ReadDir, Stat).
	// Defaults to an empty MapFS if nil.
	FS fstest.MapFS

	// StatFn overrides Stat when set (useful for testing FileExists edge cases).
	StatFn func(name string) (fs.FileInfo, error)

	// ReadFileFn overrides ReadFile when set (useful for call-counting in tests).
	ReadFileFn func(name string) ([]byte, error)

	WriteFileFn      func(path string, content []byte) error
	DeleteFileFn     func(path string) error
	MkdirAllFn       func(path string) error
	RemoveAllFn      func(path string) error
	MD5ChecksumFn    func(content []byte) string
	SymlinkFn        func(target, link string) error
	ReadlinkFn       func(path string) (string, error)
	IsSymlinkFn      func(path string) bool
	ListDirFn        func(path string) ([]string, error)
}

func (m *MockFilesystem) mapFS() fstest.MapFS {
	if m.FS != nil {
		return m.FS
	}
	return fstest.MapFS{}
}

func (m *MockFilesystem) Open(name string) (fs.File, error) {
	return m.mapFS().Open(name)
}

func (m *MockFilesystem) ReadFile(name string) ([]byte, error) {
	if m.ReadFileFn != nil {
		return m.ReadFileFn(name)
	}
	return fs.ReadFile(m.mapFS(), name)
}

func (m *MockFilesystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(m.mapFS(), name)
}

func (m *MockFilesystem) Stat(name string) (fs.FileInfo, error) {
	if m.StatFn != nil {
		return m.StatFn(name)
	}
	return fs.Stat(m.mapFS(), name)
}

func (m *MockFilesystem) WriteFile(path string, content []byte) error {
	if m.WriteFileFn == nil {
		panic("called not defined WriteFileFn")
	}
	return m.WriteFileFn(path, content)
}

func (m *MockFilesystem) DeleteFile(path string) error {
	if m.DeleteFileFn == nil {
		panic("called not defined DeleteFileFn")
	}
	return m.DeleteFileFn(path)
}

func (m *MockFilesystem) MkdirAll(path string) error {
	if m.MkdirAllFn == nil {
		panic("called not defined MkdirAllFn")
	}
	return m.MkdirAllFn(path)
}

func (m *MockFilesystem) RemoveAll(path string) error {
	if m.RemoveAllFn == nil {
		panic("called not defined RemoveAllFn")
	}
	return m.RemoveAllFn(path)
}

func (m *MockFilesystem) MD5Checksum(content []byte) string {
	if m.MD5ChecksumFn == nil {
		panic("called not defined MD5ChecksumFn")
	}
	return m.MD5ChecksumFn(content)
}

func (m *MockFilesystem) Symlink(target, link string) error {
	if m.SymlinkFn == nil {
		panic("called not defined SymlinkFn")
	}
	return m.SymlinkFn(target, link)
}

func (m *MockFilesystem) Readlink(path string) (string, error) {
	if m.ReadlinkFn == nil {
		panic("called not defined ReadlinkFn")
	}
	return m.ReadlinkFn(path)
}

func (m *MockFilesystem) IsSymlink(path string) bool {
	if m.IsSymlinkFn == nil {
		panic("called not defined IsSymlinkFn")
	}
	return m.IsSymlinkFn(path)
}

func (m *MockFilesystem) ListDir(path string) ([]string, error) {
	if m.ListDirFn == nil {
		panic("called not defined ListDirFn")
	}
	return m.ListDirFn(path)
}

