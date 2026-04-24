package installdb

import "github.com/JLugagne/agents-mercato/internal/mercato/domain"

// InstallDB provides locked read/write access to the install database.
type InstallDB interface {
	// Load reads installed.json. If the file doesn't exist, returns an empty database.
	Load(cacheDir string) (domain.InstallDatabase, error)

	// Save writes the database to installed.json with 0600 permissions.
	// Caller MUST hold the lock.
	Save(cacheDir string, db domain.InstallDatabase) error

	// Lock acquires the file lock. Waits up to 5 seconds, then returns ErrLockContention.
	// Detects and removes stale locks (PID no longer running).
	Lock(cacheDir string) error

	// Unlock releases the file lock.
	Unlock(cacheDir string) error
}
