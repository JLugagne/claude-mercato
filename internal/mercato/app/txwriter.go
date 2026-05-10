package app

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	fsrepo "github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/filesystem"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/tx"
)

// txWriter is the narrow filesystem mutation surface used by install and
// remove flows. Concrete writers either route through a tx.Tx (atomic
// install/update/remove) or write through directly (recovery, tooling,
// non-transactional paths).
type txWriter interface {
	WriteFile(path string, content []byte) error
	DeleteFile(path string) error
	DeleteAll(path string) error
}

// directWriter forwards every call straight to the filesystem. Kept as a
// last-resort fallback when no tx manager is wired in.
type directWriter struct{ fs fsrepo.Filesystem }

func (d directWriter) WriteFile(path string, content []byte) error {
	return d.fs.WriteFile(path, content)
}

func (d directWriter) DeleteFile(path string) error {
	return d.fs.DeleteFile(path)
}

func (d directWriter) DeleteAll(path string) error {
	return d.fs.RemoveAll(path)
}

// txWriterAdapter buffers all writes/deletes inside a tx.Tx. The underlying
// transaction is committed by the App layer once the in-memory install
// database has also been staged for writing (via the same tx).
type txWriterAdapter struct{ t tx.Tx }

func (w txWriterAdapter) WriteFile(path string, content []byte) error {
	return w.t.WriteFile(path, content)
}

func (w txWriterAdapter) DeleteFile(path string) error {
	return w.t.DeleteFile(path)
}

func (w txWriterAdapter) DeleteAll(path string) error {
	return w.t.DeleteAll(path)
}

// stageDBSave marshals the install database and stages the write through
// the supplied writer so the DB lands together with the rest of the
// transaction's file changes. Must be the last write before Commit.
func (a *App) stageDBSave(w txWriter, db domain.InstallDatabase) error {
	data, err := a.idb.Marshal(db)
	if err != nil {
		return err
	}
	return w.WriteFile(a.idb.Path(a.cacheDir), data)
}

// beginWriter opens a transactional writer for one install/update/remove
// operation. The returned commit promotes every staged write atomically.
// If no tx manager is configured, a passthrough manager backed by a.fs is
// used (no atomicity, behavior matches pre-tx mct).
func (a *App) beginWriter(op string) (txWriter, func() error, func() error, error) {
	mgr := a.txm
	if mgr == nil {
		mgr = tx.Passthrough(a.fs.WriteFile, a.fs.DeleteFile, a.fs.RemoveAll)
	}
	t, err := mgr.Begin(op)
	if err != nil {
		return nil, nil, nil, err
	}
	return txWriterAdapter{t: t}, t.Commit, t.Rollback, nil
}
