package transform

import (
	"strings"
	"testing"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

func TestOpenCodeTransformer(t *testing.T) {
	tr := &OpenCodeTransformer{}

	t.Run("ToolName", func(t *testing.T) {
		if got := tr.ToolName(); got != "opencode" {
			t.Errorf("ToolName() = %q, want %q", got, "opencode")
		}
	})

	t.Run("SupportsEntry", func(t *testing.T) {
		if !tr.SupportsEntry(domain.EntryTypeAgent) {
			t.Error("should support agents")
		}
		if !tr.SupportsEntry(domain.EntryTypeSkill) {
			t.Error("should support skills")
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
				want:  ".opencode/agents/my-agent.md",
			},
			{
				name:  "skill",
				entry: domain.Entry{Filename: "my-skill", Type: domain.EntryTypeSkill},
				want:  ".opencode/rules/my-skill.md",
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
		mappings := domain.ToolMapping{
			Models: map[string]map[string]string{
				"claude-sonnet-4-20250514": {"opencode": "anthropic/claude-sonnet-4-20250514"},
				"claude-haiku":             {"opencode": ""},
			},
			Tools: map[string]map[string]string{
				"Read":  {"opencode": "read"},
				"Edit":  {"opencode": "edit"},
				"Bash":  {"opencode": "bash"},
				"Write": {"opencode": "write"},
				"Glob":  {"opencode": "glob"},
				"Grep":  {"opencode": "grep"},
			},
		}

		tests := []struct {
			name      string
			entry     domain.Entry
			content   string
			mappings  domain.ToolMapping
			checkBody func(t *testing.T, result domain.TransformResult)
		}{
			{
				name:     "agent with mapped model and tools",
				entry:    domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				content:  "---\ndescription: My agent\nmodel: claude-sonnet-4-20250514\ntools:\n  - Read\n  - Edit\n  - Bash\n---\nAgent body",
				mappings: mappings,
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					content := string(result.Content)
					if !strings.Contains(content, "description: My agent") {
						t.Error("missing description")
					}
					if !strings.Contains(content, "mode: primary") {
						t.Error("missing mode: primary")
					}
					if !strings.Contains(content, "model: anthropic/claude-sonnet-4-20250514") {
						t.Error("missing mapped model")
					}
					if !strings.Contains(content, "tools: read, edit, bash") {
						t.Errorf("missing mapped tools, got: %s", content)
					}
					if !strings.Contains(content, "Agent body") {
						t.Error("missing body")
					}
					if len(result.Warnings) != 0 {
						t.Errorf("unexpected warnings: %v", result.Warnings)
					}
				},
			},
			{
				name:     "agent with unmapped model generates warning",
				entry:    domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				content:  "---\ndescription: My agent\nmodel: gpt-4o\n---\nAgent body",
				mappings: mappings,
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					content := string(result.Content)
					if strings.Contains(content, "model:") {
						t.Error("unmapped model should be stripped")
					}
					if len(result.Warnings) == 0 {
						t.Error("expected warning for unmapped model")
					}
					if !strings.Contains(result.Warnings[0], "unmapped model") {
						t.Errorf("warning = %q, want to contain 'unmapped model'", result.Warnings[0])
					}
				},
			},
			{
				name:     "agent with model mapping to empty string strips silently",
				entry:    domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				content:  "---\ndescription: My agent\nmodel: claude-haiku\n---\nAgent body",
				mappings: mappings,
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					content := string(result.Content)
					if strings.Contains(content, "model:") {
						t.Error("empty-mapped model should be stripped")
					}
					if len(result.Warnings) != 0 {
						t.Errorf("should strip silently, got warnings: %v", result.Warnings)
					}
				},
			},
			{
				name:     "agent with no model",
				entry:    domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				content:  "---\ndescription: My agent\n---\nAgent body",
				mappings: mappings,
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					content := string(result.Content)
					if strings.Contains(content, "model:") {
						t.Error("should not have model field when none in source")
					}
				},
			},
			{
				name:     "skill transform",
				entry:    domain.Entry{Filename: "my-skill", Type: domain.EntryTypeSkill},
				content:  "---\ndescription: A great skill\nauthor: me\nmct_ref: foo@bar\n---\nSkill body here",
				mappings: mappings,
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					content := string(result.Content)
					if !strings.Contains(content, "description: A great skill") {
						t.Error("missing description")
					}
					if !strings.Contains(content, "Skill body here") {
						t.Error("missing body")
					}
					if strings.Contains(content, "mct_ref") {
						t.Error("should not contain mct fields")
					}
					if result.OutputPath != ".opencode/rules/my-skill.md" {
						t.Errorf("OutputPath = %q", result.OutputPath)
					}
				},
			},
			{
				name:     "agent with nil mappings",
				entry:    domain.Entry{Filename: "my-agent.md", Type: domain.EntryTypeAgent},
				content:  "---\ndescription: My agent\nmodel: claude-sonnet-4-20250514\n---\nBody",
				mappings: domain.ToolMapping{},
				checkBody: func(t *testing.T, result domain.TransformResult) {
					t.Helper()
					if len(result.Warnings) == 0 {
						t.Error("expected warning for unmapped model with nil mappings")
					}
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tr.Transform(tt.entry, []byte(tt.content), tt.mappings)
				if result.Skipped {
					t.Error("opencode should not skip any entry")
				}
				if tt.checkBody != nil {
					tt.checkBody(t, result)
				}
			})
		}
	})
}
