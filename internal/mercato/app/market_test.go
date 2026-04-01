package app

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// fakeDir implements fs.FileInfo for a directory.
type fakeDir struct{}

func (fakeDir) Name() string       { return "dir" }
func (fakeDir) Size() int64        { return 0 }
func (fakeDir) Mode() fs.FileMode  { return fs.ModeDir | 0755 }
func (fakeDir) ModTime() time.Time { return time.Time{} }
func (fakeDir) IsDir() bool        { return true }
func (fakeDir) Sys() any           { return nil }

// ---------------------------------------------------------------------------
// marketNameFromURL
// ---------------------------------------------------------------------------

func TestMarketNameFromURL(t *testing.T) {
	cases := []struct {
		url      string
		wantName string
		wantErr  bool
	}{
		{"https://github.com/org/repo", "org/repo", false},
		{"https://github.com/org/repo.git", "org/repo", false},
		{"git@github.com:org/repo.git", "org/repo", false},
		{"git@gitlab.com:aa/bb/cc.git", "aa/bb/cc", false},
		{"https://example.com", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			name, err := marketNameFromURL(tc.url)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tc.url)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.url, err)
				return
			}
			if name != tc.wantName {
				t.Errorf("marketNameFromURL(%q) = %q, want %q", tc.url, name, tc.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ListMarkets
// ---------------------------------------------------------------------------

func TestListMarkets_Empty(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	markets, err := a.ListMarkets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(markets) != 0 {
		t.Errorf("expected empty slice, got %d markets", len(markets))
	}
}

func TestListMarkets_WithMarkets(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{
					{Name: "alpha", URL: "https://example.com/alpha", Branch: "main"},
					{Name: "beta", URL: "https://example.com/beta", Branch: "develop"},
				},
			}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	markets, err := a.ListMarkets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(markets) != 2 {
		t.Fatalf("expected 2 markets, got %d", len(markets))
	}

	for _, m := range markets {
		expectedClone := filepath.Join("/cache/dir", m.Name)
		if m.ClonePath != expectedClone {
			t.Errorf("market %q: expected ClonePath %q, got %q", m.Name, expectedClone, m.ClonePath)
		}
	}

	names := map[string]bool{}
	for _, m := range markets {
		names[m.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected markets alpha and beta, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// GetMarket
// ---------------------------------------------------------------------------

func TestGetMarket_Found(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{
					{Name: "foo", URL: "https://example.com/foo", Branch: "main"},
				},
			}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	m, err := a.GetMarket("foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "foo" {
		t.Errorf("expected Name=foo, got %q", m.Name)
	}
	if m.URL != "https://example.com/foo" {
		t.Errorf("expected URL, got %q", m.URL)
	}
	if m.Branch != "main" {
		t.Errorf("expected Branch=main, got %q", m.Branch)
	}
	expectedClone := filepath.Join("/cache/dir", "foo")
	if m.ClonePath != expectedClone {
		t.Errorf("expected ClonePath %q, got %q", expectedClone, m.ClonePath)
	}
}

func TestGetMarket_NotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{
					{Name: "other", URL: "https://example.com", Branch: "main"},
				},
			}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	_, err := a.GetMarket("foo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != "MARKET_NOT_FOUND" {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AddMarket
// ---------------------------------------------------------------------------

func TestAddMarket_Success(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	// Empty MockFilesystem: DirExists returns false (no entry in MapFS).
	fsMock := &filesystemtest.MockFilesystem{}
	git := &gitrepotest.MockGitRepo{
		ValidateRemoteFn: func(url string) error { return nil },
		CloneFn:          func(url, clonePath string) error { return nil },
		RemoteHEADFn:     func(clonePath, branch string) (string, error) { return "abc123", nil },
		ListFilesFn: func(clonePath, branch string) ([]string, error) {
			return []string{
				"category/profile/agents/foo.md",
				"category/profile/skills/bar.md",
			}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		SetMarketSyncCleanFn: func(cacheDir string, market string, newSHA string) error { return nil },
	}

	a := newTestApp(cfg, git, fsMock, state)
	result, err := a.AddMarket("https://github.com/org/repo", service.AddMarketOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Profiles != 1 {
		t.Errorf("expected Profiles=1, got %d", result.Profiles)
	}
	if result.Agents != 1 {
		t.Errorf("expected Agents=1, got %d", result.Agents)
	}
	if result.Skills != 1 {
		t.Errorf("expected Skills=1, got %d", result.Skills)
	}
}

func TestDetectSkillsOnly(t *testing.T) {
	cases := []struct {
		name       string
		files      []string
		skillsPath string
		want       bool
	}{
		{
			name:  "skills-only repo with root-level skills",
			files: []string{"skills/lint/SKILL.md", "skills/test/SKILL.md"},
			want:  true,
		},
		{
			name:  "mct market with nested skills",
			files: []string{"dev/go/agents/foo.md", "dev/go/skills/bar/SKILL.md"},
			want:  false,
		},
		{
			name:  "no SKILL.md files",
			files: []string{"agents/foo.md", "skills/bar.md"},
			want:  false,
		},
		{
			name:  "empty repo",
			files: []string{},
			want:  false,
		},
		{
			name:       "custom skills path",
			files:      []string{"src/skills/lint/SKILL.md", "src/skills/test/SKILL.md"},
			skillsPath: "src/skills",
			want:       true,
		},
		{
			name:       "custom skills path no match",
			files:      []string{"skills/lint/SKILL.md"},
			skillsPath: "src/skills",
			want:       false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := &gitrepotest.MockGitRepo{
				ListFilesFn: func(clonePath, branch string) ([]string, error) {
					return tc.files, nil
				},
			}
			got := detectSkillsOnly(git, "/some/path", "main", tc.skillsPath)
			if got != tc.want {
				t.Errorf("detectSkillsOnly() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAddMarket_AutoDetectsSkillsOnly(t *testing.T) {
	var savedMC domain.MarketConfig
	var removedPaths []string
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
		AddMarketFn: func(path string, mc domain.MarketConfig) error {
			savedMC = mc
			return nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		ListDirFn: func(path string) ([]string, error) {
			return []string{".git", "skills", "src", "README.md"}, nil
		},
		RemoveAllFn: func(path string) error {
			removedPaths = append(removedPaths, filepath.Base(path))
			return nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ValidateRemoteFn: func(url string) error { return nil },
		CloneFn:          func(url, clonePath string) error { return nil },
		RemoteHEADFn:     func(clonePath, branch string) (string, error) { return "abc123", nil },
		ListFilesFn: func(clonePath, branch string) ([]string, error) {
			return []string{
				"skills/lint/SKILL.md",
				"skills/test/SKILL.md",
			}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		SetMarketSyncCleanFn: func(cacheDir string, market string, newSHA string) error { return nil },
	}

	a := newTestApp(cfg, git, fsMock, state)
	_, err := a.AddMarket("https://github.com/org/skills-repo", service.AddMarketOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !savedMC.SkillsOnly {
		t.Error("expected SkillsOnly=true for repo with skills/*/SKILL.md, got false")
	}
	// Should have pruned src and README.md but kept .git and skills
	if len(removedPaths) != 2 {
		t.Errorf("expected 2 removals, got %d: %v", len(removedPaths), removedPaths)
	}
	for _, p := range removedPaths {
		if p == ".git" || p == "skills" {
			t.Errorf("should not have removed %q", p)
		}
	}
}

func TestParseTreeURL(t *testing.T) {
	cases := []struct {
		input      string
		wantURL    string
		wantBranch string
		wantPath   string
	}{
		{
			input:   "https://github.com/org/repo",
			wantURL: "https://github.com/org/repo",
		},
		{
			input:      "https://github.com/microsoft/azure-skills/tree/main/skills",
			wantURL:    "https://github.com/microsoft/azure-skills",
			wantBranch: "main",
			wantPath:   "skills",
		},
		{
			input:      "https://github.com/org/repo/tree/develop/src/skills",
			wantURL:    "https://github.com/org/repo",
			wantBranch: "develop",
			wantPath:   "src/skills",
		},
		{
			input:      "https://github.com/org/repo/tree/main",
			wantURL:    "https://github.com/org/repo",
			wantBranch: "main",
		},
		{
			input:   "git@github.com:org/repo.git",
			wantURL: "git@github.com:org/repo.git",
		},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			gotURL, gotBranch, gotPath := parseTreeURL(tc.input)
			if gotURL != tc.wantURL {
				t.Errorf("url: got %q, want %q", gotURL, tc.wantURL)
			}
			if gotBranch != tc.wantBranch {
				t.Errorf("branch: got %q, want %q", gotBranch, tc.wantBranch)
			}
			if gotPath != tc.wantPath {
				t.Errorf("path: got %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}

func TestAddMarket_NameAlreadyExists(t *testing.T) {
	// Two different hosts but same org/repo derive the same market name,
	// so this should hit MARKET_ALREADY_EXISTS.
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{
					{Name: "org/repo", URL: "https://gitlab.com/org/repo", Branch: "main"},
				},
			}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	_, err := a.AddMarket("https://github.com/org/repo", service.AddMarketOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != "MARKET_ALREADY_EXISTS" {
		t.Errorf("expected MARKET_ALREADY_EXISTS, got %v", err)
	}
}

func TestAddMarket_URLExists(t *testing.T) {
	// Same URL with .git suffix normalizes to the same URL, but the existing
	// market was renamed so the derived name differs — should hit URL check.
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{
					{Name: "my-custom-name", URL: "https://github.com/org/repo", Branch: "main"},
				},
			}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	_, err := a.AddMarket("https://github.com/org/repo.git", service.AddMarketOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != "MARKET_URL_EXISTS" {
		t.Errorf("expected MARKET_URL_EXISTS, got %v", err)
	}
}

func TestAddMarket_InvalidURL(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	_, err := a.AddMarket("https://example.com", service.AddMarketOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != "INVALID_MARKET_URL" {
		t.Errorf("expected INVALID_MARKET_URL, got %v", err)
	}
}

func TestAddMarket_ClonePathExists(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	// clonePath is an absolute path (/cache/dir/mymarket), which MapFS can't represent.
	// Use StatFn to simulate directory existence.
	fsMock := &filesystemtest.MockFilesystem{
		StatFn: func(name string) (fs.FileInfo, error) {
			return fakeDir{}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})

	_, err := a.AddMarket("https://github.com/org/repo", service.AddMarketOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != "CLONE_EXISTS" {
		t.Errorf("expected CLONE_EXISTS, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RemoveMarket
// ---------------------------------------------------------------------------

func TestRemoveMarket_Success(t *testing.T) {
	removeMarketCalled := false
	saveSyncCalled := false
	removeAllCalled := false

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets: []domain.MarketConfig{
					{Name: "foo", URL: "https://example.com", Branch: "main"},
				},
			}, nil
		},
		RemoveMarketFn: func(path string, name string) error {
			removeMarketCalled = true
			return nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{
				Version: 1,
				Markets: map[string]domain.MarketSyncState{
					"foo": {LastSyncedSHA: "abc", Status: "clean"},
				},
			}, nil
		},
		SaveSyncStateFn: func(cacheDir string, s domain.SyncState) error {
			saveSyncCalled = true
			if _, ok := s.Markets["foo"]; ok {
				return errors.New("foo should have been deleted from sync state")
			}
			return nil
		},
	}
	fsMock := &filesystemtest.MockFilesystem{
		RemoveAllFn: func(path string) error {
			removeAllCalled = true
			return nil
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, state)
	err := a.RemoveMarket("foo", service.RemoveMarketOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !removeMarketCalled {
		t.Error("expected RemoveMarket to be called")
	}
	if !saveSyncCalled {
		t.Error("expected SaveSyncState to be called")
	}
	if !removeAllCalled {
		t.Error("expected RemoveAll to be called")
	}
}

func TestRemoveMarket_NotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{
					{Name: "other", URL: "https://example.com", Branch: "main"},
				},
			}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	err := a.RemoveMarket("foo", service.RemoveMarketOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != "MARKET_NOT_FOUND" {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

func TestRemoveMarket_HasInstalledEntries(t *testing.T) {
	filePath := ".claude/agents/bar.md"

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: ".claude",
				Markets: []domain.MarketConfig{
					{Name: "foo", URL: "https://example.com", Branch: "main"},
				},
			}, nil
		},
	}
	symlinks := map[string]string{
		filePath: "/cache/dir/foo/agents/bar.md",
	}
	fsMock := &filesystemtest.MockFilesystem{
		FS: fstest.MapFS{
			filePath: &fstest.MapFile{Data: []byte("---\ndescription: test\n---\n")},
		},
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
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, fsMock, &statestoretest.MockStateStore{})
	err := a.RemoveMarket("foo", service.RemoveMarketOpts{Force: false})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != "MARKET_HAS_INSTALLED_ENTRIES" {
		t.Errorf("expected MARKET_HAS_INSTALLED_ENTRIES, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RenameMarket
// ---------------------------------------------------------------------------

func TestRenameMarket_Success(t *testing.T) {
	setPropertyCalled := false
	var setPropertyKey, setPropertyValue string

	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{
					{Name: "foo", URL: "https://example.com", Branch: "main"},
				},
			}, nil
		},
		SetMarketPropertyFn: func(path, marketName, key, value string) error {
			setPropertyCalled = true
			setPropertyKey = key
			setPropertyValue = value
			return nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{
				Version: 1,
				Markets: map[string]domain.MarketSyncState{
					"foo": {LastSyncedSHA: "abc", Status: "clean"},
				},
			}, nil
		},
		SaveSyncStateFn: func(cacheDir string, s domain.SyncState) error {
			if _, ok := s.Markets["bar"]; !ok {
				return errors.New("expected bar in sync state after rename")
			}
			if _, ok := s.Markets["foo"]; ok {
				return errors.New("foo should be removed from sync state after rename")
			}
			return nil
		},
	}

	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, state)
	err := a.RenameMarket("foo", "bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !setPropertyCalled {
		t.Error("expected SetMarketProperty to be called")
	}
	if setPropertyKey != "name" || setPropertyValue != "bar" {
		t.Errorf("expected SetMarketProperty(name, bar), got (%q, %q)", setPropertyKey, setPropertyValue)
	}
}

// ---------------------------------------------------------------------------
// MarketInfo
// ---------------------------------------------------------------------------

func TestMarketInfo_NotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	_, err := a.MarketInfo("missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "MARKET_NOT_FOUND") {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

func TestMarketInfo_Found(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				Markets: []domain.MarketConfig{{Name: "mkt", URL: "https://example.com", Branch: "main"}},
			}, nil
		},
	}
	state := &statestoretest.MockStateStore{
		LoadSyncStateFn: func(cacheDir string) (domain.SyncState, error) {
			return domain.SyncState{
				Version: 1,
				Markets: map[string]domain.MarketSyncState{
					"mkt": {LastSyncedSHA: "abc123", Status: "clean"},
				},
			}, nil
		},
	}
	git := &gitrepotest.MockGitRepo{
		ListFilesFn: func(clonePath, branch string) ([]string, error) {
			return []string{"dev/go/agents/foo.md", "dev/go/agents/bar.md"}, nil
		},
	}

	a := newTestApp(cfg, git, &filesystemtest.MockFilesystem{}, state)
	result, err := a.MarketInfo("mkt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Market.Name != "mkt" {
		t.Errorf("expected Market.Name=mkt, got %q", result.Market.Name)
	}
	if result.EntryCount != 2 {
		t.Errorf("expected EntryCount=2, got %d", result.EntryCount)
	}
	if result.Status != "clean" {
		t.Errorf("expected Status=clean, got %q", result.Status)
	}
}

// ---------------------------------------------------------------------------
// RenameMarket (additional cases)
// ---------------------------------------------------------------------------

func TestRenameMarket_NotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{}, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	err := a.RenameMarket("missing", "newname")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isDomainErrorWithCode(err, "MARKET_NOT_FOUND") {
		t.Errorf("expected MARKET_NOT_FOUND, got %v", err)
	}
}

func TestRenameMarket_EmptyName(t *testing.T) {
	a := newTestApp(&configstoretest.MockConfigStore{}, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	err := a.RenameMarket("foo", "")
	if err == nil {
		t.Fatal("expected error for empty new name, got nil")
	}
}

// ---------------------------------------------------------------------------
// SetConfigField
// ---------------------------------------------------------------------------

func TestSetConfigField_Success(t *testing.T) {
	var gotPath, gotKey, gotValue string
	cfg := &configstoretest.MockConfigStore{
		SetConfigFieldFn: func(path string, key string, value string) error {
			gotPath = path
			gotKey = key
			gotValue = value
			return nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	err := a.SetConfigField("local_path", ".claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/config/path" {
		t.Errorf("expected path /config/path, got %q", gotPath)
	}
	if gotKey != "local_path" {
		t.Errorf("expected key local_path, got %q", gotKey)
	}
	if gotValue != ".claude" {
		t.Errorf("expected value .claude, got %q", gotValue)
	}
}

// ---------------------------------------------------------------------------
// normalizeURL
// ---------------------------------------------------------------------------

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo", "github.com/org/repo"},
		{"https://github.com/org/repo.git", "github.com/org/repo"},
		{"https://github.com/org/repo/", "github.com/org/repo"},
		{"https://github.com/org/repo.git/", "github.com/org/repo.git"},
		{"git@github.com:org/repo.git", "github.com/org/repo"},
		{"git@github.com:org/repo", "github.com/org/repo"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeURL(tc.input)
			if got != tc.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SetMarketProperty
// ---------------------------------------------------------------------------

func TestSetMarketProperty(t *testing.T) {
	var gotPath, gotMarket, gotKey, gotValue string
	cfg := &configstoretest.MockConfigStore{
		SetMarketPropertyFn: func(path, marketName, key, value string) error {
			gotPath = path
			gotMarket = marketName
			gotKey = key
			gotValue = value
			return nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	err := a.SetMarketProperty("foo", "branch", "develop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/config/path" {
		t.Errorf("expected path /config/path, got %q", gotPath)
	}
	if gotMarket != "foo" {
		t.Errorf("expected market foo, got %q", gotMarket)
	}
	if gotKey != "branch" {
		t.Errorf("expected key branch, got %q", gotKey)
	}
	if gotValue != "develop" {
		t.Errorf("expected value develop, got %q", gotValue)
	}
}

// ---------------------------------------------------------------------------
// GetConfig
// ---------------------------------------------------------------------------

func TestGetConfig(t *testing.T) {
	expected := domain.Config{
		LocalPath: ".claude",
		Markets: []domain.MarketConfig{
			{Name: "foo", URL: "https://example.com", Branch: "main"},
		},
	}
	cfg := &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return expected, nil
		},
	}
	a := newTestApp(cfg, &gitrepotest.MockGitRepo{}, &filesystemtest.MockFilesystem{}, &statestoretest.MockStateStore{})

	got, err := a.GetConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.LocalPath != expected.LocalPath {
		t.Errorf("expected LocalPath %q, got %q", expected.LocalPath, got.LocalPath)
	}
	if len(got.Markets) != 1 || got.Markets[0].Name != "foo" {
		t.Errorf("unexpected markets: %v", got.Markets)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func isDomainErrorWithCode(err error, code string) bool {
	var de *domain.DomainError
	return errors.As(err, &de) && de.Code == code
}
