package transform

import (
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/stretchr/testify/assert"
)

func TestSupermavenTransformer(t *testing.T) {
	tr := &SupermavenTransformer{}
	entry := domain.Entry{RelPath: "test-skill", Type: domain.EntryTypeSkill}
	content := []byte("---\ndescription: test desc\n---\nbody content")

	assert.Equal(t, "supermaven", tr.ToolName())
	assert.True(t, tr.SupportsEntry(domain.EntryTypeSkill))
	assert.Equal(t, ".supermavenrules", tr.OutputPath(entry))

	res := tr.Transform(entry, content, domain.ToolMapping{})
	assert.False(t, res.Skipped)
	assert.Contains(t, string(res.Content), "# test-skill")
	assert.Contains(t, string(res.Content), "body content")
}

func TestPearAITransformer(t *testing.T) {
	tr := &PearAITransformer{}
	entry := domain.Entry{RelPath: "test-skill", Type: domain.EntryTypeSkill}
	content := []byte("---\ndescription: test desc\n---\nbody content")

	assert.Equal(t, "pearai", tr.ToolName())
	assert.True(t, tr.SupportsEntry(domain.EntryTypeSkill))
	assert.Equal(t, ".peairules", tr.OutputPath(entry))

	res := tr.Transform(entry, content, domain.ToolMapping{})
	assert.False(t, res.Skipped)
	assert.Contains(t, string(res.Content), "# test-skill")
	assert.Contains(t, string(res.Content), "body content")
}

func TestRooCodeTransformer(t *testing.T) {
	tr := &RooCodeTransformer{}
	entry := domain.Entry{RelPath: "test-skill", Type: domain.EntryTypeSkill}
	content := []byte("---\ndescription: test desc\n---\nbody content")

	assert.Equal(t, "roocode", tr.ToolName())
	assert.True(t, tr.SupportsEntry(domain.EntryTypeSkill))
	assert.Equal(t, ".roocode.rules", tr.OutputPath(entry))

	res := tr.Transform(entry, content, domain.ToolMapping{})
	assert.False(t, res.Skipped)
	assert.Contains(t, string(res.Content), "# test-skill")
	assert.Contains(t, string(res.Content), "body content")
}

func TestContinueTransformer(t *testing.T) {
	tr := &ContinueTransformer{}
	entry := domain.Entry{RelPath: "test-skill", Type: domain.EntryTypeSkill}
	content := []byte("---\ndescription: test desc\n---\nbody content")

	assert.Equal(t, "continue", tr.ToolName())
	assert.True(t, tr.SupportsEntry(domain.EntryTypeSkill))
	assert.Contains(t, tr.OutputPath(entry), ".continue/rules/")

	res := tr.Transform(entry, content, domain.ToolMapping{})
	assert.False(t, res.Skipped)
	assert.Equal(t, "body content", string(res.Content))
}
