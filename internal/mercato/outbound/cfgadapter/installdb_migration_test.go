package cfgadapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

func TestInstallDB_MigrateV1ToV2(t *testing.T) {
	dir := t.TempDir()

	// Stage a project tree with a .claude/agents/foo.md file present so the
	// migrator can hash it.
	project := filepath.Join(dir, "project")
	claudeAgents := filepath.Join(project, ".claude", "agents")
	if err := os.MkdirAll(claudeAgents, 0755); err != nil {
		t.Fatal(err)
	}
	agentContent := []byte("# foo agent\n")
	if err := os.WriteFile(filepath.Join(claudeAgents, "foo.md"), agentContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Write a v1-format installed.json (no schema_version, Locations as []string).
	v1 := []byte(`{
  "markets": [
    {
      "market": "acme",
      "packages": [
        {
          "profile": "agents/foo.md",
          "version": "abc123",
          "files": {"agents": ["foo.md"]},
          "locations": ["` + project + `"]
        }
      ]
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(dir, "installed.json"), v1, 0644); err != nil {
		t.Fatal(err)
	}

	adapter := NewInstallDB()
	db, err := adapter.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if db.SchemaVersion != domain.InstallSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", db.SchemaVersion, domain.InstallSchemaVersion)
	}
	if len(db.Markets) != 1 {
		t.Fatalf("Markets len = %d, want 1", len(db.Markets))
	}
	pkg := db.FindPackage("acme", "agents/foo.md")
	if pkg == nil {
		t.Fatal("expected package after migration")
	}
	if len(pkg.Locations) != 1 {
		t.Fatalf("Locations len = %d, want 1", len(pkg.Locations))
	}
	loc := pkg.Locations[0]
	if loc.Path != project {
		t.Errorf("Path = %q, want %q", loc.Path, project)
	}
	if loc.Type != domain.RuntimeTypeClaudeCode {
		t.Errorf("Type = %q, want %q", loc.Type, domain.RuntimeTypeClaudeCode)
	}
	if len(loc.Files) != 1 {
		t.Fatalf("Files len = %d, want 1", len(loc.Files))
	}
	if loc.Files[0].Path != ".claude/agents/foo.md" {
		t.Errorf("Files[0].Path = %q, want %q", loc.Files[0].Path, ".claude/agents/foo.md")
	}
	if loc.Files[0].XXH == "" {
		t.Error("Files[0].XXH should be populated from on-disk content")
	}

	// Verify the migrated DB was persisted.
	persisted, err := os.ReadFile(filepath.Join(dir, "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(persisted, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.SchemaVersion != domain.InstallSchemaVersion {
		t.Errorf("persisted schema_version = %d, want %d", probe.SchemaVersion, domain.InstallSchemaVersion)
	}
}

func TestInstallDB_V2RoundTrip(t *testing.T) {
	dir := t.TempDir()
	adapter := NewInstallDB()

	in := domain.InstallDatabase{
		SchemaVersion: domain.InstallSchemaVersion,
		Markets: []domain.InstalledMarket{
			{Market: "acme", Packages: []domain.InstalledPackage{
				{
					Profile:   "agents/foo.md",
					Version:   "abc",
					Files:     domain.InstalledFiles{Agents: []string{"foo.md"}},
					Locations: []domain.InstalledLocation{{Path: "/p", Type: domain.RuntimeTypeClaudeCode, Files: []domain.InstalledFile{{Path: ".claude/agents/foo.md", XXH: "deadbeef"}}}},
				},
			}},
		},
	}
	if err := adapter.Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := adapter.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.SchemaVersion != domain.InstallSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", out.SchemaVersion, domain.InstallSchemaVersion)
	}
	pkg := out.FindPackage("acme", "agents/foo.md")
	if pkg == nil || len(pkg.Locations) != 1 || pkg.Locations[0].Files[0].XXH != "deadbeef" {
		t.Errorf("round-trip lost data: %+v", out)
	}
}
