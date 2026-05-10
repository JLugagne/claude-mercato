// Package txadapter implements the tx.Manager port using a per-operation
// staging directory under <stagingRoot>/<txID>/ on the local filesystem.
//
// Layout for an in-progress transaction:
//
//	<stagingRoot>/<txID>/
//	  manifest.json   ← list of staged writes, deletes, and OnCommit hooks
//	  state           ← "open" → "committing" → "done"
//	  files/<n>       ← staged content for the n-th write
//
// Commit protocol (recoverable):
//  1. Write manifest.json + transition state from "open" to "committing".
//  2. Promote each staged file to its final path via os.Rename when
//     possible, falling back to copy+remove on cross-filesystem moves.
//  3. Apply DeleteFile entries.
//  4. Run OnCommit callbacks (the install database save lives here).
//  5. Transition state to "done" and remove the staging directory.
//
// Recovery on next startup: any "committing" tx replays steps 2–5; any
// "open" tx is discarded.
package txadapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/tx"
)

const (
	stateOpen       = "open"
	stateCommitting = "committing"
	stateDone       = "done"

	manifestName = "manifest.json"
	stateName    = "state"
	filesDir     = "files"
)

// Manager is the disk-backed implementation of tx.Manager.
type Manager struct {
	root string
}

var _ tx.Manager = (*Manager)(nil)

// New returns a Manager whose staging directories live under stagingRoot.
// The directory is created on demand.
func New(stagingRoot string) *Manager {
	return &Manager{root: stagingRoot}
}

// Begin opens a new transaction. The op string is included in the staging
// directory name purely for diagnostics.
func (m *Manager) Begin(op string) (tx.Tx, error) {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return nil, fmt.Errorf("tx: create staging root: %w", err)
	}
	id := newTxID(op)
	dir := filepath.Join(m.root, id)
	if err := os.MkdirAll(filepath.Join(dir, filesDir), 0o755); err != nil {
		return nil, fmt.Errorf("tx: create staging dir: %w", err)
	}
	t := &diskTx{dir: dir, id: id}
	if err := t.writeState(stateOpen); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return t, nil
}

// RecoverPending reconciles any leftover staging directories. Called at
// startup before the application performs any new writes.
func (m *Manager) RecoverPending() error {
	entries, err := os.ReadDir(m.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("tx: list staging root: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(m.root, e.Name())
		if err := recoverOne(dir); err != nil {
			// Best-effort: log via stderr, keep going. A dangling staging
			// dir should never block normal operation.
			fmt.Fprintf(os.Stderr, "mct: tx recovery failed for %s: %v\n", dir, err)
		}
	}
	return nil
}

func recoverOne(dir string) error {
	st, err := os.ReadFile(filepath.Join(dir, stateName))
	if err != nil {
		// No state file at all — discard.
		return os.RemoveAll(dir)
	}
	switch string(st) {
	case stateCommitting:
		// Mid-commit crash: replay file promotions + deletes. We do NOT
		// rerun OnCommit hooks during recovery — they live in the
		// application process and the install database update is handled
		// by mct's normal startup paths.
		man, err := readManifest(dir)
		if err != nil {
			return err
		}
		if err := applyWrites(dir, man.Writes); err != nil {
			return err
		}
		if err := applyDeletes(man.Deletes); err != nil {
			return err
		}
		if err := applyDeleteAlls(man.DeleteAlls); err != nil {
			return err
		}
		return os.RemoveAll(dir)
	default:
		// stateOpen or anything unknown: discard.
		return os.RemoveAll(dir)
	}
}

// diskTx is one in-progress transaction.
type diskTx struct {
	mu         sync.Mutex
	dir        string
	id         string
	writes     []writeEntry
	deletes    []string
	deleteAlls []string
	closed     bool
}

type writeEntry struct {
	Final string `json:"final"` // absolute final destination
	Slot  string `json:"slot"`  // basename under files/
}

type manifest struct {
	Writes     []writeEntry `json:"writes"`
	Deletes    []string     `json:"deletes"`
	DeleteAlls []string     `json:"delete_alls,omitempty"`
}

func (t *diskTx) WriteFile(path string, content []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return tx.ErrTxClosed
	}
	slot := strconv.Itoa(len(t.writes))
	full := filepath.Join(t.dir, filesDir, slot)
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return fmt.Errorf("tx: stage write %s: %w", path, err)
	}
	t.writes = append(t.writes, writeEntry{Final: path, Slot: slot})
	return nil
}

func (t *diskTx) DeleteFile(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return tx.ErrTxClosed
	}
	t.deletes = append(t.deletes, path)
	return nil
}

func (t *diskTx) DeleteAll(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return tx.ErrTxClosed
	}
	t.deleteAlls = append(t.deleteAlls, path)
	return nil
}

func (t *diskTx) Commit() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return tx.ErrTxClosed
	}
	t.closed = true
	writes := t.writes
	deletes := t.deletes
	deleteAlls := t.deleteAlls
	t.mu.Unlock()

	man := manifest{Writes: writes, Deletes: deletes, DeleteAlls: deleteAlls}
	if err := writeManifest(t.dir, man); err != nil {
		return err
	}
	if err := t.writeState(stateCommitting); err != nil {
		return err
	}
	if err := applyWrites(t.dir, writes); err != nil {
		return err
	}
	if err := applyDeletes(deletes); err != nil {
		return err
	}
	if err := applyDeleteAlls(deleteAlls); err != nil {
		return err
	}
	if err := t.writeState(stateDone); err != nil {
		return err
	}
	return os.RemoveAll(t.dir)
}

func (t *diskTx) Rollback() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()
	return os.RemoveAll(t.dir)
}

func (t *diskTx) writeState(s string) error {
	tmp := filepath.Join(t.dir, stateName+".tmp")
	if err := os.WriteFile(tmp, []byte(s), 0o644); err != nil {
		return fmt.Errorf("tx: write state: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(t.dir, stateName)); err != nil {
		return fmt.Errorf("tx: rename state: %w", err)
	}
	return nil
}

func writeManifest(dir string, m manifest) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("tx: marshal manifest: %w", err)
	}
	tmp := filepath.Join(dir, manifestName+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("tx: write manifest: %w", err)
	}
	return os.Rename(tmp, filepath.Join(dir, manifestName))
}

func readManifest(dir string) (manifest, error) {
	var m manifest
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return m, fmt.Errorf("tx: read manifest: %w", err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("tx: parse manifest: %w", err)
	}
	return m, nil
}

func applyWrites(dir string, writes []writeEntry) error {
	for _, w := range writes {
		src := filepath.Join(dir, filesDir, w.Slot)
		// Skip writes whose staged file is already gone — recovery may
		// re-enter after partial progress.
		if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(w.Final), 0o755); err != nil {
			return fmt.Errorf("tx: mkdir %s: %w", filepath.Dir(w.Final), err)
		}
		if err := os.Rename(src, w.Final); err != nil {
			// Cross-filesystem rename fails with EXDEV: copy + remove.
			if isCrossDevice(err) {
				if cerr := copyAndRemove(src, w.Final); cerr != nil {
					return fmt.Errorf("tx: cross-fs promote %s: %w", w.Final, cerr)
				}
				continue
			}
			return fmt.Errorf("tx: promote %s: %w", w.Final, err)
		}
	}
	return nil
}

func applyDeletes(paths []string) error {
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("tx: delete %s: %w", p, err)
		}
	}
	return nil
}

func applyDeleteAlls(paths []string) error {
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("tx: delete-all %s: %w", p, err)
		}
	}
	return nil
}

func copyAndRemove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".mct-tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Remove(src)
}

func newTxID(op string) string {
	return fmt.Sprintf("%d-%d-%s", time.Now().UnixNano(), os.Getpid(), sanitize(op))
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && i < 32; i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "op"
	}
	return string(out)
}
