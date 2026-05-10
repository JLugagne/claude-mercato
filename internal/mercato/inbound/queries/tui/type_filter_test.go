package tui

import (
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

func TestNextTypeFilter_CyclesAllTypes(t *testing.T) {
	want := []domain.EntryType{
		domain.EntryTypeAgent,
		domain.EntryTypeSkill,
		domain.EntryTypeCommand,
		domain.EntryTypeHook,
		"",
		domain.EntryTypeAgent,
	}
	cur := domain.EntryType("")
	for i, w := range want {
		cur = nextTypeFilter(cur)
		if cur != w {
			t.Errorf("step %d: got %q, want %q", i, cur, w)
		}
	}
}
