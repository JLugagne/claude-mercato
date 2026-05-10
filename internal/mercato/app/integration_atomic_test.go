package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/tx"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
	"github.com/JLugagne/agents-mercato/internal/mercato/outbound/cfgadapter"
	"github.com/JLugagne/agents-mercato/internal/mercato/outbound/fsadapter"
	"github.com/JLugagne/agents-mercato/internal/mercato/outbound/gitadapter"
	"github.com/JLugagne/agents-mercato/internal/mercato/outbound/txadapter"
)

// setupAtomicIntegration is a copy of setupIntegration that wires a real
// disk-backed tx manager so we can assert atomicity end-to-end.
func setupAtomicIntegration(t *testing.T, repoFiles map[string]string) (*App, string, string) {
	t.Helper()

	sourceDir := createTestRepo(t, repoFiles)
	cacheDir := t.TempDir()
	projectDir := t.TempDir()

	cloneDir := filepath.Join(cacheDir, marketDirName(marketName))
	cloneTestRepo(t, sourceDir, cloneDir)

	configPath := filepath.Join(projectDir, ".claude", ".mct.yaml")
	localPath := filepath.Join(projectDir, ".claude")

	cfgStore := cfgadapter.NewConfigStore()
	cfg := domain.Config{
		LocalPath: localPath,
		Markets: []domain.MarketConfig{
			{Name: marketName, URL: sourceDir, Branch: "main"},
		},
	}
	if err := cfgStore.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	stateStore := cfgadapter.NewStateStore()
	gitRepo := gitadapter.New(gitadapter.WithDepth(0))
	headSHA, err := gitRepo.RemoteHEAD(cloneDir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SetMarketSyncClean(cacheDir, marketName, headSHA); err != nil {
		t.Fatal(err)
	}

	txm := txadapter.New(filepath.Join(cacheDir, "staging"))

	application := New(
		gitRepo,
		fsadapter.New(),
		cfgStore,
		stateStore,
		cfgadapter.NewInstallDB(),
		configPath,
		cacheDir,
		WithTxManager(txm),
	)

	return application, projectDir, cacheDir
}

// failingInstallDB wraps an InstallDB and forces Save to fail. Used to
// simulate "files staged but DB save fails" — the tx must roll back
// everything.
type failingInstallDB struct {
	inner   *cfgadapter.InstallDBAdapter
	saveErr error
}

func (f *failingInstallDB) Load(cacheDir string) (domain.InstallDatabase, error) {
	return f.inner.Load(cacheDir)
}
func (f *failingInstallDB) Save(cacheDir string, db domain.InstallDatabase) error {
	return f.saveErr
}
func (f *failingInstallDB) Marshal(db domain.InstallDatabase) ([]byte, error) {
	return nil, f.saveErr
}
func (f *failingInstallDB) Path(cacheDir string) string  { return f.inner.Path(cacheDir) }
func (f *failingInstallDB) Lock(cacheDir string) error   { return f.inner.Lock(cacheDir) }
func (f *failingInstallDB) Unlock(cacheDir string) error { return f.inner.Unlock(cacheDir) }

func TestAtomic_AddRollsBackOnDBSaveFailure(t *testing.T) {
	application, projectDir, cacheDir := setupAtomicIntegration(t, marketFiles())

	// Swap in a failing install DB after the App is built so the first Add
	// will stage files, then fail at the DB save hook.
	application.idb = &failingInstallDB{
		inner:   cfgadapter.NewInstallDB(),
		saveErr: errors.New("simulated db save failure"),
	}

	ref := domain.MctRef(marketName + "@dev/go/agents/code-review.md")
	_, err := application.Add(ref, service.AddOpts{})
	if err == nil {
		t.Fatal("expected Add to fail")
	}

	// Disk must be untouched.
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "agents", "code-review.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("agent file leaked despite db save failure: %v", err)
	}

	// Staging dir must be cleaned up (rollback ran).
	stagingRoot := filepath.Join(cacheDir, "staging")
	if entries, _ := os.ReadDir(stagingRoot); len(entries) != 0 {
		t.Errorf("staging dir not cleaned, has %d entries", len(entries))
	}
}

// TestAtomic_AddRollsBackOnTxWriteFailure simulates a staging write failure
// (e.g. cache disk full) via a tx manager whose Nth WriteFile fails. The
// in-progress tx must be rolled back and no project files should appear.
func TestAtomic_AddRollsBackOnTxWriteFailure(t *testing.T) {
	files := map[string]string{
		"dev/go/skills/big/SKILL.md": "---\ndescription: \"big\"\n---\nbody\n",
		"dev/go/skills/big/a.md":     "a contents\n",
		"dev/go/skills/big/b.md":     "b contents\n",
		"dev/go/README.md":           "---\ntags: [go]\n---\n",
	}

	sourceDir := createTestRepo(t, files)
	cacheDir := t.TempDir()
	projectDir := t.TempDir()

	cloneDir := filepath.Join(cacheDir, marketDirName(marketName))
	cloneTestRepo(t, sourceDir, cloneDir)

	configPath := filepath.Join(projectDir, ".claude", ".mct.yaml")
	localPath := filepath.Join(projectDir, ".claude")

	cfgStore := cfgadapter.NewConfigStore()
	cfg := domain.Config{
		LocalPath: localPath,
		Markets:   []domain.MarketConfig{{Name: marketName, URL: sourceDir, Branch: "main"}},
	}
	if err := cfgStore.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	stateStore := cfgadapter.NewStateStore()
	gitRepo := gitadapter.New(gitadapter.WithDepth(0))
	headSHA, _ := gitRepo.RemoteHEAD(cloneDir, "main")
	_ = stateStore.SetMarketSyncClean(cacheDir, marketName, headSHA)

	txm := &failingTxManager{
		inner: txadapter.New(filepath.Join(cacheDir, "staging")),
		fail:  2,
	}
	application := New(
		gitRepo,
		fsadapter.New(),
		cfgStore,
		stateStore,
		cfgadapter.NewInstallDB(),
		configPath,
		cacheDir,
		WithTxManager(txm),
	)

	ref := domain.MctRef(marketName + "@dev/go/skills/big/SKILL.md")
	_, err := application.Add(ref, service.AddOpts{})
	if err == nil {
		t.Fatal("expected Add to fail")
	}

	skillDir := filepath.Join(projectDir, ".claude", "skills", "big")
	if _, err := os.Stat(skillDir); !errors.Is(err, os.ErrNotExist) {
		entries, _ := os.ReadDir(skillDir)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("skill dir leaked: %v", names)
	}

	stagingRoot := filepath.Join(cacheDir, "staging")
	if entries, _ := os.ReadDir(stagingRoot); len(entries) != 0 {
		t.Errorf("staging dir not cleaned, has %d entries", len(entries))
	}
}

type failingTxManager struct {
	inner *txadapter.Manager
	fail  int
}

func (m *failingTxManager) Begin(op string) (tx.Tx, error) {
	t, err := m.inner.Begin(op)
	if err != nil {
		return nil, err
	}
	return &failingTx{inner: t, fail: m.fail}, nil
}
func (m *failingTxManager) RecoverPending() error { return m.inner.RecoverPending() }

type failingTx struct {
	inner tx.Tx
	fail  int
	calls int
}

func (t *failingTx) WriteFile(path string, content []byte) error {
	t.calls++
	if t.calls == t.fail {
		return fmt.Errorf("simulated tx write failure")
	}
	return t.inner.WriteFile(path, content)
}
func (t *failingTx) DeleteFile(path string) error { return t.inner.DeleteFile(path) }
func (t *failingTx) DeleteAll(path string) error  { return t.inner.DeleteAll(path) }
func (t *failingTx) Commit() error                { return t.inner.Commit() }
func (t *failingTx) Rollback() error              { return t.inner.Rollback() }
