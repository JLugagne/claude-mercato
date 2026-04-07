package transform

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

// SupermavenTransformer converts skills to Supermaven .supermavenrules files.
type SupermavenTransformer struct{}

func (t *SupermavenTransformer) ToolName() string { return "supermaven" }

func (t *SupermavenTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *SupermavenTransformer) OutputPath(entry domain.Entry) string {
	return ".supermavenrules"
}

func (t *SupermavenTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "supermaven does not support agents",
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
