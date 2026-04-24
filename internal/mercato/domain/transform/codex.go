package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// CodexTransformer converts skills to Codex format with directory-based output.
type CodexTransformer struct{}

func (t *CodexTransformer) ToolName() string { return "codex" }

func (t *CodexTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *CodexTransformer) OutputPath(entry domain.Entry) string {
	return ".codex/skills/" + entryName(entry) + "/SKILL.md"
}

func (t *CodexTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "codex does not support agents",
			Warnings:   []string{"codex does not support agents, skipping " + entryName(entry)},
		}
	}

	desc := parseDescription(content)
	body := extractBody(content)
	name := entryName(entry)

	fm := buildFrontmatter([][2]string{
		{"name", name},
		{"description", desc},
	})

	var out []byte
	out = append(out, fm...)
	out = append(out, body...)

	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    out,
		OutputPath: t.OutputPath(entry),
	}
}
