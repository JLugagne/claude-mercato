package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
)

// readmeFileContent is valid README frontmatter with tags.
var readmeFileContent = []byte("---\ntags:\n  - golang\n  - dev\ndescription: Go tools\n---\n# README\n")

// TestReadme_Success verifies successful README retrieval.
func TestReadme_Success(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fs := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	git.ReadFileAtRefFn = func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
		return readmeFileContent, nil
	}

	app := newTestApp(cfg, git, fs, state)
	entry, err := app.Readme("mkt", "README.md")
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if entry.Market != "mkt" {
		t.Errorf("expected Market=mkt, got %q", entry.Market)
	}
	if entry.Path != "README.md" {
		t.Errorf("expected Path=README.md, got %q", entry.Path)
	}
	if entry.Content == "" {
		t.Error("expected non-empty Content")
	}
	if len(entry.MctTags) == 0 {
		t.Error("expected MctTags to be populated from frontmatter")
	}
}

// TestReadme_MarketNotFound verifies ErrMarketNotFound when market is missing.
func TestReadme_MarketNotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fs := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{Markets: nil}, nil
	}

	app := newTestApp(cfg, git, fs, state)
	_, err := app.Readme("mkt", "README.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrMarketNotFound) {
		t.Errorf("expected ErrMarketNotFound, got %v", err)
	}
}

// TestReadme_NotFound verifies README_NOT_FOUND error code when file is missing.
func TestReadme_NotFound(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fs := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	git.ReadFileAtRefFn = func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
		return nil, errors.New("not found")
	}

	app := newTestApp(cfg, git, fs, state)
	_, err := app.Readme("mkt", "README.md")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("expected DomainError, got %T: %v", err, err)
	}
	if de.Code != "README_NOT_FOUND" {
		t.Errorf("expected code README_NOT_FOUND, got %q", de.Code)
	}
}

// TestListReadmes verifies that ListReadmes returns only 3-part README.md paths.
func TestListReadmes(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fs := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	git.ListFilesFn = func(clonePath, branch string) ([]string, error) {
		return []string{
			"dev/go/README.md",
			"dev/go/agents/foo.md",
			"dev/rust/README.md",
		}, nil
	}
	git.ReadFileAtRefFn = func(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
		if strings.HasSuffix(filePath, "README.md") {
			return readmeFileContent, nil
		}
		return agentContent, nil
	}

	app := newTestApp(cfg, git, fs, state)
	readmes, err := app.ListReadmes("mkt")
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(readmes) != 2 {
		t.Fatalf("expected 2 readmes (3-part paths only), got %d", len(readmes))
	}
	for _, re := range readmes {
		if re.Market != "mkt" {
			t.Errorf("expected Market=mkt, got %q", re.Market)
		}
		if !strings.HasSuffix(re.Path, "README.md") {
			t.Errorf("expected README.md path, got %q", re.Path)
		}
		if re.Content == "" {
			t.Error("expected non-empty Content")
		}
	}
}
