package cfgadapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/installdb"
)

var _ installdb.InstallDB = (*InstallDBAdapter)(nil)

type InstallDBAdapter struct{}

func NewInstallDB() *InstallDBAdapter { return &InstallDBAdapter{} }

func (a *InstallDBAdapter) Load(cacheDir string) (domain.InstallDatabase, error) {
	path := filepath.Join(cacheDir, "installed.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return domain.InstallDatabase{Markets: []domain.InstalledMarket{}}, nil
	}
	if err != nil {
		return domain.InstallDatabase{}, fmt.Errorf("read install database: %w", err)
	}
	var db domain.InstallDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return domain.InstallDatabase{}, fmt.Errorf("parse install database: %w", err)
	}
	if db.Markets == nil {
		db.Markets = []domain.InstalledMarket{}
	}
	return db, nil
}

func (a *InstallDBAdapter) Save(cacheDir string, db domain.InstallDatabase) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, "installed.json")
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal install database: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func (a *InstallDBAdapter) Lock(cacheDir string) error {
	lockPath := filepath.Join(cacheDir, "installed.lock")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	if err := a.tryLock(lockPath); err == nil {
		return nil
	}

	// Lock file exists — check for stale lock.
	if a.removeStaleLock(lockPath) {
		// Stale lock removed, retry once.
		if err := a.tryLock(lockPath); err == nil {
			return nil
		}
	}

	// Poll for up to 5 seconds.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if err := a.tryLock(lockPath); err == nil {
			return nil
		}
	}

	return domain.ErrLockContention
}

func (a *InstallDBAdapter) Unlock(cacheDir string) error {
	lockPath := filepath.Join(cacheDir, "installed.lock")
	err := os.Remove(lockPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (a *InstallDBAdapter) tryLock(lockPath string) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%d", os.Getpid())
	return err
}

// removeStaleLock checks whether the lock file's PID is still running.
// Returns true if the stale lock was removed.
func (a *InstallDBAdapter) removeStaleLock(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	err = syscall.Kill(pid, 0)
	if err == nil {
		// Process exists — lock is not stale.
		return false
	}
	if errors.Is(err, syscall.ESRCH) {
		os.Remove(lockPath)
		return true
	}
	return false
}
