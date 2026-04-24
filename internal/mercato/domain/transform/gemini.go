package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// GeminiTransformer strips all frontmatter and outputs only the markdown body.
type GeminiTransformer struct{}

func (t *GeminiTransformer) ToolName() string { return "gemini" }

func (t *GeminiTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *GeminiTransformer) OutputPath(entry domain.Entry) string {
	return ".gemini/rules/" + entryName(entry) + ".md"
}

func (t *GeminiTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "gemini does not support agents",
		}
	}

	body := extractBody(content)

	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    body,
		OutputPath: t.OutputPath(entry),
	}
}
