package tx

// Passthrough returns a Manager whose transactions write straight through to
// the supplied direct-write functions. Commit/Rollback are no-ops at the
// filesystem level. Useful as a fallback when no on-disk staging area is
// available — most notably in unit tests that mock the Filesystem.
//
// IMPORTANT: a passthrough Manager does not provide crash atomicity. The
// production wiring in cmd/mct should always use the disk-backed manager
// from outbound/txadapter.
func Passthrough(write func(path string, content []byte) error, del func(path string) error, delAll func(path string) error) Manager {
	return &passthroughManager{write: write, del: del, delAll: delAll}
}

type passthroughManager struct {
	write  func(path string, content []byte) error
	del    func(path string) error
	delAll func(path string) error
}

func (m *passthroughManager) Begin(op string) (Tx, error) {
	return &passthroughTx{m: m}, nil
}

func (m *passthroughManager) RecoverPending() error { return nil }

type passthroughTx struct {
	m      *passthroughManager
	closed bool
}

func (t *passthroughTx) WriteFile(path string, content []byte) error {
	if t.closed {
		return ErrTxClosed
	}
	return t.m.write(path, content)
}

func (t *passthroughTx) DeleteFile(path string) error {
	if t.closed {
		return ErrTxClosed
	}
	return t.m.del(path)
}

func (t *passthroughTx) DeleteAll(path string) error {
	if t.closed {
		return ErrTxClosed
	}
	return t.m.delAll(path)
}

func (t *passthroughTx) Commit() error {
	if t.closed {
		return ErrTxClosed
	}
	t.closed = true
	return nil
}

func (t *passthroughTx) Rollback() error {
	t.closed = true
	return nil
}

// ErrTxClosed signals an operation on a finalized Tx.
var ErrTxClosed = txClosedError{}

type txClosedError struct{}

func (txClosedError) Error() string { return "tx: already closed" }
