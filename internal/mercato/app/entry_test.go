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
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// agentFile returns minimal valid agent frontmatter content (no mct fields).
func agentFile() []byte {
	return []byte("---\ntype: agent\ndescription: test agent\nauthor: alice\n---\n# Agent\nDo stuff.\n")
}

// installedAgentFile returns agent frontmatter with mct fields set (simulates installed file).
func installedAgentFile(ref, version string) []byte {
	return []byte("---\nmct_ref: " + ref + "\nmct_version: " + version + "\ntype: agent\ndescription: test\n---\n# content\n")
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
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	mapFS := fstest.MapFS{
		".claude/agents/foo.md": {Data: installedAgentFile("mkt/agents/foo.md", "v1")},
		".claude/agents/bar.md": {Data: installedAgentFile("mkt/agents/bar.md", "v2")},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: mapFS,
		// Only file1 "exists" on disk for the FileExists check.
		// Delegate to MapFS for directory paths; return not-found for file2.
		StatFn: func(name string) (fs.FileInfo, error) {
			if name == file2 {
				return nil, errors.New("not found")
			}
			return fs.Stat(mapFS, name)
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	// Without installed filter: both entries are returned
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
	if installed[0].Ref != "mkt/agents/foo.md" {
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
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			filePath: {Data: installedAgentFile("mkt/agents/foo.md", "sha1")},
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	entry, err := a.GetEntry("mkt/agents/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Market != "mkt" {
		t.Errorf("expected Market=mkt, got %q", entry.Market)
	}
	if entry.Ref != "mkt/agents/foo.md" {
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

	_, err := a.GetEntry("mkt/agents/foo.md")
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
	writeFileCalled := false
	addEntryCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
		AddEntryFn: func(path string, entry domain.EntryConfig) error {
			addEntryCalled = true
			return nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		MD5ChecksumFn: func(content []byte) string { return "md5abc" },
		WriteFileFn: func(path string, content []byte) error {
			writeFileCalled = true
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return agentFile(), nil
		},
		FileVersionFn: func(clonePath, filePath string) (domain.MctVersion, error) {
			return "sha123", nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt/agents/foo.md", service.AddOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !writeFileCalled {
		t.Error("expected WriteFile to be called")
	}
	if !addEntryCalled {
		t.Error("expected AddEntry to be called")
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
			filePath: {Data: installedAgentFile("mkt/agents/foo.md", "sha1")},
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt/agents/foo.md", service.AddOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "ENTRY_ALREADY_INSTALLED") {
		t.Errorf("expected ENTRY_ALREADY_INSTALLED, got %v", err)
	}
}

func TestAdd_DryRun(t *testing.T) {
	writeFileCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		WriteFileFn: func(path string, content []byte) error {
			writeFileCalled = true
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return agentFile(), nil
		},
		FileVersionFn: func(clonePath, filePath string) (domain.MctVersion, error) {
			return "sha123", nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt/agents/foo.md", service.AddOpts{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeFileCalled {
		t.Error("expected WriteFile NOT to be called in dry-run mode")
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

	err := a.Add("unknown/agents/foo.md", service.AddOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "MARKET_NOT_FOUND") {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

func TestAdd_MctFieldsInRepo(t *testing.T) {
	contentWithMctFields := []byte("---\nmct_ref: mkt/agents/foo.md\ntype: agent\ndescription: test\n---\n# content\n")

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			return contentWithMctFields, nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt/agents/foo.md", service.AddOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "MCT_FIELDS_IN_REPO") {
		t.Errorf("expected MCT_FIELDS_IN_REPO, got %v", err)
	}
}

func TestAdd_WithDependency(t *testing.T) {
	agentContent := []byte("---\ntype: agent\ndescription: test agent\nauthor: alice\nrequires_skills:\n  - file: skills/dep.md\n---\n# Agent\nDo stuff.\n")
	skillContent := []byte("---\ntype: skill\ndescription: a dep skill\nauthor: alice\n---\n# Skill\n")

	readFileAtRefCalls := 0
	addManagedSkillCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return cfgWithMarket("mkt", "https://example.com", "main", ".claude"), nil
		},
		AddEntryFn: func(path string, entry domain.EntryConfig) error { return nil },
		AddManagedSkillFn: func(path string, skill domain.ManagedSkillConfig) error {
			addManagedSkillCalled = true
			return nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		MD5ChecksumFn: func(content []byte) string { return "md5" },
		WriteFileFn:   func(path string, content []byte) error { return nil },
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
		FileVersionFn: func(clonePath, filePath string) (domain.MctVersion, error) {
			return "sha1", nil
		},
	}

	a := newTestApp(cfg, git, fsMock, &statestoretest.MockStateStore{})

	err := a.Add("mkt/agents/foo.md", service.AddOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readFileAtRefCalls < 2 {
		t.Errorf("expected ReadFileAtRef to be called at least twice (agent + skill), got %d", readFileAtRefCalls)
	}
	if !addManagedSkillCalled {
		t.Error("expected AddManagedSkill to be called for dependency")
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove_Success(t *testing.T) {
	filePath := ".claude/agents/foo.md"
	deleteFileCalled := false
	removeEntryCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
		RemoveEntryFn: func(path string, ref domain.MctRef) error {
			removeEntryCalled = true
			return nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			filePath: {Data: installedAgentFile("mkt/agents/foo.md", "sha1")},
		},
		DeleteFileFn: func(path string) error {
			deleteFileCalled = true
			return nil
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Remove("mkt/agents/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteFileCalled {
		t.Error("expected DeleteFile to be called")
	}
	if !removeEntryCalled {
		t.Error("expected RemoveEntry to be called")
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

	err := a.Remove("mkt/agents/foo.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "ENTRY_NOT_INSTALLED") {
		t.Errorf("expected ENTRY_NOT_INSTALLED, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pin
// ---------------------------------------------------------------------------

func TestPin_Success(t *testing.T) {
	filePath := ".claude/agents/foo.md"
	setEntryPinCalled := false
	var gotRef domain.MctRef
	var gotPin string

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
		SetEntryPinFn: func(path string, ref domain.MctRef, pin string) error {
			setEntryPinCalled = true
			gotRef = ref
			gotPin = pin
			return nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			filePath: {Data: installedAgentFile("mkt/agents/foo.md", "sha1")},
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Pin("mkt/agents/foo.md", "deadbeef")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !setEntryPinCalled {
		t.Error("expected SetEntryPin to be called")
	}
	if gotRef != "mkt/agents/foo.md" {
		t.Errorf("expected ref mkt/agents/foo.md, got %q", gotRef)
	}
	if gotPin != "deadbeef" {
		t.Errorf("expected pin deadbeef, got %q", gotPin)
	}
}

func TestPin_NotInstalled(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{LocalPath: ".claude"}, nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	err := a.Pin("mkt/agents/foo.md", "deadbeef")
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
	removeEntryCalled := false
	cfg.RemoveEntryFn = func(path string, ref domain.MctRef) error {
		removeEntryCalled = true
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
	if !removeEntryCalled {
		t.Error("expected RemoveEntry to be called")
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

	_, _, _, err := a.PrepareDiff("mkt/agents/foo.md")
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
	leftTmpPath, rightPath, _, err := a.PrepareDiff("mkt/agents/foo.md")
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
	if len(savedCfg.Entries) != 0 {
		t.Errorf("expected no entries, got %d", len(savedCfg.Entries))
	}
}

func TestInit_WithManagedFiles(t *testing.T) {
	// Content has mct_ref frontmatter to simulate an already-managed file.
	managedContent := []byte("---\nmct_ref: mkt/agents/foo.md\nmct_version: sha1\ntype: agent\ndescription: test\n---\n# foo\n")

	var savedCfg domain.Config
	cfg := &configstoretest.MockConfigStore{
		SaveFn: func(path string, c domain.Config) error {
			savedCfg = c
			return nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			"agents/foo.md": &fstest.MapFile{Data: managedContent},
		},
		MkdirAllFn: func(path string) error { return nil },
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})
	err := a.Init(service.InitOpts{LocalPath: "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(savedCfg.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(savedCfg.Entries))
	}
	if savedCfg.Entries[0].Ref != "mkt/agents/foo.md" {
		t.Errorf("expected ref mkt/agents/foo.md, got %q", savedCfg.Entries[0].Ref)
	}
}

// ---------------------------------------------------------------------------
// validateEntryType
// ---------------------------------------------------------------------------

func TestValidateEntryType(t *testing.T) {
	cases := []struct {
		entryType domain.EntryType
		relPath   string
		wantErr   bool
	}{
		{domain.EntryTypeAgent, "dev/go/agents/foo.md", false},
		{domain.EntryTypeSkill, "dev/go/skills/bar.md", false},
		{domain.EntryTypeSkill, "dev/go/agents/foo.md", true},
		{domain.EntryTypeAgent, "dev/go/skills/bar.md", true},
		{domain.EntryTypeAgent, "dev/go/README.md", false},
	}
	for _, tc := range cases {
		t.Run(string(tc.entryType)+"/"+tc.relPath, func(t *testing.T) {
			err := validateEntryType(tc.entryType, tc.relPath)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for type=%q path=%q", tc.entryType, tc.relPath)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for type=%q path=%q: %v", tc.entryType, tc.relPath, err)
			}
		})
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

	t.Run("skill path", func(t *testing.T) {
		got, err := a.resolveLocalPath(cfg, "dev/go/skills/bar.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(".claude", "skills", "bar", "SKILL.md")
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
