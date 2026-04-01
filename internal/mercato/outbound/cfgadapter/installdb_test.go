package cfgadapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

func newInstallDB(t *testing.T) (*InstallDBAdapter, string) {
	t.Helper()
	dir := t.TempDir()
	return NewInstallDB(), dir
}

func TestInstallDB_LoadEmpty(t *testing.T) {
	db, dir := newInstallDB(t)

	got, err := db.Load(dir)
	if err != nil {
		t.Fatalf("Load on empty dir: %v", err)
	}
	if got.Markets == nil {
		t.Error("Markets = nil, want non-nil empty slice")
	}
	if len(got.Markets) != 0 {
		t.Errorf("len(Markets) = %d, want 0", len(got.Markets))
	}
}

func TestInstallDB_SaveAndLoad(t *testing.T) {
	adapter, dir := newInstallDB(t)

	want := domain.InstallDatabase{
		Markets: []domain.InstalledMarket{
			{
				Market: "core",
				Packages: []domain.InstalledPackage{
					{
						Profile: "my-skill",
						Version: "abc123",
						Files: domain.InstalledFiles{
							Skills: []string{"do-thing.md"},
						},
						Locations: []string{"/home/user/project"},
					},
				},
			},
		},
	}

	if err := adapter.Save(dir, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := adapter.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.Markets) != 1 {
		t.Fatalf("len(Markets) = %d, want 1", len(got.Markets))
	}
	m := got.Markets[0]
	if m.Market != "core" {
		t.Errorf("Market = %q, want %q", m.Market, "core")
	}
	if len(m.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(m.Packages))
	}
	pkg := m.Packages[0]
	if pkg.Profile != "my-skill" {
		t.Errorf("Profile = %q, want %q", pkg.Profile, "my-skill")
	}
	if pkg.Version != "abc123" {
		t.Errorf("Version = %q, want %q", pkg.Version, "abc123")
	}
	if len(pkg.Files.Skills) != 1 || pkg.Files.Skills[0] != "do-thing.md" {
		t.Errorf("Files.Skills = %v, want [do-thing.md]", pkg.Files.Skills)
	}
	if len(pkg.Locations) != 1 || pkg.Locations[0] != "/home/user/project" {
		t.Errorf("Locations = %v, want [/home/user/project]", pkg.Locations)
	}
}

func TestInstallDB_SavePermissions(t *testing.T) {
	adapter, dir := newInstallDB(t)

	db := domain.InstallDatabase{Markets: []domain.InstalledMarket{}}
	if err := adapter.Save(dir, db); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "installed.json"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %04o, want 0600", perm)
	}
}

func TestInstallDB_LockAndUnlock(t *testing.T) {
	adapter, dir := newInstallDB(t)

	if err := adapter.Lock(dir); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	lockPath := filepath.Join(dir, "installed.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile lock: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("lock file does not contain a valid PID: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("lock PID = %d, want %d", pid, os.Getpid())
	}

	if err := adapter.Unlock(dir); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("lock file still exists after unlock")
	}
}

func TestInstallDB_LockContention(t *testing.T) {
	adapter, dir := newInstallDB(t)

	if err := adapter.Lock(dir); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	defer adapter.Unlock(dir)

	// Same PID is alive, so a second lock should fail with ErrLockContention.
	err := adapter.Lock(dir)
	if err == nil {
		t.Fatal("expected ErrLockContention, got nil")
	}
	if !errors.Is(err, domain.ErrLockContention) {
		t.Errorf("error = %v, want ErrLockContention", err)
	}
}

func TestInstallDB_StaleLockRecovery(t *testing.T) {
	adapter, dir := newInstallDB(t)

	// Write a lock file with a non-existent PID.
	lockPath := filepath.Join(dir, "installed.lock")
	if err := os.WriteFile(lockPath, fmt.Appendf(nil, "%d", 999999999), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := adapter.Lock(dir); err != nil {
		t.Fatalf("Lock after stale lock: %v", err)
	}
	defer adapter.Unlock(dir)

	// Verify the lock is now held by us.
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid != os.Getpid() {
		t.Errorf("lock PID = %d, want %d", pid, os.Getpid())
	}
}

func TestInstallDB_UnlockIdempotent(t *testing.T) {
	adapter, dir := newInstallDB(t)

	// Unlock without prior lock should not error.
	if err := adapter.Unlock(dir); err != nil {
		t.Fatalf("Unlock without lock: %v", err)
	}
}
