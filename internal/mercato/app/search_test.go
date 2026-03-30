package app

import (
	"testing"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore/configstoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem/filesystemtest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo/gitrepotest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore/statestoretest"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// noInstalledEntries configures the fs mock so scanInstalledEntries returns nothing.
// An empty MockFilesystem has no files in MapFS, so DirExists returns false.
func noInstalledEntries(fs *filesystemtest.MockFilesystem) {
	// Empty MapFS means Stat on any path returns error → DirExists = false.
	// Nothing to do — zero-value MockFilesystem already behaves this way.
}

// agentContent is a minimal agent frontmatter for search tests.
var agentContent = []byte("---\ntype: agent\ndescription: Go expert\nauthor: alice\n---\n# foo\n")

// readmeContentBytes is a minimal README frontmatter for search tests.
var readmeContentBytes = []byte("---\ntags:\n  - golang\n  - dev\ndescription: Go profile\n---\n# README\n")

// rustAgentContent is an agent not related to Go, ensuring BM25 IDF is non-zero for "go".
var rustAgentContent = []byte("---\ntype: agent\ndescription: Rust expert\nauthor: bob\n---\n# bar\n")

// rustReadmeContentBytes is a README for the rust profile.
var rustReadmeContentBytes = []byte("---\ntags:\n  - rust\n  - systems\ndescription: Rust profile\n---\n# README\n")

// setupSearchMocks configures mocks with one market "mkt" and four files:
// a Go agent, a Go README, a Rust agent and a Rust README. The two profiles
// ensure BM25 IDF is non-zero so "go" returns results.
func setupSearchMocks() (*configstoretest.MockConfigStore, *gitrepotest.MockGitRepo, *filesystemtest.MockFilesystem, *statestoretest.MockStateStore) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fs := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{
			Markets: []domain.MarketConfig{{Name: "mkt", Branch: "main"}},
		}, nil
	}
	git.ReadMarketFilesFn = func(clonePath, branch string) ([]gitrepo.MarketFile, error) {
		return []gitrepo.MarketFile{
			{Path: "dev/go/agents/foo.md", Content: agentContent, Version: "sha1"},
			{Path: "dev/go/README.md", Content: readmeContentBytes, Version: "sha0"},
			{Path: "dev/rust/agents/bar.md", Content: rustAgentContent, Version: "sha2"},
			{Path: "dev/rust/README.md", Content: rustReadmeContentBytes, Version: "sha3"},
		}, nil
	}
	noInstalledEntries(fs)
	return cfg, git, fs, state
}

// TestBuildCorpus_Empty verifies that an empty market list returns nil.
func TestBuildCorpus_Empty(t *testing.T) {
	cfg := &configstoretest.MockConfigStore{}
	git := &gitrepotest.MockGitRepo{}
	fs := &filesystemtest.MockFilesystem{}
	state := &statestoretest.MockStateStore{}

	cfg.LoadFn = func(path string) (domain.Config, error) {
		return domain.Config{Markets: nil}, nil
	}
	noInstalledEntries(fs)

	app := newTestApp(cfg, git, fs, state)
	entries, err := app.buildCorpus()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestBuildCorpus_WithEntries verifies that buildCorpus returns one entry (agent) and
// populates MctTags from the sibling README.md.
func TestBuildCorpus_WithEntries(t *testing.T) {
	cfg, git, fs, state := setupSearchMocks()

	app := newTestApp(cfg, git, fs, state)
	entries, err := app.buildCorpus()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (READMEs excluded), got %d", len(entries))
	}
	// Find the Go agent entry.
	var e domain.Entry
	for _, candidate := range entries {
		if candidate.Ref == "mkt/dev/go/agents/foo.md" {
			e = candidate
		}
	}
	if e.Ref != "mkt/dev/go/agents/foo.md" {
		t.Errorf("unexpected Ref: %q", e.Ref)
	}
	if e.Market != "mkt" {
		t.Errorf("unexpected Market: %q", e.Market)
	}
	if e.Type != domain.EntryTypeAgent {
		t.Errorf("unexpected Type: %q", e.Type)
	}
	if len(e.MctTags) != 2 {
		t.Errorf("expected 2 MctTags from README, got %d: %v", len(e.MctTags), e.MctTags)
	}
	foundGolang, foundDev := false, false
	for _, tag := range e.MctTags {
		if tag == "golang" {
			foundGolang = true
		}
		if tag == "dev" {
			foundDev = true
		}
	}
	if !foundGolang || !foundDev {
		t.Errorf("expected tags [golang dev], got %v", e.MctTags)
	}
	if e.ProfileDescription != "Go profile" {
		t.Errorf("expected ProfileDescription=Go profile, got %q", e.ProfileDescription)
	}
}

// TestSearch_ReturnsResults verifies that a relevant query returns at least one result.
// "golang" is used because it appears only in the Go agent doc (IDF > 0).
func TestSearch_ReturnsResults(t *testing.T) {
	cfg, git, fs, state := setupSearchMocks()

	app := newTestApp(cfg, git, fs, state)
	results, err := app.Search("golang", service.SearchOpts{Limit: 10})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 search result")
	}
}

// TestSearch_FilterByType verifies that filtering by a type with no matches returns 0 results.
func TestSearch_FilterByType(t *testing.T) {
	cfg, git, fs, state := setupSearchMocks()

	app := newTestApp(cfg, git, fs, state)
	results, err := app.Search("go", service.SearchOpts{Type: domain.EntryTypeSkill})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when filtering for skills, got %d", len(results))
	}
}

// TestSearch_FilterByMarket verifies that filtering by a non-existent market returns 0 results.
func TestSearch_FilterByMarket(t *testing.T) {
	cfg, git, fs, state := setupSearchMocks()

	app := newTestApp(cfg, git, fs, state)
	results, err := app.Search("go", service.SearchOpts{Market: "other"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when filtering for market=other, got %d", len(results))
	}
}

// TestDumpIndex verifies that DumpIndex returns all entries.
func TestDumpIndex(t *testing.T) {
	cfg, git, fs, state := setupSearchMocks()

	app := newTestApp(cfg, git, fs, state)
	entries, err := app.DumpIndex()
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	// Two agents (Go + Rust), READMEs excluded.
	if len(entries) != 2 {
		t.Errorf("expected 2 entries in index, got %d", len(entries))
	}
}

// TestLevenshtein verifies the levenshtein edit distance function.
func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abx", 1},
		{"cat", "car", 1},
		{"abc", "", 3},
		{"", "abc", 3},
		{"kitten", "sitting", 3}, // classic example — length diff > 2 triggers early exit returning 3
	}
	for _, tc := range cases {
		got := levenshtein(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestExpandFuzzy_ExactMatch verifies that a token already in the vocabulary
// is not duplicated.
func TestExpandFuzzy_ExactMatch(t *testing.T) {
	vocab := []string{"golang", "rust", "python"}
	tokens := []string{"golang"}
	got := expandFuzzy(tokens, vocab)
	// Should contain exactly one "golang" (no fuzzy expansion needed).
	count := 0
	for _, tok := range got {
		if tok == "golang" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'golang' token in expanded result, got %d: %v", count, got)
	}
}

// TestExpandFuzzy_FuzzyMatch verifies that a misspelled token close to a
// vocabulary term is expanded with the closest term.
func TestExpandFuzzy_FuzzyMatch(t *testing.T) {
	vocab := []string{"golang", "rust"}
	// "golng" is distance 1 from "golang"
	tokens := []string{"golng"}
	got := expandFuzzy(tokens, vocab)
	// Should contain both the original and the fuzzy-matched term.
	hasOriginal := false
	hasExpanded := false
	for _, tok := range got {
		if tok == "golng" {
			hasOriginal = true
		}
		if tok == "golang" {
			hasExpanded = true
		}
	}
	if !hasOriginal {
		t.Error("expected original token 'golng' in result")
	}
	if !hasExpanded {
		t.Errorf("expected fuzzy-expanded token 'golang' in result, got %v", got)
	}
}

// TestTokenize verifies the tokenize helper function.
func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"Go-lang", []string{"go", "lang"}},
	}
	for _, tt := range tests {
		got := tokenize(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("tokenize(%q): expected %v, got %v", tt.input, tt.expected, got)
			continue
		}
		for i, tok := range got {
			if tok != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d]: expected %q, got %q", tt.input, i, tt.expected[i], tok)
			}
		}
	}
}

// TestIsReadme verifies the isReadme helper function.
func TestIsReadme(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"dev/go/README.md", true},
		{"dev/go/agents/foo.md", false},
		{"dev/go/sub/README.md", false}, // 4 parts
		{"README.md", false},            // only 1 part
	}
	for _, tt := range tests {
		got := isReadme(tt.path)
		if got != tt.expected {
			t.Errorf("isReadme(%q): expected %v, got %v", tt.path, tt.expected, got)
		}
	}
}
