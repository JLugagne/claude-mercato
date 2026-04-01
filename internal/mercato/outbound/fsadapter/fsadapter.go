package fsadapter

import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Adapter implements filesystem.Filesystem.
// Read operations delegate to an fs.FS (os.DirFS by default).
// Write operations use os directly.
type Adapter struct {
	fsys fs.FS
}

// New returns an Adapter rooted at the process working directory.
func New() *Adapter {
	return &Adapter{fsys: os.DirFS(".")}
}

// NewAt returns an Adapter rooted at the given directory.
func NewAt(root string) *Adapter {
	return &Adapter{fsys: os.DirFS(root)}
}

// --- ReadFS methods ---

func (a *Adapter) Open(name string) (fs.File, error) {
	return a.fsys.Open(name)
}

func (a *Adapter) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(a.fsys, name)
}

func (a *Adapter) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(a.fsys, name)
}

func (a *Adapter) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(a.fsys, name)
}

// --- Write methods ---

func (a *Adapter) WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func (a *Adapter) DeleteFile(path string) error {
	return os.Remove(path)
}

func (a *Adapter) MkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func (a *Adapter) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (a *Adapter) MD5Checksum(content []byte) string {
	hash := md5.Sum(content)
	return fmt.Sprintf("%x", hash)
}

func (a *Adapter) Symlink(target, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}
	return os.Symlink(target, link)
}

func (a *Adapter) Readlink(path string) (string, error) {
	return os.Readlink(path)
}

func (a *Adapter) IsSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

func (a *Adapter) ListDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

func (a *Adapter) TempFile(name string, content []byte) (string, error) {
	f, err := os.CreateTemp("", name)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}
