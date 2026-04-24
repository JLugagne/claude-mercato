package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/installdb/installdbtest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

// newTestApp constructs an App for testing.
func newTestApp(cfg *configstoretest.MockConfigStore, git *gitrepotest.MockGitRepo, fs *filesystemtest.MockFilesystem, state *statestoretest.MockStateStore, idbOpts ...*installdbtest.MockInstallDB) *App {
	var idb *installdbtest.MockInstallDB
	if len(idbOpts) > 0 && idbOpts[0] != nil {
		idb = idbOpts[0]
	} else {
		idb = &installdbtest.MockInstallDB{
			LockFn:   func(cacheDir string) error { return nil },
			UnlockFn: func(cacheDir string) error { return nil },
			LoadFn: func(cacheDir string) (domain.InstallDatabase, error) {
				return domain.InstallDatabase{Markets: []domain.InstalledMarket{}}, nil
			},
			SaveFn: func(cacheDir string, db domain.InstallDatabase) error { return nil },
		}
	}
	return New(git, fs, cfg, state, idb, "/config/path", "/cache/dir")
}

// testProjectPath returns the absolute project path that projectPath(".claude")
// resolves to in the test process. Matches what the app code computes.
func testProjectPath() string {
	abs, _ := filepath.Abs(".claude")
	return filepath.Dir(abs)
}

// installedDB returns an installdb with one package at the test project location.
func installedDB(market, profile, version string, files domain.InstalledFiles) domain.InstallDatabase {
	return domain.InstallDatabase{
		Markets: []domain.InstalledMarket{
			{
				Market: market,
				Packages: []domain.InstalledPackage{
					{
						Profile:   profile,
						Version:   version,
						Files:     files,
						Locations: []string{testProjectPath()},
					},
				},
			},
		},
	}
}

// idbWithData returns a MockInstallDB that returns the given database.
func idbWithData(db domain.InstallDatabase) *installdbtest.MockInstallDB {
	return &installdbtest.MockInstallDB{
		LockFn:   func(cacheDir string) error { return nil },
		UnlockFn: func(cacheDir string) error { return nil },
		LoadFn: func(cacheDir string) (domain.InstallDatabase, error) {
			return db, nil
		},
		SaveFn: func(cacheDir string, db domain.InstallDatabase) error { return nil },
	}
}

// ---------------------------------------------------------------------------
// Check
// ---------------------------------------------------------------------------

// TestCheck_Clean verifies that matching versions produce StateClean.
func TestCheck_Clean(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		RemoteHEADFn: func(clonePath, branch string) (string, error) {
			return "abc123", nil // same as installed version
		},
		// ReadFileAtRef returns error to skip drift detection (file not found at ref)
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return nil, errors.New("not found at ref")
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}

	db := installedDB("mkt", "agents/foo.md", "abc123", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{}, idb)
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

// TestCheck_Drift verifies that modified local files produce StateDrift.
func TestCheck_Drift(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		RemoteHEADFn: func(clonePath, branch string) (string, error) {
			return "abc123", nil // same version => no update available
		},
		// ReadFileAtRef will fail => drift detection sees missing local file as drift
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return []byte("original content"), nil
		},
	}
	// MD5 returns different hashes to simulate drift. But since detectDrift
	// uses os.ReadFile for local files, and the files don't actually exist on
	// disk in test, the os.ReadFile will fail and that counts as drift.
	fsMock := &filesystemtest.MockFilesystem{
		MD5ChecksumFn: func(content []byte) string { return "hash" },
	}

	db := installedDB("mkt", "agents/foo.md", "abc123", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{}, idb)
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

// TestCheck_UpdateAvailable verifies that a newer remote HEAD produces StateUpdateAvailable.
func TestCheck_UpdateAvailable(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		RemoteHEADFn: func(clonePath, branch string) (string, error) {
			return "newsha456", nil // different from installed version
		},
		// Return error so drift detection skips all files (no drift)
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return nil, errors.New("not found at ref")
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}

	db := installedDB("mkt", "agents/foo.md", "oldsha123", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{}, idb)
	statuses, err := app.Check(service.CheckOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].State != domain.StateUpdateAvailable {
		t.Errorf("expected StateUpdateAvailable, got %v", statuses[0].State)
	}
}

// TestCheck_UpdateAndDrift verifies StateUpdateAndDrift when both conditions hold.
func TestCheck_UpdateAndDrift(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		RemoteHEADFn: func(clonePath, branch string) (string, error) {
			return "newsha456", nil // different from installed version => update available
		},
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return []byte("original"), nil // drift detection succeeds, but local file differs
		},
	}
	// detectDrift uses os.ReadFile for local files which will fail => drift detected
	fsMock := &filesystemtest.MockFilesystem{
		MD5ChecksumFn: func(content []byte) string { return "hash" },
	}

	db := installedDB("mkt", "agents/foo.md", "oldsha123", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{}, idb)
	statuses, err := app.Check(service.CheckOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].State != domain.StateUpdateAndDrift {
		t.Errorf("expected StateUpdateAndDrift, got %v", statuses[0].State)
	}
}

// TestCheck_MarketFilter verifies that market filtering works.
func TestCheck_MarketFilter(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets: []domain.MarketConfig{
					{Name: "alpha", Branch: "main"},
					{Name: "beta", Branch: "main"},
				},
			}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		RemoteHEADFn: func(clonePath, branch string) (string, error) {
			return "abc123", nil
		},
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return nil, errors.New("skip")
		},
	}

	db := domain.InstallDatabase{
		Markets: []domain.InstalledMarket{
			{Market: "alpha", Packages: []domain.InstalledPackage{
				{Profile: "agents/a.md", Version: "abc123", Files: domain.InstalledFiles{Agents: []string{"a.md"}}, Locations: []string{testProjectPath()}},
			}},
			{Market: "beta", Packages: []domain.InstalledPackage{
				{Profile: "agents/b.md", Version: "abc123", Files: domain.InstalledFiles{Agents: []string{"b.md"}}, Locations: []string{testProjectPath()}},
			}},
		},
	}
	idb := idbWithData(db)

	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{}, idb)
	statuses, err := app.Check(service.CheckOpts{Market: "beta"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status (only beta), got %d", len(statuses))
	}
	if statuses[0].Ref.Market() != "beta" {
		t.Errorf("expected market=beta, got %q", statuses[0].Ref.Market())
	}
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

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
	state.SetMarketSyncDirtyFn = func(cacheDir, market string) error { return nil }
	state.SetMarketSyncCleanFn = func(cacheDir, market, newSHA string) error { return nil }

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

// TestRefresh_UpdatesAvailable verifies that UpdatesAvailable counts packages
// whose version differs from the new SHA.
func TestRefresh_UpdatesAvailable(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{
				Version: 1,
				Markets: map[string]domain.MarketSyncState{
					"mkt": {LastSyncedSHA: "oldhash"},
				},
			}, nil
		},
		SetMarketSyncDirtyFn: func(cacheDir, market string) error { return nil },
		SetMarketSyncCleanFn: func(cacheDir, market, newSHA string) error { return nil },
	}
	git := &gitrepotest.MockGitRepo{
		FetchFn: func(clonePath, branch string) (string, error) { return "newhash", nil },
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return nil, nil
		},
	}

	db := domain.InstallDatabase{
		Markets: []domain.InstalledMarket{
			{Market: "mkt", Packages: []domain.InstalledPackage{
				{Profile: "agents/a.md", Version: "oldhash", Files: domain.InstalledFiles{Agents: []string{"a.md"}}, Locations: []string{testProjectPath()}},
				{Profile: "agents/b.md", Version: "newhash", Files: domain.InstalledFiles{Agents: []string{"b.md"}}, Locations: []string{testProjectPath()}},
			}},
		},
	}
	idb := idbWithData(db)

	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state, idb)
	results, err := app.Refresh(service.RefreshOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].UpdatesAvailable != 1 {
		t.Errorf("expected UpdatesAvailable=1, got %d", results[0].UpdatesAvailable)
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

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// TestUpdate_Success verifies that an installed entry with upstream changes
// reports action=update.
func TestUpdate_Success(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
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

	git := &gitrepotest.MockGitRepo{
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return []domain.FileDiff{
				{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
			}, nil
		},
		// ReadFileAtRef for drift detection: return error to skip drift
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			if commitSHA == "HEAD" {
				return []byte("new content"), nil
			}
			return nil, errors.New("not found at ref")
		},
		RemoteHEADFn: func(clonePath, branch string) (string, error) {
			return "newhash", nil
		},
	}

	fsMock := &filesystemtest.MockFilesystem{
		WriteFileFn:  func(path string, content []byte) error { return nil },
		DeleteFileFn: func(path string) error { return nil },
	}

	db := installedDB("mkt", "agents/foo.md", "oldhash", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, state, idb)
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
	if results[0].NewVersion != "newhash" {
		t.Errorf("expected NewVersion=newhash, got %q", results[0].NewVersion)
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

	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
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
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
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
	git := &gitrepotest.MockGitRepo{
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return []domain.FileDiff{
				{Action: domain.DiffDelete, From: "agents/foo.md", To: ""},
			}, nil
		},
	}

	db := installedDB("mkt", "agents/foo.md", "oldhash", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state, idb)
	results, err := app.Update(service.UpdateOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for deleted entry, got %d", len(results))
	}
}

// TestUpdate_DriftAllKeep verifies that drift + AllKeep produces action=kept.
func TestUpdate_DriftAllKeep(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
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
	git := &gitrepotest.MockGitRepo{
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return []domain.FileDiff{
				{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
			}, nil
		},
		// Return content so drift detection runs; os.ReadFile will fail => drift
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return []byte("original"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		MD5ChecksumFn: func(content []byte) string { return "hash" },
	}

	db := installedDB("mkt", "agents/foo.md", "oldhash", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, state, idb)
	results, err := app.Update(service.UpdateOpts{AllKeep: true})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "kept" {
		t.Errorf("expected Action=kept, got %q", results[0].Action)
	}
	if len(results[0].DriftFiles) == 0 {
		t.Error("expected DriftFiles to be populated")
	}
}

// TestUpdate_DriftAllMerge verifies that drift + AllMerge overwrites and reports update.
func TestUpdate_DriftAllMerge(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
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
	git := &gitrepotest.MockGitRepo{
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return []domain.FileDiff{
				{Action: domain.DiffModify, From: "agents/foo.md", To: "agents/foo.md"},
			}, nil
		},
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			if commitSHA == "HEAD" {
				return []byte("new content"), nil
			}
			return []byte("original"), nil
		},
		RemoteHEADFn: func(clonePath, branch string) (string, error) {
			return "newhash", nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		MD5ChecksumFn: func(content []byte) string { return "hash" },
		WriteFileFn:   func(path string, content []byte) error { return nil },
		DeleteFileFn:  func(path string) error { return nil },
	}

	db := installedDB("mkt", "agents/foo.md", "oldhash", domain.InstalledFiles{Agents: []string{"foo.md"}})
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, state, idb)
	results, err := app.Update(service.UpdateOpts{AllMerge: true})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "update" {
		t.Errorf("expected Action=update, got %q", results[0].Action)
	}
	if results[0].NewVersion != "newhash" {
		t.Errorf("expected NewVersion=newhash, got %q", results[0].NewVersion)
	}
}

// ---------------------------------------------------------------------------
// Refresh error paths
// ---------------------------------------------------------------------------

// TestRefresh_ConfigLoadError verifies that a config load error is returned.
func TestRefresh_ConfigLoadError(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, errors.New("config broken")
		},
	}
	app := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})
	_, err := app.Refresh(service.RefreshOpts{})
	if err == nil {
		t.Fatal("expected error from config load, got nil")
	}
	if err.Error() != "config broken" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRefresh_LoadSyncStateError verifies that a LoadSyncState error is returned.
func TestRefresh_LoadSyncStateError(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{}, errors.New("state corrupted")
		},
	}
	app := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, state)
	_, err := app.Refresh(service.RefreshOpts{})
	if err == nil {
		t.Fatal("expected error from LoadSyncState, got nil")
	}
	if err.Error() != "state corrupted" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRefresh_FetchError verifies that a Fetch error is recorded in the result.
func TestRefresh_FetchError(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{Version: 1, Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			}}, nil
		},
		SetMarketSyncDirtyFn: func(cacheDir string, market string) error {
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		FetchFn: func(clonePath, branch string) (string, error) {
			return "", errors.New("network down")
		},
	}
	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
	results, err := app.Refresh(service.RefreshOpts{})
	if err != nil {
		t.Fatal("unexpected top-level error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected error in result, got nil")
	}
	if results[0].Err.Error() != "network down" {
		t.Errorf("unexpected error: %v", results[0].Err)
	}
	if results[0].OldSHA != "oldhash" {
		t.Errorf("expected OldSHA=oldhash, got %q", results[0].OldSHA)
	}
}

// TestRefresh_SetMarketSyncDirtyError verifies that a SetMarketSyncDirty error
// is recorded in the result when DryRun=false.
func TestRefresh_SetMarketSyncDirtyError(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{Version: 1, Markets: map[string]domain.MarketSyncState{}}, nil
		},
		SetMarketSyncDirtyFn: func(cacheDir, market string) error {
			return errors.New("dirty failed")
		},
	}
	app := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, state)
	results, err := app.Refresh(service.RefreshOpts{DryRun: false})
	if err != nil {
		t.Fatal("unexpected top-level error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil || results[0].Err.Error() != "dirty failed" {
		t.Errorf("expected 'dirty failed' error, got %v", results[0].Err)
	}
}

// TestRefresh_SetMarketSyncCleanError verifies that a SetMarketSyncClean error
// is recorded in the result when DryRun=false.
func TestRefresh_SetMarketSyncCleanError(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{Version: 1, Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			}}, nil
		},
		SetMarketSyncDirtyFn: func(cacheDir, market string) error { return nil },
		SetMarketSyncCleanFn: func(cacheDir, market, newSHA string) error {
			return errors.New("clean failed")
		},
	}
	git := &gitrepotest.MockGitRepo{
		FetchFn: func(clonePath, branch string) (string, error) { return "newhash", nil },
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return nil, nil
		},
	}
	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
	results, err := app.Refresh(service.RefreshOpts{DryRun: false})
	if err != nil {
		t.Fatal("unexpected top-level error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil || results[0].Err.Error() != "clean failed" {
		t.Errorf("expected 'clean failed' error, got %v", results[0].Err)
	}
	if results[0].NewSHA != "newhash" {
		t.Errorf("expected NewSHA=newhash, got %q", results[0].NewSHA)
	}
}

// TestRefresh_NewMarketSkipsDiff verifies that when oldSHA is empty (new market,
// no previous sync), the diff step is skipped and ChangedFiles=0.
func TestRefresh_NewMarketSkipsDiff(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{Version: 1, Markets: map[string]domain.MarketSyncState{}}, nil
		},
	}
	diffCalled := false
	git := &gitrepotest.MockGitRepo{
		FetchFn: func(clonePath, branch string) (string, error) { return "newhash", nil },
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			diffCalled = true
			return nil, nil
		},
	}
	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
	results, err := app.Refresh(service.RefreshOpts{DryRun: true})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if diffCalled {
		t.Error("DiffSinceCommit should NOT be called when oldSHA is empty")
	}
	if results[0].ChangedFiles != 0 {
		t.Errorf("expected ChangedFiles=0, got %d", results[0].ChangedFiles)
	}
	if results[0].OldSHA != "" {
		t.Errorf("expected OldSHA=\"\", got %q", results[0].OldSHA)
	}
	if results[0].NewSHA != "newhash" {
		t.Errorf("expected NewSHA=newhash, got %q", results[0].NewSHA)
	}
}

// TestRefresh_DiffSinceCommitError verifies that a DiffSinceCommit error is
// silently handled (continues with 0 changed files).
func TestRefresh_DiffSinceCommitError(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{Version: 1, Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			}}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		FetchFn: func(clonePath, branch string) (string, error) { return "newhash", nil },
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return nil, errors.New("diff broken")
		},
	}
	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
	results, err := app.Refresh(service.RefreshOpts{DryRun: true})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("expected no error in result (diff error is silent), got %v", results[0].Err)
	}
	if results[0].ChangedFiles != 0 {
		t.Errorf("expected ChangedFiles=0 after diff error, got %d", results[0].ChangedFiles)
	}
}

// TestRefresh_SkillsOnlyFiltering verifies that SkillsOnly markets filter
// non-skill paths from the changed files count.
func TestRefresh_SkillsOnlyFiltering(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{
				{Name: "mkt", Branch: "main", SkillsOnly: true, SkillsPath: "skills"},
			}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{Version: 1, Markets: map[string]domain.MarketSyncState{
				"mkt": {LastSyncedSHA: "oldhash"},
			}}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		FetchFn: func(clonePath, branch string) (string, error) { return "newhash", nil },
		DiffSinceCommitFn: func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
			return []domain.FileDiff{
				{Action: domain.DiffModify, From: "skills/foo.md", To: "skills/foo.md"},
				{Action: domain.DiffModify, From: "agents/bar.md", To: "agents/bar.md"},
				{Action: domain.DiffModify, From: "README.md", To: "README.md"},
			}, nil
		},
	}
	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
	results, err := app.Refresh(service.RefreshOpts{DryRun: true})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ChangedFiles != 1 {
		t.Errorf("expected ChangedFiles=1 (only skill path), got %d", results[0].ChangedFiles)
	}
}

// TestRefresh_MarketNameFiltering verifies that opts.Market filters markets by name.
func TestRefresh_MarketNameFiltering(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{Markets: []domain.MarketConfig{
				{Name: "alpha", Branch: "main"},
				{Name: "beta", Branch: "main"},
			}}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{Version: 1, Markets: map[string]domain.MarketSyncState{}}, nil
		},
	}
	fetchedMarkets := []string{}
	git := &gitrepotest.MockGitRepo{
		FetchFn: func(clonePath, branch string) (string, error) {
			fetchedMarkets = append(fetchedMarkets, clonePath)
			return "newhash", nil
		},
	}
	app := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
	results, err := app.Refresh(service.RefreshOpts{DryRun: true, Market: "beta"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only beta), got %d", len(results))
	}
	if results[0].Market != "beta" {
		t.Errorf("expected Market=beta, got %q", results[0].Market)
	}
}

// ---------------------------------------------------------------------------
// Sync
// ---------------------------------------------------------------------------

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
	state.SetMarketSyncDirtyFn = func(cacheDir, market string) error { return nil }
	state.SetMarketSyncCleanFn = func(cacheDir, market, newSHA string) error { return nil }

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
