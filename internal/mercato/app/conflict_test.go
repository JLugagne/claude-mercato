package app

import (
	"testing"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
)

// TestConflicts_NoConflicts verifies no conflicts when all filenames are unique.
func TestConflicts_NoConflicts(t *testing.T) {
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
		return []string{"agents/foo.md"}, nil
	}
	// ReadFileAtRef used for dep-deleted check — not needed here (no managed skills).

	app := newTestApp(cfg, git, fs, state)
	conflicts, err := app.Conflicts()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	for _, c := range conflicts {
		t.Errorf("unexpected conflict: %+v", c)
	}
}

// TestConflicts_RefCollision verifies ref-collision when two markets share a filename.
func TestConflicts_RefCollision(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fs := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{
				{Name: "mkt1", Branch: "main"},
				{Name: "mkt2", Branch: "main"},
			},
		}, nil
	}
	git.ListFilesFn = func(clonePath, branch string) ([]string, error) {
		// Both markets expose the same filename.
		return []string{"agents/foo.md"}, nil
	}

	app := newTestApp(cfg, git, fs, state)
	conflicts, err := app.Conflicts()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	refCollisions := 0
	for _, c := range conflicts {
		if c.Type == "ref-collision" {
			refCollisions++
		}
	}
	if refCollisions != 1 {
		t.Errorf("expected 1 ref-collision conflict, got %d (total conflicts: %d)", refCollisions, len(conflicts))
	}
}

