package transform

import (
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

func TestClaudeTransformer(t *testing.T) {
	tr := &ClaudeTransformer{}

	t.Run("ToolName", func(t *testing.T) {
		if got := tr.ToolName(); got != "claude" {
			t.Errorf("ToolName() = %q, want %q", got, "claude")
		}
	})

	t.Run("SupportsEntry", func(t *testing.T) {
		tests := []struct {
			entryType domain.EntryType
			want      bool
		}{
			{domain.EntryTypeAgent, true},
			{domain.EntryTypeSkill, true},
		}
		for _, tt := range tests {
			if got := tr.SupportsEntry(tt.entryType); got != tt.want {
				t.Errorf("SupportsEntry(%q) = %v, want %v", tt.entryType, got, tt.want)
			}
		}
	})

	t.Run("OutputPath", func(t *testing.T) {
		tests := []struct {
			name  string
			entry domain.Entry
			want  string
		}{
			{
				name:  "agent",
				entry: domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				want:  ".claude/agents/my-agent.md",
			},
			{
				name:  "skill",
				entry: domain.Entry{Filename: "my-skill", Type: domain.EntryTypeSkill},
				want:  ".claude/skills/my-skill/SKILL.md",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tr.OutputPath(tt.entry); got != tt.want {
					t.Errorf("OutputPath() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("Transform", func(t *testing.T) {
		tests := []struct {
			name    string
			entry   domain.Entry
			content string
		}{
			{
				name:    "agent passthrough",
				entry:   domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				content: "---\ndescription: test agent\n---\nAgent body",
			},
			{
				name:    "skill passthrough",
				entry:   domain.Entry{Filename: "my-skill", Type: domain.EntryTypeSkill},
				content: "---\ndescription: test skill\nauthor: me\n---\nSkill body",
			},
			{
				name:    "no frontmatter",
				entry:   domain.Entry{Filename: "plain", Type: domain.EntryTypeSkill},
				content: "Just plain content",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tr.Transform(tt.entry, []byte(tt.content), domain.ToolMapping{})
				if result.Skipped {
					t.Error("expected not skipped")
				}
				if string(result.Content) != tt.content {
					t.Errorf("Transform() content = %q, want %q", string(result.Content), tt.content)
				}
				if result.ToolName != "claude" {
					t.Errorf("ToolName = %q, want %q", result.ToolName, "claude")
				}
			})
		}
	})
}
