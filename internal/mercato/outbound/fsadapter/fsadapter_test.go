package fsadapter

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	fsrepo "github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem"
)

func newAdapter(t *testing.T) *Adapter {
	t.Helper()
	return New()
}

func TestWriteAndReadFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	a := NewAt(dir)
	path := "hello.txt"
	content := []byte("hello world")

	if err := a.WriteFile(filepath.Join(dir, path), content); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := a.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestStat_FileExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	a := NewAt(dir)
	if err := a.WriteFile(filepath.Join(dir, "exists.txt"), []byte("data")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if !fsrepo.FileExists(a, "exists.txt") {
		t.Errorf("FileExists(%q) = false, want true", "exists.txt")
	}

	if fsrepo.FileExists(a, "nonexistent.txt") {
		t.Error("FileExists on nonexistent path = true, want false")
	}
}

func TestDeleteFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	a := NewAt(dir)
	if err := a.WriteFile(filepath.Join(dir, "todelete.txt"), []byte("bye")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := a.DeleteFile(filepath.Join(dir, "todelete.txt")); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	if fsrepo.FileExists(a, "todelete.txt") {
		t.Error("FileExists after delete = true, want false")
	}

	if err := a.DeleteFile(filepath.Join(dir, "nonexistent.txt")); err != nil {
		t.Errorf("DeleteFile on nonexistent path: expected nil, got %v", err)
	}
}

func TestStat_DirExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	// The adapter is rooted at dir's parent, and we check dir's base name.
	parent := filepath.Dir(dir)
	base := filepath.Base(dir)
	a := NewAt(parent)

	if !fsrepo.DirExists(a, base) {
		t.Errorf("DirExists(%q) = false, want true", base)
	}

	if fsrepo.DirExists(a, "nonexistent") {
		t.Error("DirExists on nonexistent path = true, want false")
	}

	// Write a file in dir, verify it's not a dir.
	if err := a.WriteFile(filepath.Join(parent, base, "afile.txt"), []byte("x")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if fsrepo.DirExists(a, filepath.Join(base, "afile.txt")) {
		t.Error("DirExists on a file path = true, want false")
	}
}

func TestMkdirAll(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	a := newAdapter(t)
	nested := filepath.Join(dir, "a", "b", "c")

	if err := a.MkdirAll(nested); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	info, err := os.Stat(nested)
	if err != nil || !info.IsDir() {
		t.Errorf("expected %q to be a directory after MkdirAll", nested)
	}

	// MkdirAll on existing dir should not error
	if err := a.MkdirAll(nested); err != nil {
		t.Errorf("MkdirAll on existing dir: %v", err)
	}
}

func TestRemoveAll(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	a := newAdapter(t)
	target := filepath.Join(dir, "target")
	if err := a.MkdirAll(target); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := a.WriteFile(filepath.Join(target, "file.txt"), []byte("data")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := a.RemoveAll(target); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("directory still exists after RemoveAll")
	}
}

func TestMD5Checksum(t *testing.T) {
	a := newAdapter(t)

	got := a.MD5Checksum([]byte("hello"))
	want := "5d41402abc4b2a76b9719d911017c592"
	if got != want {
		t.Errorf("MD5Checksum(\"hello\") = %q, want %q", got, want)
	}

	// Same content → same checksum
	if a.MD5Checksum([]byte("hello")) != got {
		t.Error("MD5Checksum not deterministic for same input")
	}

	// Different content → different checksum
	if a.MD5Checksum([]byte("world")) == got {
		t.Error("MD5Checksum collision for different inputs")
	}
}

func TestReadDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	a := NewAt(dir)

	for _, name := range []string{"foo.md", "bar.md", "baz.txt"} {
		if err := a.WriteFile(filepath.Join(dir, name), []byte("content")); err != nil {
			t.Fatalf("WriteFile(%q): %v", name, err)
		}
	}

	// Use WalkDir to find .md files (equivalent to old ListFiles).
	var mdFiles []string
	err = fs.WalkDir(a, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	if len(mdFiles) != 2 {
		t.Errorf("WalkDir(.md) returned %d files, want 2: %v", len(mdFiles), mdFiles)
	}
	sort.Strings(mdFiles)
	want := []string{"bar.md", "foo.md"}
	for i, f := range mdFiles {
		if f != want[i] {
			t.Errorf("mdFiles[%d] = %q, want %q", i, f, want[i])
		}
	}
}

func TestWriteFile_CreatesParentDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "fsadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	a := NewAt(dir)
	nestedAbs := filepath.Join(dir, "deep", "nested", "path", "file.txt")

	if err := a.WriteFile(nestedAbs, []byte("hello")); err != nil {
		t.Fatalf("WriteFile to nested path: %v", err)
	}

	if !fsrepo.FileExists(a, filepath.Join("deep", "nested", "path", "file.txt")) {
		t.Errorf("FileExists = false after WriteFile with parent creation")
	}
}
