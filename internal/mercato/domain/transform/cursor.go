package transform

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

// CursorTransformer converts skills to Cursor .mdc rule files.
type CursorTransformer struct{}

func (t *CursorTransformer) ToolName() string { return "cursor" }

func (t *CursorTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *CursorTransformer) OutputPath(entry domain.Entry) string {
	return ".cursor/rules/" + entryName(entry) + ".mdc"
}

func (t *CursorTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "cursor does not support agents",
		}
	}

	desc := parseDescription(content)
	body := extractBody(content)

	fm := buildFrontmatter([][2]string{
		{"alwaysApply", "false"},
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
