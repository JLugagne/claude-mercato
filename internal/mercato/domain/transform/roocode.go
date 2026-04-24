package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// RooCodeTransformer converts skills to Roo Code .roocode.rules files.
type RooCodeTransformer struct{}

func (t *RooCodeTransformer) ToolName() string { return "roocode" }

func (t *RooCodeTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *RooCodeTransformer) OutputPath(entry domain.Entry) string {
	return ".roocode.rules"
}

func (t *RooCodeTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "roocode does not support agents",
		}
	}

	body := extractBody(content)

	var out []byte
	out = append(out, []byte("# "+string(entry.RelPath)+"\n\n")...)
	out = append(out, body...)
	out = append(out, '\n')

	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    out,
		OutputPath: t.OutputPath(entry),
	}
}
