package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

func cmdWithStdin(stdin string) *cobra.Command {
	c := &cobra.Command{}
	c.SetIn(strings.NewReader(stdin))
	c.SetOut(&bytes.Buffer{})
	c.SetErr(&bytes.Buffer{})
	return c
}

func sample() []service.DeletedFile {
	return []service.DeletedFile{
		{Market: "m", Profile: "p", Location: "/proj", RelPath: ".claude/agents/a.md"},
		{Market: "m", Profile: "p", Location: "/proj", RelPath: ".claude/agents/b.md"},
		{Market: "m", Profile: "p", Location: "/proj", RelPath: ".claude/agents/c.md"},
	}
}

func TestChooseFilesToRestore_None(t *testing.T) {
	got := chooseFilesToRestore(cmdWithStdin(""), sample(), restoreFlags{none: true})
	if got != nil {
		t.Fatalf("expected nil with --restore-none, got %v", got)
	}
}

func TestChooseFilesToRestore_All(t *testing.T) {
	got := chooseFilesToRestore(cmdWithStdin(""), sample(), restoreFlags{all: true})
	if len(got) != 3 {
		t.Fatalf("expected all 3 with --restore-all, got %d", len(got))
	}
}

func TestChooseFilesToRestore_CIDefaultsToNone(t *testing.T) {
	got := chooseFilesToRestore(cmdWithStdin(""), sample(), restoreFlags{ci: true})
	if got != nil {
		t.Fatalf("expected nil under CI mode (no opt-in), got %v", got)
	}
}

func TestChooseFilesToRestore_CIWithAllRestoresAll(t *testing.T) {
	got := chooseFilesToRestore(cmdWithStdin(""), sample(), restoreFlags{ci: true, all: true})
	if len(got) != 3 {
		t.Fatalf("expected all 3 even under CI when --restore-all is set, got %d", len(got))
	}
}

// TestChooseFilesToRestore_PerFile drives the prompt: restore the first,
// keep the second, then "all" on the third (which expands to the rest from
// the current index — here just the third).
func TestChooseFilesToRestore_PerFile(t *testing.T) {
	got := chooseFilesToRestore(cmdWithStdin("r\nk\na\n"), sample(), restoreFlags{})
	if len(got) != 2 {
		t.Fatalf("expected [a.md, c.md], got %v", got)
	}
	if got[0].RelPath != ".claude/agents/a.md" || got[1].RelPath != ".claude/agents/c.md" {
		t.Errorf("unexpected selection: %+v", got)
	}
}

// TestChooseFilesToRestore_NoneEarlyAbort verifies that "n" stops the loop
// and returns whatever was already picked.
func TestChooseFilesToRestore_NoneEarlyAbort(t *testing.T) {
	got := chooseFilesToRestore(cmdWithStdin("r\nn\n"), sample(), restoreFlags{})
	if len(got) != 1 || got[0].RelPath != ".claude/agents/a.md" {
		t.Fatalf("expected [a.md] only, got %v", got)
	}
}

// TestChooseFilesToRestore_EmptyAnswerKeeps verifies that pressing enter
// (empty string) is treated as keep-deleted.
func TestChooseFilesToRestore_EmptyAnswerKeeps(t *testing.T) {
	got := chooseFilesToRestore(cmdWithStdin("\n\n\n"), sample(), restoreFlags{})
	if got != nil {
		t.Fatalf("expected nothing restored when all answers are empty, got %v", got)
	}
}
