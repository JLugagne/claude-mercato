package domain

import (
	"strings"
	"testing"
)

func makeFrontmatter(fields string) []byte {
	return []byte("---\n" + fields + "\n---\n# body\n")
}

// --- TestExtractFrontmatterBytes ---

func TestExtractFrontmatterBytes(t *testing.T) {
	t.Run("valid content returns yaml bytes", func(t *testing.T) {
		content := []byte("---\nfoo: bar\n---\n# body")
		got, err := ExtractFrontmatterBytes(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != "foo: bar" {
			t.Errorf("expected %q, got %q", "foo: bar", string(got))
		}
	})

	t.Run("no leading --- returns error", func(t *testing.T) {
		content := []byte("foo: bar\n---\n# body")
		_, err := ExtractFrontmatterBytes(content)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("no closing --- returns error", func(t *testing.T) {
		content := []byte("---\nfoo: bar\n# body")
		_, err := ExtractFrontmatterBytes(content)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty frontmatter returns empty bytes", func(t *testing.T) {
		content := []byte("---\n---\n")
		got, err := ExtractFrontmatterBytes(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty bytes, got %q", string(got))
		}
	})
}

// --- TestParseFrontmatter ---

func TestParseFrontmatter(t *testing.T) {
	t.Run("valid agent frontmatter parsed correctly", func(t *testing.T) {
		content := makeFrontmatter("type: agent\ndescription: A test agent\nauthor: alice\nbreaking_change: true\ndeprecated: false")
		fm, err := ParseFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fm.Type != EntryTypeAgent {
			t.Errorf("expected type %q, got %q", EntryTypeAgent, fm.Type)
		}
		if fm.Description != "A test agent" {
			t.Errorf("expected description %q, got %q", "A test agent", fm.Description)
		}
		if fm.Author != "alice" {
			t.Errorf("expected author %q, got %q", "alice", fm.Author)
		}
		if !fm.BreakingChange {
			t.Error("expected breaking_change to be true")
		}
		if fm.Deprecated {
			t.Error("expected deprecated to be false")
		}
	})

	t.Run("valid frontmatter with requires_skills populates SkillDep slice", func(t *testing.T) {
		content := makeFrontmatter("type: agent\nrequires_skills:\n  - file: skills/foo.md\n  - file: skills/bar.md\n    pin: abc123")
		fm, err := ParseFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fm.RequiresSkills) != 2 {
			t.Fatalf("expected 2 skill deps, got %d", len(fm.RequiresSkills))
		}
		if fm.RequiresSkills[0].File != "skills/foo.md" {
			t.Errorf("expected file %q, got %q", "skills/foo.md", fm.RequiresSkills[0].File)
		}
		if fm.RequiresSkills[1].File != "skills/bar.md" {
			t.Errorf("expected file %q, got %q", "skills/bar.md", fm.RequiresSkills[1].File)
		}
		if fm.RequiresSkills[1].Pin != "abc123" {
			t.Errorf("expected pin %q, got %q", "abc123", fm.RequiresSkills[1].Pin)
		}
	})

	t.Run("missing --- markers returns error", func(t *testing.T) {
		content := []byte("type: agent\ndescription: no markers")
		_, err := ParseFrontmatter(content)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("requires_skills path with .. returns error", func(t *testing.T) {
		content := makeFrontmatter("type: agent\nrequires_skills:\n  - file: ../escape.md")
		_, err := ParseFrontmatter(content)
		if err == nil {
			t.Fatal("expected error for path traversal, got nil")
		}
		if !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("expected path traversal error, got: %v", err)
		}
	})

	t.Run("requires_skills with absolute path returns error", func(t *testing.T) {
		content := makeFrontmatter("type: agent\nrequires_skills:\n  - file: /etc/passwd")
		_, err := ParseFrontmatter(content)
		if err == nil {
			t.Fatal("expected error for absolute path, got nil")
		}
		if !strings.Contains(err.Error(), "absolute path") {
			t.Errorf("expected absolute path error, got: %v", err)
		}
	})

	t.Run("mct_ref, mct_version, mct_market, mct_checksum are parsed if present", func(t *testing.T) {
		content := makeFrontmatter("type: agent\nmct_ref: mymarket/agents/foo.md\nmct_version: v1.2.3\nmct_market: mymarket\nmct_checksum: abc123def456")
		fm, err := ParseFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fm.MctRef != "mymarket/agents/foo.md" {
			t.Errorf("expected mct_ref %q, got %q", "mymarket/agents/foo.md", fm.MctRef)
		}
		if fm.MctVersion != "v1.2.3" {
			t.Errorf("expected mct_version %q, got %q", "v1.2.3", fm.MctVersion)
		}
		if fm.MctMarket != "mymarket" {
			t.Errorf("expected mct_market %q, got %q", "mymarket", fm.MctMarket)
		}
		if fm.MctChecksum != "abc123def456" {
			t.Errorf("expected mct_checksum %q, got %q", "abc123def456", fm.MctChecksum)
		}
	})
}

// --- TestParseReadmeFrontmatter ---

func TestParseReadmeFrontmatter(t *testing.T) {
	t.Run("valid readme frontmatter parsed correctly", func(t *testing.T) {
		content := []byte("---\ntags:\n  - golang\n  - dev\ndescription: Go profile\n---\n# Readme\n")
		fm, err := ParseReadmeFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fm.Description != "Go profile" {
			t.Errorf("expected description %q, got %q", "Go profile", fm.Description)
		}
		if len(fm.MctTags) != 2 {
			t.Fatalf("expected 2 tags, got %d", len(fm.MctTags))
		}
		if fm.MctTags[0] != "golang" {
			t.Errorf("expected tag %q, got %q", "golang", fm.MctTags[0])
		}
		if fm.MctTags[1] != "dev" {
			t.Errorf("expected tag %q, got %q", "dev", fm.MctTags[1])
		}
	})

	t.Run("no frontmatter returns error", func(t *testing.T) {
		content := []byte("# Just a README\nNo frontmatter here.\n")
		_, err := ParseReadmeFrontmatter(content)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("valid frontmatter with no tags yields empty/nil MctTags", func(t *testing.T) {
		content := []byte("---\ndescription: No tags here\n---\n# body\n")
		fm, err := ParseReadmeFrontmatter(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fm.MctTags) != 0 {
			t.Errorf("expected empty tags, got %v", fm.MctTags)
		}
	})
}

// --- TestInjectMctFields ---

func TestInjectMctFields(t *testing.T) {
	content := makeFrontmatter("type: agent\ndescription: Test agent\nauthor: bob")

	ref := MctRef("mymarket/agents/test.md")
	version := MctVersion("v2.0.0")
	market := "mymarket"
	checksum := "sha256abc"

	result, err := InjectMctFields(content, ref, version, market, checksum)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fm, err := ParseFrontmatter(result)
	if err != nil {
		t.Fatalf("re-parse error: %v", err)
	}

	if fm.MctRef != ref {
		t.Errorf("expected mct_ref %q, got %q", ref, fm.MctRef)
	}
	if fm.MctVersion != version {
		t.Errorf("expected mct_version %q, got %q", version, fm.MctVersion)
	}
	if fm.MctMarket != market {
		t.Errorf("expected mct_market %q, got %q", market, fm.MctMarket)
	}
	if fm.MctChecksum != checksum {
		t.Errorf("expected mct_checksum %q, got %q", checksum, fm.MctChecksum)
	}
}

// --- TestPatchMctVersion ---

func TestPatchMctVersion(t *testing.T) {
	t.Run("patches existing mct_version field", func(t *testing.T) {
		content := makeFrontmatter("type: agent\nmct_version: v1.0.0")
		result, err := PatchMctVersion(content, "v2.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(result), "mct_version: v2.0.0") {
			t.Errorf("expected mct_version: v2.0.0 in result, got:\n%s", string(result))
		}
		if strings.Contains(string(result), "mct_version: v1.0.0") {
			t.Error("old mct_version should not be present after patch")
		}
	})

	t.Run("content without mct_version returns error", func(t *testing.T) {
		content := makeFrontmatter("type: agent\ndescription: no version field")
		_, err := PatchMctVersion(content, "v1.0.0")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// --- TestPatchMctChecksum ---

func TestPatchMctChecksum(t *testing.T) {
	t.Run("patches existing mct_checksum field", func(t *testing.T) {
		content := makeFrontmatter("type: agent\nmct_checksum: oldchecksum")
		result, err := PatchMctChecksum(content, "newchecksum")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(result), "mct_checksum: newchecksum") {
			t.Errorf("expected mct_checksum: newchecksum in result, got:\n%s", string(result))
		}
		if strings.Contains(string(result), "mct_checksum: oldchecksum") {
			t.Error("old mct_checksum should not be present after patch")
		}
	})

	t.Run("content without mct_checksum returns error", func(t *testing.T) {
		content := makeFrontmatter("type: agent\ndescription: no checksum field")
		_, err := PatchMctChecksum(content, "somechecksum")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// --- TestValidateSkillDepPath (via ParseFrontmatter) ---

func TestValidateSkillDepPath(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		wantErr bool
	}{
		{
			name:    "valid relative path",
			file:    "skills/foo.md",
			wantErr: false,
		},
		{
			name:    "path traversal with ..",
			file:    "../escape.md",
			wantErr: true,
		},
		{
			name:    "absolute path",
			file:    "/abs/path.md",
			wantErr: true,
		},
		{
			name:    "path with space (invalid chars)",
			file:    "has space.md",
			wantErr: true,
		},
		{
			name:    "empty path",
			file:    "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var skillsField string
			if tc.file == "" {
				// yaml will parse an empty string for "file:"
				skillsField = "requires_skills:\n  - file: \"\""
			} else {
				skillsField = "requires_skills:\n  - file: " + tc.file
			}
			content := makeFrontmatter("type: agent\n" + skillsField)
			_, err := ParseFrontmatter(content)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for file=%q, got nil", tc.file)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for file=%q: %v", tc.file, err)
			}
		})
	}
}
