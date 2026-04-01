package app

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// agentFile returns minimal valid agent frontmatter content (no mct fields).
func agentFile() []byte {
	return []byte("---\ndescription: test agent\nauthor: alice\n---\n# Agent\nDo stuff.\n")
}

// cfgWithMarket returns a config with one market and a given local path.
func cfgWithMarket(name, url, branch, localPath string) domain.Config {
	return domain.Config{
		LocalPath: localPath,
		Markets: []domain.MarketConfig{
			{Name: name, URL: url, Branch: branch},
		},
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_Empty(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	entries, err := a.List(service.ListOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestList_InstalledFilter(t *testing.T) {
	const file1 = ".claude/agents/foo.md"
	const file2 = ".claude/agents/bar.md"

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	mapFS := fstest.MapFS{
		".claude/agents/foo.md": {Data: []byte("---\ndescription: test\n---\n")},
		".claude/agents/bar.md": {Data: []byte("---\ndescription: test\n---\n")},
	}
	// Simulate symlinks: both files are symlinks pointing into the cache
	symlinks := map[string]string{
		file1: "/cache/dir/mkt/agents/foo.md",
		file2: "/cache/dir/mkt/agents/bar.md",
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: mapFS,
		IsSymlinkFn: func(path string) bool {
			_, ok := symlinks[path]
			return ok
		},
		ReadlinkFn: func(path string) (string, error) {
			target, ok := symlinks[path]
			if !ok {
				return "", errors.New("not a symlink")
			}
			return target, nil
		},
		StatFn: func(name string) (fs.FileInfo, error) {
			if name == file2 {
				return nil, errors.New("not found")
			}
			return fs.Stat(mapFS, name)
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	// Without installed filter: both symlinked entries are returned
	entries, err := a.List(service.ListOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries without filter, got %d", len(entries))
	}

	// With installed filter: only file1 (which Stat returns info for)
	installed, err := a.List(service.ListOpts{Installed: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(installed) != 1 {
		t.Errorf("expected 1 installed entry, got %d", len(installed))
	}
	if installed[0].Ref != "mkt@agents/foo.md" {
		t.Errorf("unexpected ref: %v", installed[0].Ref)
	}
}

// ---------------------------------------------------------------------------
// GetEntry
// ---------------------------------------------------------------------------

func TestGetEntry_Found(t *testing.T) {
	filePath := ".claude/agents/foo.md"

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			filePath: {Data: []byte("---\ndescription: test\n---\n")},
		},
	}
	setupSymlinkMock(fsMock, map[string]string{
		filePath: "/cache/dir/mkt/agents/foo.md",
	})

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	entry, err := a.GetEntry("mkt@agents/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Market != "mkt" {
		t.Errorf("expected Market=mkt, got %q", entry.Market)
	}
	if entry.Ref != "mkt@agents/foo.md" {
		t.Errorf("unexpected Ref: %v", entry.Ref)
	}
	if entry.Type != domain.EntryTypeAgent {
		t.Errorf("expected type agent, got %q", entry.Type)
	}
}

func TestGetEntry_NotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	_, err := a.GetEntry("mkt@agents/foo.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "ENTRY_NOT_FOUND") {
		t.Errorf("expected ENTRY_NOT_FOUND, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Add
// ---------------------------------------------------------------------------

func TestAdd_Success(t *testing.T) {
	symlinkCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		SymlinkFn: func(target, link string) error {
			symlinkCalled = true
			expectedTarget := filepath.Join("/cache/dir/mkt", "agents/foo.md")
			if target != expectedTarget {
				t.Errorf("expected symlink target %q, got %q", expectedTarget, target)
			}
			if link != ".claude/agents/foo.md" {
				t.Errorf("expected symlink link %q, got %q", ".claude/agents/foo.md", link)
			}
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return agentFile(), nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@agents/foo.md", service.AddOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !symlinkCalled {
		t.Error("expected Symlink to be called")
	}
}

func TestAdd_AlreadyInstalled(t *testing.T) {
	filePath := ".claude/agents/foo.md"

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			filePath: {Data: []byte("---\ndescription: test\n---\n")},
		},
	}
	setupSymlinkMock(fsMock, map[string]string{
		filePath: "/cache/dir/mkt/agents/foo.md",
	})

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@agents/foo.md", service.AddOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "ENTRY_ALREADY_INSTALLED") {
		t.Errorf("expected ENTRY_ALREADY_INSTALLED, got %v", err)
	}
}

func TestAdd_DryRun(t *testing.T) {
	symlinkCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		SymlinkFn: func(target, link string) error {
			symlinkCalled = true
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return agentFile(), nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@agents/foo.md", service.AddOpts{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if symlinkCalled {
		t.Error("expected Symlink NOT to be called in dry-run mode")
	}
}

func TestAdd_MarketNotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("unknown@agents/foo.md", service.AddOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "MARKET_NOT_FOUND") {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

func TestAdd_MctFieldsInRepo(t *testing.T) {
	// mct_* fields in repo files are no longer rejected (symlink-based install).
	// This test verifies that Add succeeds even if the repo file has mct_* fields.
	contentWithMctFields := []byte("---\nmct_ref: mkt@agents/foo.md\ndescription: test\n---\n# content\n")

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		SymlinkFn: func(target, link string) error { return nil },
	}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return contentWithMctFields, nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@agents/foo.md", service.AddOpts{})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestAdd_WithDependency(t *testing.T) {
	agentContent := []byte("---\ndescription: test agent\nauthor: alice\nrequires_skills:\n  - file: skills/dep.md\n---\n# Agent\nDo stuff.\n")
	skillContent := []byte("---\ndescription: a dep skill\nauthor: alice\n---\n# Skill\n")

	readFileAtRefCalls := 0

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		SymlinkFn: func(target, link string) error { return nil },
	}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			readFileAtRefCalls++
			if filePath == "agents/foo.md" {
				return agentContent, nil
			}
			if filePath == "skills/dep.md" {
				return skillContent, nil
			}
			return nil, errors.New("unexpected file: " + filePath)
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@agents/foo.md", service.AddOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readFileAtRefCalls < 2 {
		t.Errorf("expected ReadFileAtRef to be called at least twice (agent + skill), got %d", readFileAtRefCalls)
	}
}

// ---------------------------------------------------------------------------
// Add — profile-level refs
// ---------------------------------------------------------------------------

func skillFile() []byte {
	return []byte("---\ndescription: test skill\nauthor: alice\n---\n# Skill\nDo stuff.\n")
}

func TestAdd_ProfileExpand_Success(t *testing.T) {
	symlinkPaths := []string{}

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		SymlinkFn: func(target, link string) error {
			symlinkPaths = append(symlinkPaths, link)
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ReadMarketFilesFn: func(clonePath, branch string) ([]gitrepo.MarketFile, error) {
			return []gitrepo.MarketFile{
				{Path: "dev/go/agents/foo.md", Content: agentFile()},
				{Path: "dev/go/skills/bar.md", Content: skillFile()},
			}, nil
		},
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			if filePath == "dev/go/agents/foo.md" {
				return agentFile(), nil
			}
			if filePath == "dev/go/skills/bar.md" {
				return skillFile(), nil
			}
			return nil, errors.New("unexpected file: " + filePath)
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@dev/go", service.AddOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symlinkPaths) != 2 {
		t.Errorf("expected Symlink called twice, got %d", len(symlinkPaths))
	}
}

func TestAdd_ProfileExpand_DryRun(t *testing.T) {
	symlinkCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		SymlinkFn: func(target, link string) error {
			symlinkCalled = true
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ReadMarketFilesFn: func(clonePath, branch string) ([]gitrepo.MarketFile, error) {
			return []gitrepo.MarketFile{
				{Path: "dev/go/agents/foo.md", Content: agentFile()},
			}, nil
		},
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return agentFile(), nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@dev/go", service.AddOpts{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if symlinkCalled {
		t.Error("expected Symlink NOT to be called in dry-run mode")
	}
}

func TestAdd_ProfileExpand_AllAlreadyInstalled(t *testing.T) {
	const agentLocalPath = ".claude/agents/foo.md"

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			agentLocalPath: {Data: []byte("---\ndescription: test\n---\n")},
		},
	}
	setupSymlinkMock(fsMock, map[string]string{
		agentLocalPath: "/cache/dir/mkt/dev/go/agents/foo.md",
	})
	git := &gitrepotest.MockGitRepo{
		ReadMarketFilesFn: func(clonePath, branch string) ([]gitrepo.MarketFile, error) {
			return []gitrepo.MarketFile{
				{Path: "dev/go/agents/foo.md"},
			}, nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@dev/go", service.AddOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "ENTRY_ALREADY_INSTALLED") {
		t.Errorf("expected ENTRY_ALREADY_INSTALLED, got %v", err)
	}
}

func TestAdd_ProfileExpand_PartialInstall(t *testing.T) {
	const agentLocalPath = ".claude/agents/foo.md"
	symlinkPaths := []string{}

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			// foo.md already installed
			agentLocalPath: {Data: []byte("---\ndescription: test\n---\n")},
		},
		SymlinkFn: func(target, link string) error {
			symlinkPaths = append(symlinkPaths, link)
			return nil
		},
	}
	setupSymlinkMock(fsMock, map[string]string{
		agentLocalPath: "/cache/dir/mkt/dev/go/agents/foo.md",
	})
	git := &gitrepotest.MockGitRepo{
		ReadMarketFilesFn: func(clonePath, branch string) ([]gitrepo.MarketFile, error) {
			return []gitrepo.MarketFile{
				{Path: "dev/go/agents/foo.md"},   // already installed
				{Path: "dev/go/skills/bar.md"},   // not installed
			}, nil
		},
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return skillFile(), nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt@dev/go", service.AddOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symlinkPaths) != 1 {
		t.Errorf("expected Symlink called once (only bar.md), got %d", len(symlinkPaths))
	}
}

func TestAdd_ProfileExpand_MarketNotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	err := a.Add("unknown@dev/go", service.AddOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "MARKET_NOT_FOUND") {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove_Success(t *testing.T) {
	filePath := ".claude/agents/foo.md"
	deleteFileCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			filePath: {Data: []byte("---\ndescription: test\n---\n")},
		},
		DeleteFileFn: func(path string) error {
			deleteFileCalled = true
			return nil
		},
	}
	setupSymlinkMock(fsMock, map[string]string{
		filePath: "/cache/dir/mkt/agents/foo.md",
	})

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Remove("mkt@agents/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteFileCalled {
		t.Error("expected DeleteFile to be called")
	}
}

func TestRemove_NotInstalled(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Remove("mkt@agents/foo.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "ENTRY_NOT_INSTALLED") {
		t.Errorf("expected ENTRY_NOT_INSTALLED, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReadEntryContent
// ---------------------------------------------------------------------------

func TestReadEntryContent_Success(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
	want := []byte("# agent content\n")
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return want, nil
		},
	}
	a := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	got, err := a.ReadEntryContent("mkt", "agents/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestReadEntryContent_MarketNotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	_, err := a.ReadEntryContent("missing", "agents/foo.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "MARKET_NOT_FOUND") {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Prune
// ---------------------------------------------------------------------------

func TestPrune_AllKeep(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	fsMock := &filesystemtest.MockFilesystem{}
	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	git := &gitrepotest.MockGitRepo{
		FileVersionFn: func(clonePath, filePath string) (domain.MctVersion, error) {
			return "", errors.New("file gone from registry")
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})
	results, err := a.Prune(service.PruneOpts{AllKeep: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "kept" {
		t.Errorf("expected action=kept, got %q", results[0].Action)
	}
}

func TestPrune_AllRemove(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	fsMock := &filesystemtest.MockFilesystem{}
	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	git := &gitrepotest.MockGitRepo{
		FileVersionFn: func(clonePath, filePath string) (domain.MctVersion, error) {
			return "", errors.New("file gone from registry")
		},
	}

	deleteFileCalled := false
	fsMock.DeleteFileFn = func(path string) error {
		deleteFileCalled = true
		return nil
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})
	results, err := a.Prune(service.PruneOpts{AllRemove: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "removed" {
		t.Errorf("expected action=removed, got %q", results[0].Action)
	}
	if !deleteFileCalled {
		t.Error("expected DeleteFile to be called")
	}
}

// ---------------------------------------------------------------------------
// PrepareDiff
// ---------------------------------------------------------------------------

func TestPrepareDiff_EntryNotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	_, _, _, err := a.PrepareDiff("mkt@agents/foo.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "ENTRY_NOT_FOUND") {
		t.Errorf("expected ENTRY_NOT_FOUND, got %v", err)
	}
}

func TestPrepareDiff_Success(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	fsMock := &filesystemtest.MockFilesystem{}
	setupInstalledEntry(cfg, fsMock, ".claude/agents/foo.md")

	registryContent := []byte("# registry content\n")
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return registryContent, nil
		},
	}

	tmpPath := "/tmp/foo.md.12345"
	fsMock.TempFileFn = func(name string, content []byte) (string, error) {
		return tmpPath, nil
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})
	leftTmpPath, rightPath, _, err := a.PrepareDiff("mkt@agents/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if leftTmpPath != tmpPath {
		t.Errorf("expected leftTmpPath=%q, got %q", tmpPath, leftTmpPath)
	}
	wantRightPath := ".claude/agents/foo.md"
	if rightPath != wantRightPath {
		t.Errorf("expected rightPath=%q, got %q", wantRightPath, rightPath)
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestInit_EmptyDir(t *testing.T) {
	saveCalled := false
	var savedCfg domain.Config

	cfg := &configstoretest.MockConfigStore{
		SaveFn: func(path string, c domain.Config) error {
			saveCalled = true
			savedCfg = c
			return nil
		},
	}
	mkdirCalled := false
	fsMock := &filesystemtest.MockFilesystem{
		MkdirAllFn: func(path string) error {
			mkdirCalled = true
			return nil
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})
	err := a.Init(service.InitOpts{LocalPath: ".claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mkdirCalled {
		t.Error("expected MkdirAll to be called")
	}
	if !saveCalled {
		t.Error("expected Save to be called")
	}
	if savedCfg.LocalPath != ".claude" {
		t.Errorf("expected LocalPath=.claude, got %q", savedCfg.LocalPath)
	}
}

// ---------------------------------------------------------------------------
// inferEntryType
// ---------------------------------------------------------------------------

func TestInferEntryType(t *testing.T) {
	cases := []struct {
		relPath  string
		expected domain.EntryType
	}{
		{"dev/go/agents/foo.md", domain.EntryTypeAgent},
		{"dev/go/skills/bar.md", domain.EntryTypeSkill},
		{"dev/go/README.md", ""},
		{"agents/foo.md", domain.EntryTypeAgent},
		{"skills/bar.md", domain.EntryTypeSkill},
	}
	for _, tc := range cases {
		t.Run(tc.relPath, func(t *testing.T) {
			got := inferEntryType(tc.relPath)
			if got != tc.expected {
				t.Errorf("inferEntryType(%q) = %q, want %q", tc.relPath, got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolveLocalPath
// ---------------------------------------------------------------------------

func TestResolveLocalPath(t *testing.T) {
	a := newTestApp(&configstoretest.MockConfigStore{}, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})
	cfg := domain.Config{LocalPath: ".claude"}

	t.Run("agent path", func(t *testing.T) {
		got, err := a.resolveLocalPath(cfg, "dev/go/agents/foo.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(".claude", "agents", "foo.md")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("skill flat file", func(t *testing.T) {
		got, err := a.resolveLocalPath(cfg, "dev/go/skills/bar.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(".claude", "skills", "bar")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("skill directory path", func(t *testing.T) {
		got, err := a.resolveLocalPath(cfg, "dev/go-hexagonal/skills/go-architect/SKILL.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(".claude", "skills", "go-architect")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("path traversal", func(t *testing.T) {
		_, err := a.resolveLocalPath(cfg, "../etc/passwd")
		if err == nil {
			t.Fatal("expected error for path traversal, got nil")
		}
		if !isDomainErrorWithCode(err, "UNSAFE_PATH") {
			t.Errorf("expected UNSAFE_PATH error, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// isSkillDirRef
// ---------------------------------------------------------------------------

func TestIsSkillDirRef(t *testing.T) {
	cases := []struct {
		relPath  string
		expected bool
	}{
		{"skills/azure-ai", true},
		{"skills/go-architect", true},
		{"skills/azure-ai/SKILL.md", false},
		{"skills/bar.md", false},
		{"dev/go/skills/bar", true},
		{"dev/go/skills/bar/SKILL.md", false},
		{"agents/foo.md", false},
		{"agents/foo", false},     // agents dir, not skill
		{"dev/go-hexagonal", false}, // profile, no skills segment
	}
	for _, tc := range cases {
		t.Run(tc.relPath, func(t *testing.T) {
			got := isSkillDirRef(tc.relPath)
			if got != tc.expected {
				t.Errorf("isSkillDirRef(%q) = %v, want %v", tc.relPath, got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Add — skill directory ref normalization
// ---------------------------------------------------------------------------

func TestAdd_SkillDirRef(t *testing.T) {
	symlinkCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		SymlinkFn: func(target, link string) error {
			symlinkCalled = true
			// Skill symlinks point to the directory, not the SKILL.md file
			expectedTarget := filepath.Dir(filepath.Join("/cache/dir/mkt", "skills/go-arch/SKILL.md"))
			if target != expectedTarget {
				t.Errorf("expected symlink target %q, got %q", expectedTarget, target)
			}
			if link != ".claude/skills/go-arch" {
				t.Errorf("expected symlink link %q, got %q", ".claude/skills/go-arch", link)
			}
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			if filePath != "skills/go-arch/SKILL.md" {
				t.Errorf("expected ReadFileAtRef for skills/go-arch/SKILL.md, got %q", filePath)
			}
			return skillFile(), nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	// Pass skill directory ref without /SKILL.md — should be normalized
	err := a.Add("mkt@skills/go-arch", service.AddOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !symlinkCalled {
		t.Error("expected Symlink to be called")
	}
}

// ---------------------------------------------------------------------------
// Remove — skill directory ref normalization
// ---------------------------------------------------------------------------

func TestRemove_SkillDirRef(t *testing.T) {
	skillLocalPath := ".claude/skills/go-arch"
	deleteFileCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			".claude/skills/go-arch/SKILL.md": {Data: []byte("---\ndescription: test\n---\n")},
		},
		DeleteFileFn: func(path string) error {
			deleteFileCalled = true
			if path != skillLocalPath {
				t.Errorf("expected DeleteFile on symlink %q, got %q", skillLocalPath, path)
			}
			return nil
		},
	}
	// Simulate skill directory symlink: .claude/skills/go-arch → cache/mkt/skills/go-arch
	setupSymlinkMock(fsMock, map[string]string{
		skillLocalPath: "/cache/dir/mkt/skills/go-arch",
	})
	// ListDir returns the skill directory name for scanning
	fsMock.ListDirFn = func(dir string) ([]string, error) {
		if dir == ".claude/skills" {
			return []string{"go-arch"}, nil
		}
		return nil, nil
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	// Pass skill directory ref without /SKILL.md — should be normalized
	err := a.Remove("mkt@skills/go-arch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteFileCalled {
		t.Error("expected DeleteFile to be called")
	}
}
