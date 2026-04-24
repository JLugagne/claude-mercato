package transform

import (
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/stretchr/testify/assert"
)

func TestCopilotTransformer(t *testing.T) {
	tr := &CopilotTransformer{}
	entry := domain.Entry{RelPath: "test-skill", Type: domain.EntryTypeSkill}
	content := []byte("---\ndescription: test desc\n---\nbody content")

	t.Run("ToolName", func(t *testing.T) {
		assert.Equal(t, "copilot", tr.ToolName())
	})

	t.Run("SupportsEntry", func(t *testing.T) {
		assert.True(t, tr.SupportsEntry(domain.EntryTypeSkill))
		assert.False(t, tr.SupportsEntry(domain.EntryTypeAgent))
	})

	t.Run("Transform Skill", func(t *testing.T) {
		res := tr.Transform(entry, content, domain.ToolMapping{})
		assert.False(t, res.Skipped)
		assert.Equal(t, ".github/copilot-instructions.md", res.OutputPath)
		assert.Contains(t, string(res.Content), "## test-skill")
		assert.Contains(t, string(res.Content), "body content")
	})

	t.Run("Transform Agent", func(t *testing.T) {
		agent := domain.Entry{RelPath: "test-agent", Type: domain.EntryTypeAgent}
		res := tr.Transform(agent, content, domain.ToolMapping{})
		assert.True(t, res.Skipped)
	})
}
