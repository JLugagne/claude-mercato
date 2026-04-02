package transform

import (
	"strings"
	"testing"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

func TestGeminiTransformer(t *testing.T) {
	tr := &GeminiTransformer{}

	t.Run("ToolName", func(t *testing.T) {
		if got := tr.ToolName(); got != "gemini" {
			t.Errorf("ToolName() = %q, want %q", got, "gemini")
		}
	})

	t.Run("SupportsEntry", func(t *testing.T) {
		if tr.SupportsEntry(domain.EntryTypeAgent) {
			t.Error("should not support agents")
		}
		if !tr.SupportsEntry(domain.EntryTypeSkill) {
			t.Error("should support skills")
		}
	})

	t.Run("OutputPath", func(t *testing.T) {
		entry := domain.Entry{Filename: "my-skill", Type: domain.EntryTypeSkill}
		want := ".gemini/rules/my-skill.md"
		if got := tr.OutputPath(entry); got != want {
			t.Errorf("OutputPath() = %q, want %q", got, want)
		}
	})

	t.Run("Transform", func(t *testing.T) {
		tests := []struct {
			name       string
			entry      domain.Entry
			content    string
			wantSkip   bool
			wantReason string
			checkBody  func(t *testing.T, result domain.TransformResult)
		}{
			{
				name:       "agent is skipped",
				entry:      domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				content:    "---\ndescription: test\n---\nBody",
				wantSkip:   true,
				wantReason: "gemini does not support agents",
			},
			{
				name:    "skill strips all frontmatter",
				entry:   domain.Entry{Filename: "my-skill", Type: domain.EntryTypeSkill},
				content: "---\ndescription: A great skill\nauthor: me\nmct_ref: foo@bar\n---\nSkill body here",
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					content := string(result.Content)
					if strings.Contains(content, "---") {
						t.Error("should not contain frontmatter delimiters")
					}
					if strings.Contains(content, "description:") {
						t.Error("should not contain any frontmatter fields")
					}
					if !strings.Contains(content, "Skill body here") {
						t.Error("missing body")
					}
				},
			},
			{
				name:    "content with no frontmatter",
				entry:   domain.Entry{Filename: "plain", Type: domain.EntryTypeSkill},
				content: "Just plain content",
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					if string(result.Content) != "Just plain content" {
						t.Errorf("content = %q, want %q", string(result.Content), "Just plain content")
					}
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tr.Transform(tt.entry, []byte(tt.content), domain.ToolMapping{})
				if result.Skipped != tt.wantSkip {
					t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkip)
				}
				if tt.wantReason != "" && result.SkipReason != tt.wantReason {
					t.Errorf("SkipReason = %q, want %q", result.SkipReason, tt.wantReason)
				}
				if tt.checkBody != nil {
					tt.checkBody(t, result)
				}
			})
		}
	})
}
