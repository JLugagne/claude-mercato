package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

func cfgWithLocalPath(local string) *configstoretest.MockConfigStore {
	return &configstoretest.MockConfigStore{
		LoadFn: func(path string) (domain.Config, error) {
			return domain.Config{
				LocalPath: local,
				Markets:   []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
			}, nil
		},
	}
}

// TestDetectDeleted_FindsMissingFiles verifies that files recorded in the
// install database but missing on disk are reported. Hooks (encoded as
// "...settings.json#hooks/x") are excluded.
func TestDetectDeleted_FindsMissingFiles(t *testing.T) {
	cfg := cfgWithLocalPath(".claude")
	state := &statestoretest.MockStateStore{}
	git := &gitrepotest.MockGitRepo{}

	fsMock := &filesystemtest.MockFilesystem{
		StatFn: func(name string) (fs.FileInfo, error) {
			if filepath.Base(name) == "alive.md" {
				return fakeDirInfo{name: name}, nil
			}
			return nil, &fs.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
		},
	}

	db := domain.InstallDatabase{
		Markets: []domain.InstalledMarket{
			{Market: "mkt", Packages: []domain.InstalledPackage{
				{
					Profile: "agents/group",
					Version: "v1",
					Files:   domain.InstalledFiles{Agents: []string{"alive.md", "gone.md"}, Hooks: []string{"h.json"}},
					Locations: []domain.InstalledLocation{
						{
							Path: testProjectPath(),
							Type: domain.RuntimeTypeClaudeCode,
							Files: []domain.InstalledFile{
								{Path: ".claude/agents/alive.md", XXH: "h1"},
								{Path: ".claude/agents/gone.md", XXH: "h2"},
								{Path: ".claude/settings.json#hooks/h.json", XXH: "h3"},
							},
						},
					},
				},
			}},
		},
	}
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, state, idb)
	got, err := app.DetectDeleted(service.DetectDeletedOpts{})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 deleted file, got %d (%+v)", len(got), got)
	}
	if got[0].RelPath != ".claude/agents/gone.md" {
		t.Errorf("expected gone.md, got %s", got[0].RelPath)
	}
	if got[0].Market != "mkt" || got[0].Profile != "agents/group" {
		t.Errorf("wrong identification: %+v", got[0])
	}
}

// TestRestoreDeleted_ReadsAndWritesFromClone verifies that RestoreDeleted
// reads the original content from the cached clone at pkg.Version and writes
// it back via the tx writer (captured here by intercepting WriteFile).
func TestRestoreDeleted_ReadsAndWritesFromClone(t *testing.T) {
	cfg := cfgWithLocalPath(".claude")
	state := &statestoretest.MockStateStore{}
	git := &gitrepotest.MockGitRepo{
		ReadFileAtRefFn: func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
			if commitSHA != "v1" {
				return nil, errors.New("wrong version")
			}
			if filePath != "agents/group/agents/gone.md" {
				return nil, errors.New("wrong path: " + filePath)
			}
			return []byte("original content"), nil
		},
	}

	written := map[string][]byte{}
	fsMock := &filesystemtest.MockFilesystem{
		WriteFileFn: func(path string, content []byte) error {
			written[path] = content
			return nil
		},
	}

	db := domain.InstallDatabase{
		Markets: []domain.InstalledMarket{
			{Market: "mkt", Packages: []domain.InstalledPackage{
				{
					Profile: "agents/group",
					Version: "v1",
					Files:   domain.InstalledFiles{Agents: []string{"gone.md"}},
					Locations: []domain.InstalledLocation{
						{Path: "/proj", Type: domain.RuntimeTypeClaudeCode},
					},
				},
			}},
		},
	}
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, state, idb)
	results, err := app.RestoreDeleted(
		[]service.DeletedFile{{
			Market:   "mkt",
			Profile:  "agents/group",
			Location: "/proj",
			RelPath:  ".claude/agents/gone.md",
			XXH:      "originalhash",
		}},
		service.RestoreOpts{},
	)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 || results[0].Action != "restored" {
		t.Fatalf("expected single restored result, got %+v", results)
	}
	want := "/proj/.claude/agents/gone.md"
	if string(written[want]) != "original content" {
		t.Errorf("expected WriteFile(%s) with original content, got: %v", want, written)
	}
}

// TestRestoreDeleted_DryRun ensures no write happens and every result is
// flagged as would_restore.
func TestRestoreDeleted_DryRun(t *testing.T) {
	cfg := cfgWithLocalPath(".claude")
	state := &statestoretest.MockStateStore{}
	git := &gitrepotest.MockGitRepo{}

	fsMock := &filesystemtest.MockFilesystem{
		WriteFileFn: func(path string, content []byte) error {
			t.Fatalf("WriteFile must not be called under DryRun, got %s", path)
			return nil
		},
	}

	db := domain.InstallDatabase{
		Markets: []domain.InstalledMarket{
			{Market: "mkt", Packages: []domain.InstalledPackage{
				{
					Profile: "p",
					Version: "v1",
					Files:   domain.InstalledFiles{Agents: []string{"x.md"}},
					Locations: []domain.InstalledLocation{
						{Path: "/proj", Type: domain.RuntimeTypeClaudeCode},
					},
				},
			}},
		},
	}
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, state, idb)
	results, err := app.RestoreDeleted(
		[]service.DeletedFile{{
			Market: "mkt", Profile: "p", Location: "/proj",
			RelPath: ".claude/agents/x.md", XXH: "h",
		}},
		service.RestoreOpts{DryRun: true},
	)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 || results[0].Action != "would_restore" {
		t.Fatalf("expected would_restore, got %+v", results)
	}
}

// TestRestoreDeleted_FailsOnMissingMarket verifies that a request targeting
// a market absent from config produces a failed result without panicking.
func TestRestoreDeleted_FailsOnMissingMarket(t *testing.T) {
	cfg := cfgWithLocalPath(".claude")
	state := &statestoretest.MockStateStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}

	db := domain.InstallDatabase{}
	idb := idbWithData(db)

	app := newTestApp(cfg, git, fsMock, state, idb)
	results, err := app.RestoreDeleted(
		[]service.DeletedFile{{
			Market: "unknown", Profile: "p", Location: "/proj",
			RelPath: ".claude/agents/x.md", XXH: "h",
		}},
		service.RestoreOpts{},
	)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 || results[0].Action != "failed" {
		t.Fatalf("expected failed result, got %+v", results)
	}
	if !errors.Is(results[0].Err, errMarketNotFound) {
		t.Errorf("expected errMarketNotFound, got %v", results[0].Err)
	}
}
