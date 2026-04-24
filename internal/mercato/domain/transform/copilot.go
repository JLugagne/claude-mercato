package transform

import (
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// CopilotTransformer converts skills to GitHub Copilot instruction files.
// Files are placed in .github/copilot-instructions.md.
// Since Copilot usually supports only one global file, mct will concatenate all installed skills
// into that file or place them in a dedicated folder if it's supported by specific extensions.
// Standard Copilot uses .github/copilot-instructions.md as the main entry.
type CopilotTransformer struct{}

func (t *CopilotTransformer) ToolName() string { return "copilot" }

func (t *CopilotTransformer) SupportsEntry(entryType domain.EntryType) bool {
	return entryType == domain.EntryTypeSkill
}

func (t *CopilotTransformer) OutputPath(entry domain.Entry) string {
	return ".github/copilot-instructions.md"
}

func (t *CopilotTransformer) Transform(entry domain.Entry, content []byte, _ domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return domain.TransformResult{
			ToolName:   t.ToolName(),
			Skipped:    true,
			SkipReason: "copilot does not support individual agents",
		}
	}

	body := extractBody(content)

	// Since Copilot normally uses one file, we prefix the content with the skill name
	var out []byte
	out = append(out, []byte("## "+string(entry.RelPath)+"\n\n")...)
	out = append(out, body...)
	out = append(out, '\n')

	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    out,
		OutputPath: t.OutputPath(entry),
	}
}
