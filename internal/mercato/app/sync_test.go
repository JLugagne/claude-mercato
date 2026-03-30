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

// installedContent is a valid installed entry with mct fields.
const installedContent = "---\nmct_ref: mkt/agents/foo.md\nmct_version: sha123\nmct_market: mkt\nmct_checksum: md5abc\ntype: agent\ndescription: test\n---\n# foo\n"

// newTestApp constructs an App for testing.
func newTestApp(cfg *configstoretest.MockConfigStore, git *gitrepotest.MockGitRepo, fs *filesystemtest.MockFilesystem, state *statestoretest.MockStateStore) *App {
	return New(git, fs, cfg, state, "/config/path", "/cache/dir")
}

// setupInstalledEntry configures mocks so that scanInstalledEntries returns one
// entry for "mkt/agents/foo.md". localFile is the full path key in MapFS.
func setupInstalledEntry(cfg *configstoretest.MockConfigStore, fsMock *filesystemtest.MockFilesystem, localFile string) {
	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			LocalPath: ".claude",
			Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	fsMock.FS = fstest.MapFS{
		localFile: &fstest.MapFile{Data: []byte(installedContent)},
	}
}

// TestCheck_Clean verifies that a matching checksum and version produces StateClean.
func TestCheck_Clean(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	fsMock.MD5ChecksumFn = func(content []byte) string { return "md5abc" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "sha123", nil
	}

	app := newTestApp(cfg, git, fsMock, state)
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

// TestCheck_Drift verifies that a checksum mismatch produces StateDrift.
func TestCheck_Drift(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	fsMock.MD5ChecksumFn = func(content []byte) string { return "DIFFERENT" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "sha123", nil
	}

	app := newTestApp(cfg, git, fsMock, state)
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

// TestCheck_UpdateAvailable verifies that a new version produces StateUpdateAvailable.
func TestCheck_UpdateAvailable(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	fsMock.MD5ChecksumFn = func(content []byte) string { return "md5abc" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "newsha", nil
	}

	app := newTestApp(cfg, git, fsMock, state)
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
	if statuses[0].NewVersion != "newsha" {
		t.Errorf("expected NewVersion=newsha, got %q", statuses[0].NewVersion)
	}
}

// TestCheck_UpdateAndDrift verifies that both drift and version change produces StateUpdateAndDrift.
func TestCheck_UpdateAndDrift(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	fsMock.MD5ChecksumFn = func(content []byte) string { return "DIFFERENT" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "newsha", nil
	}

	app := newTestApp(cfg, git, fsMock, state)
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

// TestCheck_Orphaned verifies that a missing local file produces StateOrphaned.
func TestCheck_Orphaned(t *testing.T) {
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
	// Put the file in MapFS so DirExists(".claude/agents") works and WalkDir finds it.
	fsMock.FS = fstest.MapFS{
		".claude/agents/foo.md": &fstest.MapFile{Data: []byte(installedContent)},
	}
	// Use ReadFileFn with a call counter:
	// - First call (scan): return content so entry is found.
	// - Second call (Check): return error to simulate orphaned.
	callCount := 0
	fsMock.ReadFileFn = func(path string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return []byte(installedContent), nil
		}
		return nil, errors.New("file gone")
	}

	app := newTestApp(cfg, git, fsMock, state)
	statuses, err := app.Check(service.CheckOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].State != domain.StateOrphaned {
		t.Errorf("expected StateOrphaned, got %v", statuses[0].State)
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
		localFile: &fstest.MapFile{Data: []byte(installedContent)},
	}
}

// TestUpdate_Success verifies a successful update writes the file and returns correct result.
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
	// MD5Checksum returns "md5abc" so no drift.
	fsMock.MD5ChecksumFn = func(content []byte) string { return "md5abc" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "newsha", nil
	}
	newContent := []byte("---\ntype: agent\ndescription: updated\n---\n# foo updated\n")
	git.ReadFileAtRefFn = func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
		return newContent, nil
	}
	writeFileCalled := false
	fsMock.WriteFileFn = func(path string, content []byte) error {
		writeFileCalled = true
		return nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Update(service.UpdateOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Action != "update" {
		t.Errorf("expected Action=update, got %q", r.Action)
	}
	if r.OldVersion != "sha123" {
		t.Errorf("expected OldVersion=sha123, got %q", r.OldVersion)
	}
	if r.NewVersion != "newsha" {
		t.Errorf("expected NewVersion=newsha, got %q", r.NewVersion)
	}
	if r.Err != nil {
		t.Errorf("expected no error, got %v", r.Err)
	}
	if !writeFileCalled {
		t.Error("expected WriteFile to be called")
	}
}

// TestUpdate_DryRun verifies that DryRun does not write the file but returns action=update.
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
	fsMock.MD5ChecksumFn = func(content []byte) string { return "md5abc" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "newsha", nil
	}
	writeFileCalled := false
	fsMock.WriteFileFn = func(path string, content []byte) error {
		writeFileCalled = true
		return nil
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
	if writeFileCalled {
		t.Error("WriteFile should NOT be called in dry-run mode")
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

// TestUpdate_Conflict verifies that when the local checksum differs from the
// stored checksum, the result action is "conflict".
func TestUpdate_Conflict(t *testing.T) {
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
	// Return a different checksum than the stored "md5abc" to trigger conflict.
	fsMock.MD5ChecksumFn = func(content []byte) string { return "DRIFTED" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "newsha", nil
	}
	writeFileCalled := false
	fsMock.WriteFileFn = func(path string, content []byte) error {
		writeFileCalled = true
		return nil
	}

	app := newTestApp(cfg, git, fsMock, state)
	results, err := app.Update(service.UpdateOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "conflict" {
		t.Errorf("expected action=conflict, got %q", results[0].Action)
	}
	if writeFileCalled {
		t.Error("WriteFile should NOT be called for a conflict")
	}
}

// TestUpdate_AppliesUpdate verifies that when not in dry-run mode and no drift,
// WriteFile is called and the result action is "update".
func TestUpdate_AppliesUpdate(t *testing.T) {
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
	fsMock.MD5ChecksumFn = func(content []byte) string { return "md5abc" }
	git.FileVersionFn = func(clonePath, filePath string) (domain.MctVersion, error) {
		return "newsha", nil
	}
	newContent := []byte("---\ntype: agent\ndescription: updated\n---\n# updated\n")
	git.ReadFileAtRefFn = func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
		return newContent, nil
	}
	writeFileCalled := false
	fsMock.WriteFileFn = func(path string, content []byte) error {
		writeFileCalled = true
		return nil
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
		t.Errorf("expected action=update, got %q", results[0].Action)
	}
	if !writeFileCalled {
		t.Error("expected WriteFile to be called")
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
