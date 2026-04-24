package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// ContinueTransformer converts skills to Continue .continue/rules/*.md files.
type ContinueTransformer struct{}

func (t *ContinueTransformer) ToolName() string { return "continue" }

func (t *ContinueTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *ContinueTransformer) OutputPath(entry domain.Entry) string {
	return ".continue/rules/" + entryName(entry) + ".md"
}

func (t *ContinueTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "continue does not support individual agents",
		}
	}

	body := extractBody(content)

	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    body,
		OutputPath: t.OutputPath(entry),
	}
}
