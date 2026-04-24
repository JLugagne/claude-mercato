package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// WindsurfTransformer converts skills to Windsurf rule files.
type WindsurfTransformer struct{}

func (t *WindsurfTransformer) ToolName() string { return "windsurf" }

func (t *WindsurfTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *WindsurfTransformer) OutputPath(entry domain.Entry) string {
	return ".windsurf/rules/" + entryName(entry) + ".md"
}

func (t *WindsurfTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "windsurf does not support agents",
		}
	}

	desc := parseDescription(content)
	body := extractBody(content)

	fm := buildFrontmatter([][2]string{
		{"trigger", "model_decision"},
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
