// Package tx defines a per-operation transactional filesystem boundary used
// by install/update/remove flows. Writes and deletes are buffered until
// Commit, which applies them atomically (best-effort across same-fs renames)
// — including the install database, which is staged just like any other
// file. On Rollback or process crash, no on-disk changes are visible and a
// recovery pass on next startup reconciles any leftover staging directories.
package tx

// Tx represents a single in-progress filesystem transaction. All write
// operations are buffered in a per-operation staging area on disk and only
// become visible to other readers once Commit returns nil. After Commit or
// Rollback the Tx is closed and further calls return ErrTxClosed.
type Tx interface {
	// WriteFile records that the given absolute path should hold the
	// supplied content when the transaction commits.
	WriteFile(path string, content []byte) error

	// DeleteFile records that the given absolute path should be removed
	// when the transaction commits.
	DeleteFile(path string) error

	// DeleteAll records that the given absolute directory tree should be
	// removed when the transaction commits.
	DeleteAll(path string) error

	// Commit promotes all staged changes to their final locations.
	// Returns nil only when every change landed.
	Commit() error

	// Rollback discards all staged changes. Safe to call after a failed
	// Commit; safe to defer.
	Rollback() error
}

// Manager opens new transactions and recovers leftover ones from prior runs.
type Manager interface {
	// Begin opens a new transaction. The op string is included in the
	// staging directory name purely for diagnostics.
	Begin(op string) (Tx, error)

	// RecoverPending scans the staging root for transactions left over by
	// a previous (likely crashed) process. Transactions that completed
	// their pre-commit phase are replayed; everything else is discarded.
	RecoverPending() error
}
