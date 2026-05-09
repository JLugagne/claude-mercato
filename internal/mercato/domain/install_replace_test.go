package domain

import (
	"reflect"
	"testing"
)

// TestAddOrUpdatePackage_ReplacesPackageFiles verifies that re-installing
// a package over an existing one REPLACES pkg.Files instead of growing
// the list — the package-level lists are derived indices, not accumulators.
// Acceptance test for issue #3 case "Files.Skills no longer grows monotonically".
func TestAddOrUpdatePackage_ReplacesPackageFiles(t *testing.T) {
	var db InstallDatabase

	db.AddOrUpdatePackage("acme", "p", "v1",
		InstalledFiles{Skills: []string{"foo", "bar"}},
		InstalledLocation{Path: "/proj", Type: RuntimeTypeClaudeCode},
	)
	db.AddOrUpdatePackage("acme", "p", "v2",
		InstalledFiles{Skills: []string{"foo"}}, // bar dropped upstream
		InstalledLocation{Path: "/proj", Type: RuntimeTypeClaudeCode},
	)

	pkg := db.FindPackage("acme", "p")
	if !reflect.DeepEqual(pkg.Files.Skills, []string{"foo"}) {
		t.Fatalf("pkg.Files.Skills should be replaced, got %v", pkg.Files.Skills)
	}
}

// TestAddOrUpdatePackage_ReplacesLocationFiles verifies the per-location
// Files []InstalledFile list is replaced (not merged) so removed files
// disappear from the DB.
func TestAddOrUpdatePackage_ReplacesLocationFiles(t *testing.T) {
	var db InstallDatabase

	db.AddOrUpdatePackage("acme", "p", "v1",
		InstalledFiles{Skills: []string{"foo"}},
		InstalledLocation{
			Path: "/proj", Type: RuntimeTypeClaudeCode,
			Files: []InstalledFile{
				{Path: ".claude/skills/foo/SKILL.md", XXH: "h1"},
				{Path: ".claude/skills/foo/references/old.md", XXH: "h2"},
			},
		},
	)
	db.AddOrUpdatePackage("acme", "p", "v2",
		InstalledFiles{Skills: []string{"foo"}},
		InstalledLocation{
			Path: "/proj", Type: RuntimeTypeClaudeCode,
			Files: []InstalledFile{
				{Path: ".claude/skills/foo/SKILL.md", XXH: "h1new"},
			},
		},
	)

	loc := db.FindPackage("acme", "p").FindLocation("/proj")
	want := []InstalledFile{{Path: ".claude/skills/foo/SKILL.md", XXH: "h1new"}}
	if !reflect.DeepEqual(loc.Files, want) {
		t.Fatalf("location files should be replaced; got %+v want %+v", loc.Files, want)
	}
}

// TestAddOrUpdatePackage_DistinctRuntimeTypesCoexist confirms that the
// replace-wholesale semantic is keyed on (Path, Type), so installing the
// same package for cursor doesn't wipe the claude-code location.
func TestAddOrUpdatePackage_DistinctRuntimeTypesCoexist(t *testing.T) {
	var db InstallDatabase

	db.AddOrUpdatePackage("acme", "p", "v1",
		InstalledFiles{Skills: []string{"foo"}},
		InstalledLocation{
			Path: "/proj", Type: RuntimeTypeClaudeCode,
			Files: []InstalledFile{{Path: ".claude/skills/foo/SKILL.md", XXH: "c1"}},
		},
	)
	db.AddOrUpdatePackage("acme", "p", "v1",
		InstalledFiles{Skills: []string{"foo"}},
		InstalledLocation{
			Path: "/proj", Type: "cursor",
			Files: []InstalledFile{{Path: ".cursor/rules/foo.mdc", XXH: "k1"}},
		},
	)

	pkg := db.FindPackage("acme", "p")
	if len(pkg.Locations) != 2 {
		t.Fatalf("expected 2 locations (claude-code + cursor), got %d", len(pkg.Locations))
	}
}

// TestMergeLocationFiles_ReplacesHash exercises the helper that callers
// installing single entries use to compose the full set before calling
// AddOrUpdatePackage.
func TestMergeLocationFiles_ReplacesHash(t *testing.T) {
	existing := []InstalledFile{
		{Path: "a", XXH: "h1"},
		{Path: "b", XXH: "h2"},
	}
	incoming := []InstalledFile{
		{Path: "b", XXH: "h2new"}, // re-write
		{Path: "c", XXH: "h3"},    // new
	}
	got := MergeLocationFiles(existing, incoming)
	want := []InstalledFile{
		{Path: "a", XXH: "h1"},
		{Path: "b", XXH: "h2new"},
		{Path: "c", XXH: "h3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

// TestMergePackageFiles_DedupesLeafNames verifies the leaf-name merger
// used by single-entry adds.
func TestMergePackageFiles_DedupesLeafNames(t *testing.T) {
	got := MergePackageFiles(
		InstalledFiles{Skills: []string{"a", "b"}, Agents: []string{"x"}},
		InstalledFiles{Skills: []string{"b", "c"}, Agents: []string{"x", "y"}},
	)
	want := InstalledFiles{Skills: []string{"a", "b", "c"}, Agents: []string{"x", "y"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
