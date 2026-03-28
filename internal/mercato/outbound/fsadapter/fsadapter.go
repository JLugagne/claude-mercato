package fsadapter

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (a *Adapter) WriteFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func (a *Adapter) DeleteFile(path string) error {
	return os.Remove(path)
}

func (a *Adapter) FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (a *Adapter) DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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

func (a *Adapter) RemoveTempFile(path string) error {
	return os.Remove(path)
}

func (a *Adapter) ListFiles(dir, suffix string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, suffix) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
