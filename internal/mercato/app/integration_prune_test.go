package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
	"github.com/JLugagne/agents-mercato/internal/mercato/outbound/cfgadapter"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// removeFromRepo removes the given paths from the repo and creates a new commit.
func removeFromRepo(t *testing.T, repoDir string, paths []string, msg string) {
	t.Helper()
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if _, err := wt.Remove(p); err != nil {
			t.Fatalf("worktree remove %s: %v", p, err)
		}
	}
	if _, err = wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
}

// fetchClone runs a full fetch on the cached clone without touching sync state,
// so the next Update() sees diffs from the recorded SHA to HEAD.
func fetchClone(t *testing.T, app *App) {
	t.Helper()
	cloneDir := filepath.Join(app.cacheDir, marketDirName(marketName))
	repo, err := git.PlainOpen(cloneDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(&git.FetchOptions{}); err != nil && err != git.NoErrAlreadyUpToDate {
		t.Fatalf("fetch: %v", err)
	}
}

// TestIntegration_UpdatePrunesRemovedSkillFile is the issue #3 scenario:
// v1 of skill go-arch ships SKILL.md + prompt.md. v2 removes prompt.md.
// After Update, prompt.md should be gone from disk AND from the installdb
// location.Files list. This was previously broken — copyUpdatedFiles wrote
// the new file set but never deleted orphans.
func TestIntegration_UpdatePrunesRemovedSkillFile(t *testing.T) {
	app, projectDir, sourceDir := setupIntegration(t, marketFiles())

	ref := domain.MctRef(marketName + "@dev/go/skills/go-arch/SKILL.md")
	if _, err := app.Add(ref, service.AddOpts{}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	skillDir := filepath.Join(projectDir, ".claude", "skills", "go-arch")
	for _, f := range []string{"SKILL.md", "prompt.md"} {
		if _, err := os.Stat(filepath.Join(skillDir, f)); err != nil {
			t.Fatalf("expected %s present after Add: %v", f, err)
		}
	}

	// Upstream: drop prompt.md from the skill, modify SKILL.md.
	removeFromRepo(t, sourceDir, []string{"dev/go/skills/go-arch/prompt.md"}, "drop prompt")
	addCommitToRepo(t, sourceDir, map[string]string{
		"dev/go/skills/go-arch/SKILL.md": "---\ndescription: \"Go architect v2\"\n---\nv2 body.\n",
	}, "update skill")

	fetchClone(t, app)

	if _, err := app.Update(service.UpdateOpts{AllMerge: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md should still be present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "prompt.md")); !os.IsNotExist(err) {
		t.Fatalf("prompt.md should have been pruned, got err=%v", err)
	}

	db, err := cfgadapter.NewInstallDB().Load(app.cacheDir)
	if err != nil {
		t.Fatalf("load installdb: %v", err)
	}
	pkg := db.FindPackage(marketName, "dev/go")
	if pkg == nil {
		t.Fatal("package not found")
	}
	loc := pkg.FindLocation(projectDir)
	if loc == nil {
		t.Fatal("location not found")
	}
	for _, f := range loc.Files {
		if filepath.Base(f.Path) == "prompt.md" {
			t.Fatalf("prompt.md should be gone from installdb Files, still present: %+v", f)
		}
	}
}

// TestIntegration_UpdateReplacesPackageFiles verifies issue #3 acceptance:
// re-installing/updating must not grow pkg.Files monotonically.
// After updating to a version with a single skill, pkg.Files.Skills should
// contain just that skill — not the union of all historically installed.
func TestIntegration_UpdateReplacesPackageFiles(t *testing.T) {
	// Start with a market that has TWO skills in dev/go.
	files := marketFiles()
	files["dev/go/skills/extra/SKILL.md"] = "---\ndescription: extra\n---\nbody\n"
	app, _, sourceDir := setupIntegration(t, files)

	// Install the whole profile so both skills land in pkg.Files.Skills.
	if _, err := app.Add(domain.MctRef(marketName+"@dev/go"), service.AddOpts{}); err != nil {
		t.Fatalf("Add profile: %v", err)
	}

	db, err := cfgadapter.NewInstallDB().Load(app.cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	pkg := db.FindPackage(marketName, "dev/go")
	if pkg == nil || len(pkg.Files.Skills) < 2 {
		t.Fatalf("expected >=2 skills after profile install, got %+v", pkg)
	}

	// Upstream: drop the "extra" skill entirely.
	removeFromRepo(t, sourceDir,
		[]string{"dev/go/skills/extra/SKILL.md"},
		"drop extra skill",
	)
	// Touch go-arch so Update has work to do for the package.
	addCommitToRepo(t, sourceDir, map[string]string{
		"dev/go/skills/go-arch/SKILL.md": "---\ndescription: arch v2\n---\nv2\n",
	}, "bump")

	fetchClone(t, app)
	if _, err := app.Update(service.UpdateOpts{AllMerge: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	db2, err := cfgadapter.NewInstallDB().Load(app.cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	pkg2 := db2.FindPackage(marketName, "dev/go")
	for _, s := range pkg2.Files.Skills {
		if s == "extra" {
			t.Fatalf("pkg.Files.Skills should no longer contain dropped skill 'extra', got %v", pkg2.Files.Skills)
		}
	}
}

// TestIntegration_UpdateRenamedFile covers the rename-across-versions case
// in issue #3: v1 ships skills/go-arch/prompt.md; v2 renames it to
// skills/go-arch/lib/prompt.md. Update must delete the old path, write the
// new one, and the installdb must reflect only the new path.
//
// Note: the current installer flattens nested files into the skill root
// (writes via filepath.Base), so a true subdirectory rename ends up at the
// root with the new basename. We just verify the rename is observed: the
// old basename is gone and the new basename is present, both on disk and
// in the location's Files list.
func TestIntegration_UpdateRenamedFile(t *testing.T) {
	app, projectDir, sourceDir := setupIntegration(t, marketFiles())

	if _, err := app.Add(domain.MctRef(marketName+"@dev/go/skills/go-arch/SKILL.md"), service.AddOpts{}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Upstream: rename prompt.md → guidance.md.
	removeFromRepo(t, sourceDir, []string{"dev/go/skills/go-arch/prompt.md"}, "drop prompt")
	addCommitToRepo(t, sourceDir, map[string]string{
		"dev/go/skills/go-arch/guidance.md": "You are a Go architect (renamed).\n",
	}, "rename to guidance")

	fetchClone(t, app)
	if _, err := app.Update(service.UpdateOpts{AllMerge: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	skillDir := filepath.Join(projectDir, ".claude", "skills", "go-arch")
	if _, err := os.Stat(filepath.Join(skillDir, "prompt.md")); !os.IsNotExist(err) {
		t.Errorf("old prompt.md should be pruned after rename, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "guidance.md")); err != nil {
		t.Errorf("new guidance.md should be present, err=%v", err)
	}

	db, _ := cfgadapter.NewInstallDB().Load(app.cacheDir)
	loc := db.FindPackage(marketName, "dev/go").FindLocation(projectDir)
	hasOld, hasNew := false, false
	for _, f := range loc.Files {
		if filepath.Base(f.Path) == "prompt.md" {
			hasOld = true
		}
		if filepath.Base(f.Path) == "guidance.md" {
			hasNew = true
		}
	}
	if hasOld {
		t.Error("installdb still references old prompt.md after rename")
	}
	if !hasNew {
		t.Error("installdb missing new guidance.md after rename")
	}
}
