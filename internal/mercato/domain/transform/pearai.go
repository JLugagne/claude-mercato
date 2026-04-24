package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// PearAITransformer converts skills to PearAI .peairules files.
type PearAITransformer struct{}

func (t *PearAITransformer) ToolName() string { return "pearai" }

func (t *PearAITransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *PearAITransformer) OutputPath(entry domain.Entry) string {
	return ".peairules"
}

func (t *PearAITransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "pearai does not support agents",
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
