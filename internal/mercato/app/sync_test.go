package app

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// newTestApp constructs an App for testing.
func newTestApp(cfg *configstoretest.MockConfigStore, git *gitrepotest.MockGitRepo, fs *filesystemtest.MockFilesystem, state *statestoretest.MockStateStore) *App {
	return New(git, fs, cfg, state, "/config/path", "/cache/dir")
}

// setupInstalledEntry configures mocks so that scanInstalledEntries returns one
// entry for "mkt@agents/foo.md". localFile is the full path key in MapFS.
func setupInstalledEntry(cfg *configstoretest.MockConfigStore, fsMock *filesystemtest.MockFilesystem, localFile string) {
	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			LocalPath: ".claude",
			Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	fsMock.FS = fstest.MapFS{
		localFile: &fstest.MapFile{Data: []byte("---\ndescription: test\n---\n")},
	}
	setupSymlinkMock(fsMock, map[string]string{
		localFile: "/cache/dir/mkt/agents/foo.md",
	})
}

// setupSymlinkMock configures IsSymlinkFn and ReadlinkFn on a MockFilesystem
// to simulate symlinks. The map keys are local file paths and the values are
// the symlink targets (absolute paths inside the cache directory).
func setupSymlinkMock(fsMock *filesystemtest.MockFilesystem, symlinks map[string]string) {
	fsMock.IsSymlinkFn = func(path string) bool {
		_, ok := symlinks[path]
		return ok
	}
	fsMock.ReadlinkFn = func(path string) (string, error) {
		target, ok := symlinks[path]
		if !ok {
			return "", errors.New("not a symlink")
		}
		return target, nil
	}
}

// TestCheck_Clean verifies that a valid symlink produces StateClean.
func TestCheck_Clean(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	app := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, state)
	statuses, err := app.Check(service.CheckOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].State != domain.StateClean {
		t.Errorf("expected StateClean, got %v", statuses[0].State)
	}
}

// TestCheck_Drift verifies that a non-symlink file produces StateDrift.
func TestCheck_Drift(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			".claude/agents/foo.md": &fstest.MapFile{Data: []byte("---\ndescription: test\n---\n")},
		},
	}
	// File exists in walker but IsSymlink returns true for scan, then false for check.
	// To simulate: scan finds symlink, but Check sees it's no longer a symlink.
	// Use a counter: first IsSymlink call (scan) returns true, second (check) returns false.
	callCount := 0
	fsMock.IsSymlinkFn = func(path string) bool {
		callCount++
		return callCount <= 1 // true on first call (scan), false on second (check)
	}
	fsMock.ReadlinkFn = func(path string) (string, error) {
		return "/cache/dir/mkt/agents/foo.md", nil
	}

	app := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})
	statuses, err := app.Check(service.CheckOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].State != domain.StateDrift {
		t.Errorf("expected StateDrift, got %v", statuses[0].State)
	}
}

// TestRefresh_Success verifies that a successful fetch produces a complete result.
func TestRefresh_Success(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return domain.SyncState{
			Version: 1,
			Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			},
		}, nil
	}
	dirtyCalledWith := ""
	state.SetMarketSyncDirtyFn = func(cacheDir, market string) error {
		dirtyCalledWith = market
		return nil
	}
	cleanCalledWith := ""
	state.SetMarketSyncCleanFn = func(cacheDir, market, newSHA string) error {
		cleanCalledWith = market
		return nil
	}
	git.FetchFn = func(clonePath, branch string) (string, error) {
		return "newhash", nil
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return []domain.FileDiff{
			{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
			{Action: domain.DiffModify, From: "agents/bar.md", To: "agents/bar.md"},
		}, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Refresh(service.RefreshOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Market != "mkt" {
		t.Errorf("expected Market=mkt, got %q", r.Market)
	}
	if r.OldSHA != "oldhash" {
		t.Errorf("expected OldSHA=oldhash, got %q", r.OldSHA)
	}
	if r.NewSHA != "newhash" {
		t.Errorf("expected NewSHA=newhash, got %q", r.NewSHA)
	}
	if r.ChangedFiles != 2 {
		t.Errorf("expected ChangedFiles=2, got %d", r.ChangedFiles)
	}
	if r.Err != nil {
		t.Errorf("expected no error, got %v", r.Err)
	}
	if dirtyCalledWith != "mkt" {
		t.Errorf("SetMarketSyncDirty not called with mkt, got %q", dirtyCalledWith)
	}
	if cleanCalledWith != "mkt" {
		t.Errorf("SetMarketSyncClean not called with mkt, got %q", cleanCalledWith)
	}
}

// TestRefresh_UpToDate verifies that when SHA doesn't change, ChangedFiles=0.
func TestRefresh_UpToDate(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return domain.SyncState{
			Version: 1,
			Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "samehash"},
			},
		}, nil
	}
	git.FetchFn = func(clonePath, branch string) (string, error) {
		return "samehash", nil
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return nil, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Refresh(service.RefreshOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ChangedFiles != 0 {
		t.Errorf("expected ChangedFiles=0, got %d", results[0].ChangedFiles)
	}
}

// TestRefresh_DryRun verifies that DryRun skips state mutations but still fetches.
func TestRefresh_DryRun(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return domain.SyncState{
			Version: 1,
			Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			},
		}, nil
	}
	dirtyCalled := false
	state.SetMarketSyncDirtyFn = func(cacheDir, market string) error {
		dirtyCalled = true
		return nil
	}
	cleanCalled := false
	state.SetMarketSyncCleanFn = func(cacheDir, market, newSHA string) error {
		cleanCalled = true
		return nil
	}
	fetchCalled := false
	git.FetchFn = func(clonePath, branch string) (string, error) {
		fetchCalled = true
		return "newhash", nil
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return nil, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	_, err := app.Refresh(service.RefreshOpts{DryRun: true})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if !fetchCalled {
		t.Error("expected Fetch to be called in dry-run mode")
	}
	if dirtyCalled {
		t.Error("SetMarketSyncDirty should NOT be called in dry-run mode")
	}
	if cleanCalled {
		t.Error("SetMarketSyncClean should NOT be called in dry-run mode")
	}
}

// TestSyncState verifies that SyncState() returns what the state store returns.
func TestSyncState(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	want := domain.SyncState{
		Version: 1,
		Markets: map[string]domain.MarketSyncState{
			"mkt": {LastSyncedSHA: "abc123"},
		},
	}
	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return want, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	got, err := app.SyncState()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if got.Version != want.Version {
		t.Errorf("expected Version=%d, got %d", want.Version, got.Version)
	}
	if ms, ok := got.Markets["mkt"]; !ok || ms.LastSyncedSHA != "abc123" {
		t.Errorf("unexpected markets state: %+v", got.Markets)
	}
}

// setupInstalledEntryForUpdate sets up mocks for Update tests using a specific localFile path.
func setupInstalledEntryForUpdate(cfg *configstoretest.MockConfigStore, fsMock *filesystemtest.MockFilesystem, localFile string) {
	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			LocalPath: ".claude",
			Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	fsMock.FS = fstest.MapFS{
		localFile: &fstest.MapFile{Data: []byte("---\ndescription: test\n---\n")},
	}
	setupSymlinkMock(fsMock, map[string]string{
		localFile: "/cache/dir/mkt/agents/foo.md",
	})
}

// TestUpdate_Success verifies that an installed entry with upstream changes reports action=update.
func TestUpdate_Success(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntryForUpdate(cfg, fsMock, ".claude/agents/foo.md")

	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return domain.SyncState{
			Version: 1,
			Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			},
		}, nil
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return []domain.FileDiff{
			{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
		}, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Update(service.UpdateOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "update" {
		t.Errorf("expected Action=update, got %q", results[0].Action)
	}
}

// TestUpdate_DryRun verifies that DryRun still reports changes (symlinks auto-update).
func TestUpdate_DryRun(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntryForUpdate(cfg, fsMock, ".claude/agents/foo.md")

	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return domain.SyncState{
			Version: 1,
			Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			},
		}, nil
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return []domain.FileDiff{
			{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
		}, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Update(service.UpdateOpts{DryRun: true})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "update" {
		t.Errorf("expected Action=update, got %q", results[0].Action)
	}
}

// TestUpdate_NothingInstalled verifies that Update returns empty results when
// there are no installed entries that match the diff.
func TestUpdate_NothingInstalled(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{
				Version: 1,
				Markets: map[string]domain.MarketSyncState{
					"mkt": {LastSyncedSHA: "oldhash"},
				},
			}, nil
		},
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return []domain.FileDiff{
			{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
		}, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Update(service.UpdateOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results with no installed entries, got %d", len(results))
	}
}

// TestUpdate_DeletedEntrySkipped verifies that deleted diffs are skipped.
func TestUpdate_DeletedEntrySkipped(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntryForUpdate(cfg, fsMock, ".claude/agents/foo.md")

	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return domain.SyncState{
			Version: 1,
			Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			},
		}, nil
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return []domain.FileDiff{
			{Action: domain.DiffDelete, From: "agents/foo.md", To: ""},
		}, nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Update(service.UpdateOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for deleted entry, got %d", len(results))
	}
}

// TestSync_CallsRefreshAndUpdate verifies that Sync calls Refresh then Update.
func TestSync_CallsRefreshAndUpdate(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			LocalPath: ".claude",
			Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	state.LoadSyncStateFn = func(cacheDir string) (domain.SyncState, error) {
		return domain.SyncState{
			Version: 1,
			Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			},
		}, nil
	}
	git.FetchFn = func(clonePath, branch string) (string, error) {
		return "newhash", nil
	}
	git.DiffSinceCommitFn = func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
		return []domain.FileDiff{
			{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
		}, nil
	}
	// No installed entries — empty MapFS.

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Sync(service.SyncOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 SyncResult, got %d", len(results))
	}
	r := results[0]
	if r.Refresh.Market != "mkt" {
		t.Errorf("expected Refresh.Market=mkt, got %q", r.Refresh.Market)
	}
	if r.Refresh.NewSHA != "newhash" {
		t.Errorf("expected Refresh.NewSHA=newhash, got %q", r.Refresh.NewSHA)
	}
}
