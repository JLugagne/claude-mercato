package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
	"github.com/JLugagne/agents-mercato/internal/mercato/outbound/cfgadapter"
)

// commandMarketFiles returns a market with a command that requires a skill,
// and the skill the command depends on.
func commandMarketFiles() map[string]string {
	return map[string]string{
		"dev/go/commands/refactor.md":     "---\ndescription: \"Refactor command\"\nrequires_skills:\n  - file: dev/go/skills/go-arch/SKILL.md\n---\nRefactor the code at hand.\n",
		"dev/go/skills/go-arch/SKILL.md":  "---\ndescription: \"Go architect\"\n---\nArchitect Go applications.\n",
		"dev/go/skills/go-arch/prompt.md": "You are a Go architect.\n",
		"dev/go/README.md":                "---\ntags:\n  - go\n---\nGo profile.\n",
	}
}

func TestIntegration_AddCommand(t *testing.T) {
	application, projectDir, _ := setupIntegration(t, commandMarketFiles())

	ref := domain.MctRef(marketName + "@dev/go/commands/refactor.md")
	if _, err := application.Add(ref, service.AddOpts{}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify command file written to .claude/commands/.
	cmdPath := filepath.Join(projectDir, ".claude", "commands", "refactor.md")
	got, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("read installed command: %v", err)
	}
	if len(got) == 0 {
		t.Errorf("expected command content, got empty")
	}

	// Verify required skill was resolved and installed.
	skillPath := filepath.Join(projectDir, ".claude", "skills", "go-arch", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf("expected dependency skill go-arch/SKILL.md to be installed: %v", err)
	}

	// Verify installdb records the command.
	db, err := cfgadapter.NewInstallDB().Load(application.cacheDir)
	if err != nil {
		t.Fatalf("load installdb: %v", err)
	}
	im := db.FindMarket(marketName)
	if im == nil {
		t.Fatal("market not found in installdb")
	}
	found := false
	for _, pkg := range im.Packages {
		for _, c := range pkg.Files.Commands {
			if c == "refactor.md" {
				found = true
			}
		}
	}
	if !found {
		t.Error("command refactor.md not found in installdb")
	}
}

func TestIntegration_CheckCleanCommand(t *testing.T) {
	application, _, _ := setupIntegration(t, commandMarketFiles())
	ref := domain.MctRef(marketName + "@dev/go/commands/refactor.md")
	if _, err := application.Add(ref, service.AddOpts{}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	statuses, err := application.Check(service.CheckOpts{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var found *domain.EntryStatus
	for i := range statuses {
		if statuses[i].Ref == ref {
			found = &statuses[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("command ref %q not present in Check results: %+v", ref, statuses)
	}
	if found.State != domain.StateClean {
		t.Errorf("expected StateClean for command, got %v", found.State)
	}
}

func TestIntegration_RemoveCommand(t *testing.T) {
	application, projectDir, _ := setupIntegration(t, commandMarketFiles())
	ref := domain.MctRef(marketName + "@dev/go/commands/refactor.md")
	if _, err := application.Add(ref, service.AddOpts{}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := application.Remove(ref, service.RemoveOpts{}); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	cmdPath := filepath.Join(projectDir, ".claude", "commands", "refactor.md")
	if _, err := os.Stat(cmdPath); !os.IsNotExist(err) {
		t.Errorf("expected command file removed, stat err = %v", err)
	}

	db, err := cfgadapter.NewInstallDB().Load(application.cacheDir)
	if err != nil {
		t.Fatalf("load installdb: %v", err)
	}
	im := db.FindMarket(marketName)
	if im != nil {
		for _, pkg := range im.Packages {
			for _, c := range pkg.Files.Commands {
				if c == "refactor.md" {
					t.Error("command still present in installdb after remove")
				}
			}
		}
	}
}

func TestIntegration_ListIncludesCommands(t *testing.T) {
	application, _, _ := setupIntegration(t, commandMarketFiles())
	ref := domain.MctRef(marketName + "@dev/go/commands/refactor.md")
	if _, err := application.Add(ref, service.AddOpts{}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	all, err := application.List(service.ListOpts{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	cmdCount := 0
	for _, e := range all {
		if e.Type == domain.EntryTypeCommand && e.Filename == "refactor.md" {
			cmdCount++
		}
	}
	if cmdCount != 1 {
		t.Errorf("expected exactly 1 command in unfiltered List, got %d", cmdCount)
	}

	onlyCmds, err := application.List(service.ListOpts{Type: domain.EntryTypeCommand})
	if err != nil {
		t.Fatalf("List Type=command: %v", err)
	}
	if len(onlyCmds) != 1 || onlyCmds[0].Type != domain.EntryTypeCommand {
		t.Errorf("expected exactly 1 command-typed entry, got %v", onlyCmds)
	}

	onlyAgents, err := application.List(service.ListOpts{Type: domain.EntryTypeAgent})
	if err != nil {
		t.Fatalf("List Type=agent: %v", err)
	}
	for _, e := range onlyAgents {
		if e.Type == domain.EntryTypeCommand {
			t.Errorf("agent filter returned a command: %v", e)
		}
	}
}
