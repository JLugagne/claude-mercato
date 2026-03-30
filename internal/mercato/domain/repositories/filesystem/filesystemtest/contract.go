package filesystemtest

import (
	"io/fs"
	"testing/fstest"
)

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
	TempFileFn       func(name string, content []byte) (string, error)
	RemoveTempFileFn func(path string) error
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
	if m.MD5ChecksumFn != nil {
		return m.MD5ChecksumFn(content)
	}
	return ""
}

func (m *MockFilesystem) TempFile(name string, content []byte) (string, error) {
	if m.TempFileFn != nil {
		return m.TempFileFn(name, content)
	}
	return "", nil
}

func (m *MockFilesystem) RemoveTempFile(path string) error {
	if m.RemoveTempFileFn != nil {
		return m.RemoveTempFileFn(path)
	}
	return nil
}
