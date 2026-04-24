package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// ClaudeTransformer passes content through unchanged.
type ClaudeTransformer struct{}

func (t *ClaudeTransformer) ToolName() string { return "claude" }

func (t *ClaudeTransformer) SupportsEntry(_ domain.EntryType) bool { return true }

func (t *ClaudeTransformer) OutputPath(entry domain.Entry) string {
	name := entryName(entry)
	if entry.Type == domain.EntryTypeAgent {
		return ".claude/agents/" + name + ".md"
	}
	return ".claude/skills/" + name + "/SKILL.md"
}

func (t *ClaudeTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    content,
		OutputPath: t.OutputPath(entry),
	}
}
