package app

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
)

func newLintTestApp() *App {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fsMock := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}
	return newTestApp(cfg, git, fsMock, state)
}

func TestLintMarket_EmptyDir(t *testing.T) {
	a := newLintTestApp()
	result, err := a.LintMarket(fstest.MapFS{}, ".")
	if err != nil {
		t.Fatal(err)
	}
	if result.Profiles != 0 {
		t.Errorf("expected 0 profiles, got %d", result.Profiles)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %v", result.Issues)
	}
}

func TestLintMarket_ValidProfile(t *testing.T) {
	fsys := fstest.MapFS{
		"dev/go/README.md":     {Data: []byte("---\ntags:\n  - golang\ndescription: Go tools\n---\n# README\n")},
		"dev/go/agents/foo.md": {Data: []byte("---\ndescription: test\n---\n# foo\n")},
		"dev/go/skills/bar.md": {Data: []byte("---\ndescription: test\n---\n# bar\n")},
	}
	a := newLintTestApp()
	result, err := a.LintMarket(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if result.Profiles != 1 {
		t.Errorf("expected 1 profile, got %d", result.Profiles)
	}
	if result.Agents != 1 {
		t.Errorf("expected 1 agent, got %d", result.Agents)
	}
	if result.Skills != 1 {
		t.Errorf("expected 1 skill, got %d", result.Skills)
	}
	if len(result.Issues) != 0 {
		t.Errorf("unexpected issues: %v", result.Issues)
	}
}

func TestLintMarket_MissingReadme(t *testing.T) {
	fsys := fstest.MapFS{
		"dev/go/agents/foo.md": {Data: []byte("---\ndescription: test\n---\n# foo\n")},
	}
	a := newLintTestApp()
	result, err := a.LintMarket(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Severity == "warn" && strings.Contains(issue.Message, "missing README") {
			found = true
		}
	}
	if !found {
		t.Error("expected missing README warn issue")
	}
}

func TestLintMarket_BadFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		"dev/go/agents/foo.md": {Data: []byte("no frontmatter at all")},
	}
	a := newLintTestApp()
	result, err := a.LintMarket(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Error("expected error issue for bad frontmatter")
	}
}

func TestLintMarket_MissingSkillDep(t *testing.T) {
	agentContent := []byte("---\ndescription: test\nrequires_skills:\n  - file: dev/go/skills/missing.md\n---\n# agent\n")
	fsys := fstest.MapFS{
		"dev/go/README.md":     {Data: []byte("---\ntags:\n  - golang\n---\n# README\n")},
		"dev/go/agents/foo.md": {Data: agentContent},
	}
	a := newLintTestApp()
	result, err := a.LintMarket(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Severity == "error" && strings.Contains(issue.Message, "missing file") {
			found = true
		}
	}
	if !found {
		t.Error("expected missing skill dep error")
	}
}

func TestLintMarket_ReadmeNoTags(t *testing.T) {
	fsys := fstest.MapFS{
		"dev/go/README.md":     {Data: []byte("---\ndescription: no tags here\n---\n# README\n")},
		"dev/go/agents/foo.md": {Data: []byte("---\ndescription: test\n---\n# foo\n")},
	}
	a := newLintTestApp()
	result, err := a.LintMarket(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Severity == "warn" && strings.Contains(issue.Message, "no tags") {
			found = true
		}
	}
	if !found {
		t.Error("expected no-tags warn issue")
	}
}
