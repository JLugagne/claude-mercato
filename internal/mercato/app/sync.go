package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash/v2"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

// detectDrift compares installed files at a location against the cached clone
// at the version recorded in installdb. Returns list of files that differ.
func (a *App) detectDrift(pkg domain.InstalledPackage, location, clonePath, branch string) []string {
	// Prefer per-location file hashes (v2 schema). Fall back to git-comparison
	// for legacy packages whose locations have no Files list yet.
	loc := pkg.FindLocation(location)
	if loc != nil && len(loc.Files) > 0 && loc.Type == domain.RuntimeTypeClaudeCode {
		var drifted []string
		for _, f := range loc.Files {
			abs := filepath.Join(location, f.Path)
			data, err := os.ReadFile(abs)
			if err != nil {
				drifted = append(drifted, f.Path)
				continue
			}
			if xxhashHex(data) != f.XXH {
				drifted = append(drifted, f.Path)
			}
		}
		return drifted
	}

	var drifted []string
	localBase := filepath.Join(location, ".claude")

	for _, skill := range pkg.Files.Skills {
		// First check if we can read the original SKILL.md at the recorded version.
		// If the original doesn't exist at that ref, skip drift check for this skill.
		repoRelPath := a.skillFileRepoPath(pkg.Profile, skill, "SKILL.md")
		_, origErr := a.git.ReadFileAtRef(clonePath, branch, repoRelPath, pkg.Version)
		if origErr != nil {
			continue
		}

		skillDir := filepath.Join(localBase, "skills", skill)
		entries, err := os.ReadDir(skillDir)
		if err != nil {
			drifted = append(drifted, "skills/"+skill)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fileRepoPath := a.skillFileRepoPath(pkg.Profile, skill, entry.Name())
			originalContent, err := a.git.ReadFileAtRef(clonePath, branch, fileRepoPath, pkg.Version)
			if err != nil {
				continue
			}

			localPath := filepath.Join(skillDir, entry.Name())
			localContent, err := os.ReadFile(localPath)
			if err != nil {
				drifted = append(drifted, "skills/"+skill+"/"+entry.Name())
				continue
			}

			localHash := xxhash.Sum64(localContent)
			originalHash := xxhash.Sum64(originalContent)
			if localHash != originalHash {
				drifted = append(drifted, "skills/"+skill+"/"+entry.Name())
			}
		}
	}
	for _, agent := range pkg.Files.Agents {
		repoRelPath := a.agentFileRepoPath(pkg.Profile, agent)
		originalContent, err := a.git.ReadFileAtRef(clonePath, branch, repoRelPath, pkg.Version)
		if err != nil {
			continue
		}

		localPath := filepath.Join(localBase, "agents", agent)
		localContent, err := os.ReadFile(localPath)
		if err != nil {
			drifted = append(drifted, "agents/"+agent)
			continue
		}

		localHash := xxhash.Sum64(localContent)
		originalHash := xxhash.Sum64(originalContent)
		if localHash != originalHash {
			drifted = append(drifted, "agents/"+agent)
		}
	}
	return drifted
}

// skillFileRepoPath derives the repo-relative path for a skill file given a
// package profile and the skill leaf name.
// Profile "dev/go" + skill "bar" + file "SKILL.md" -> "dev/go/skills/bar/SKILL.md"
// Profile ""       + skill "bar" + file "SKILL.md" -> "skills/bar/SKILL.md"
// Profile "skills/foo" + skill "foo" + file "SKILL.md" -> "skills/foo/SKILL.md"
func (a *App) skillFileRepoPath(profile, skill, filename string) string {
	// When the profile itself is a skill directory (e.g. "skills/azure-aigateway"),
	// the skill name is already embedded in the profile path. Appending
	// "/skills/<name>" again would produce a broken double path.
	if strings.HasSuffix(profile, "/"+skill) {
		parts := strings.Split(profile, "/")
		for _, p := range parts {
			if p == "skills" {
				return profile + "/" + filename
			}
		}
	}
	if profile != "" {
		return profile + "/skills/" + skill + "/" + filename
	}
	return "skills/" + skill + "/" + filename
}

// agentFileRepoPath derives the repo-relative path for an agent file given a
// package profile and the agent filename.
// Profile "dev/go" + agent "foo.md" -> "dev/go/agents/foo.md"
// Profile ""       + agent "foo.md" -> "agents/foo.md"
func (a *App) agentFileRepoPath(profile, agent string) string {
	if profile != "" {
		return profile + "/agents/" + agent
	}
	return "agents/" + agent
}

// skillDirRepoPath returns the repo-relative directory path for a skill,
// accounting for profiles that are themselves skill directories.
// Profile "dev/go"        + skill "bar" -> "dev/go/skills/bar"
// Profile ""              + skill "bar" -> "skills/bar"
// Profile "skills/foo"    + skill "foo" -> "skills/foo"
func (a *App) skillDirRepoPath(profile, skill string) string {
	return strings.TrimSuffix(a.skillFileRepoPath(profile, skill, "SKILL.md"), "/SKILL.md")
}

// packageFileRefs returns all MctRefs for the files in an installed package.
func (a *App) packageFileRefs(market string, pkg domain.InstalledPackage) []domain.MctRef {
	refs := make([]domain.MctRef, 0, len(pkg.Files.Skills)+len(pkg.Files.Agents))
	for _, skill := range pkg.Files.Skills {
		repoPath := a.skillFileRepoPath(pkg.Profile, skill, "SKILL.md")
		refs = append(refs, domain.MctRef(market+"@"+repoPath))
	}
	for _, agent := range pkg.Files.Agents {
		repoPath := a.agentFileRepoPath(pkg.Profile, agent)
		refs = append(refs, domain.MctRef(market+"@"+repoPath))
	}
	return refs
}

func (a *App) Check(opts service.CheckOpts) ([]domain.EntryStatus, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return nil, err
	}

	// Clean stale locations
	stale := db.CleanStaleLocations(func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	})
	if len(stale) > 0 {
		if lockErr := a.idb.Lock(a.cacheDir); lockErr == nil {
			_ = a.idb.Save(a.cacheDir, db)
			_ = a.idb.Unlock(a.cacheDir)
		}
	}

	projectPath := projectPath(cfg.LocalPath)

	var statuses []domain.EntryStatus

	for _, im := range db.Markets {
		if opts.Market != "" && im.Market != opts.Market {
			continue
		}

		mc := findMarketConfig(cfg, im.Market)
		if mc == nil {
			continue
		}

		clonePath := a.clonePath(im.Market)

		for _, pkg := range im.Packages {
			if pkg.FindLocation(projectPath) == nil {
				continue
			}

			driftFiles := a.detectDrift(pkg, projectPath, clonePath, mc.Branch)
			hasDrift := len(driftFiles) > 0

			headSHA, err := a.git.RemoteHEAD(clonePath, mc.Branch)
			hasUpdate := err == nil && headSHA != pkg.Version

			var state domain.EntryState
			switch {
			case hasDrift && hasUpdate:
				state = domain.StateUpdateAndDrift
			case hasDrift:
				state = domain.StateDrift
			case hasUpdate:
				state = domain.StateUpdateAvailable
			default:
				state = domain.StateClean
			}

			for _, ref := range a.packageFileRefs(im.Market, pkg) {
				es := domain.EntryStatus{
					Ref:   ref,
					State: state,
				}

				_, refRelPath, _ := ref.Parse()
				entryForCheck := domain.Entry{
					Ref:      ref,
					Market:   im.Market,
					RelPath:  refRelPath,
					Filename: filepath.Base(refRelPath),
					Type:     inferEntryType(refRelPath),
					Profile:  pkg.Profile,
				}
				if toolStates := a.detectToolDrift(entryForCheck, pkg, projectPath); len(toolStates) > 0 {
					es.ToolStates = toolStates
				}

				statuses = append(statuses, es)
			}
		}
	}

	return statuses, nil
}

func (a *App) Refresh(opts service.RefreshOpts) ([]service.RefreshResult, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	syncState, err := a.state.LoadSyncState(a.cacheDir)
	if err != nil {
		return nil, err
	}

	var results []service.RefreshResult

	for _, mc := range cfg.Markets {
		if opts.Market != "" && mc.Name != opts.Market {
			continue
		}

		clonePath := a.clonePath(mc.Name)

		oldSHA := ""
		if ms, ok := syncState.Markets[mc.Name]; ok {
			oldSHA = ms.LastSyncedSHA
		}

		if !opts.DryRun {
			if err := a.state.SetMarketSyncDirty(a.cacheDir, mc.Name); err != nil {
				results = append(results, service.RefreshResult{
					Market: mc.Name,
					OldSHA: oldSHA,
					Err:    err,
				})
				continue
			}
		}

		newSHA, err := a.git.Fetch(clonePath, mc.Branch)
		if err != nil {
			results = append(results, service.RefreshResult{
				Market: mc.Name,
				OldSHA: oldSHA,
				Err:    err,
			})
			continue
		}

		var changedFiles int
		if oldSHA != "" {
			diffs, diffErr := a.git.DiffSinceCommit(clonePath, mc.Branch, oldSHA)
			if diffErr == nil {
				for _, d := range diffs {
					path := d.To
					if path == "" {
						path = d.From
					}
					if mc.SkillsOnly && !isSkillPath(path, mc.SkillsPath) {
						continue
					}
					changedFiles++
				}
			}
		}

		// Count updates available from installdb
		updatesAvailable := 0
		db, dbErr := a.idb.Load(a.cacheDir)
		if dbErr == nil {
			im := db.FindMarket(mc.Name)
			if im != nil {
				for _, pkg := range im.Packages {
					if pkg.Version != newSHA {
						updatesAvailable++
					}
				}
			}
		}

		if !opts.DryRun {
			if err := a.state.SetMarketSyncClean(a.cacheDir, mc.Name, newSHA); err != nil {
				results = append(results, service.RefreshResult{
					Market: mc.Name,
					OldSHA: oldSHA,
					NewSHA: newSHA,
					Err:    err,
				})
				continue
			}
		}

		results = append(results, service.RefreshResult{
			Market:           mc.Name,
			OldSHA:           oldSHA,
			NewSHA:           newSHA,
			ChangedFiles:     changedFiles,
			UpdatesAvailable: updatesAvailable,
		})
	}

	return results, nil
}

func (a *App) Update(opts service.UpdateOpts) ([]service.UpdateResult, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	if err := a.idb.Lock(a.cacheDir); err != nil {
		return nil, err
	}
	defer func() { _ = a.idb.Unlock(a.cacheDir) }()

	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return nil, err
	}

	var results []service.UpdateResult

	for _, mc := range cfg.Markets {
		if opts.Market != "" && mc.Name != opts.Market {
			continue
		}

		clonePath := a.clonePath(mc.Name)

		im := db.FindMarket(mc.Name)
		if im == nil {
			continue
		}

		var refRelPath string
		if opts.Ref != "" {
			refMarket, rp, refErr := opts.Ref.Parse()
			if refErr != nil || refMarket != mc.Name {
				continue
			}
			refRelPath = rp
		}

		for pi := range im.Packages {
			pkg := &im.Packages[pi]

			if pkg.Version == "" {
				continue
			}

			if refRelPath != "" && !fileInPackage(refRelPath, pkg.Files) {
				continue
			}

			diffs, err := a.git.DiffSinceCommit(clonePath, mc.Branch, pkg.Version)
			if err != nil {
				continue
			}

			affected := a.findAffectedPackages(im, diffs, mc)
			if _, ok := affected[pi]; !ok {
				continue
			}

			seen := make(map[string]bool)
			for _, location := range pkg.Locations {
				if seen[location.Path] {
					continue
				}
				seen[location.Path] = true
				r := a.updatePackageAtLocation(updateCtx{
					mc:        mc,
					im:        im,
					pkg:       pkg,
					pkgIdx:    pi,
					location:  location.Path,
					clonePath: clonePath,
					cfg:       cfg,
					opts:      opts,
					db:        &db,
				})
				results = append(results, r)
			}
		}
	}

	return results, nil
}

// updateCtx bundles the parameters needed by updatePackageAtLocation.
// updateCtx bundles the parameters needed by updatePackageAtLocation.
type updateCtx struct {
	mc        domain.MarketConfig
	im        *domain.InstalledMarket
	pkg       *domain.InstalledPackage
	pkgIdx    int
	location  string
	clonePath string
	cfg       domain.Config
	opts      service.UpdateOpts
	db        *domain.InstallDatabase
}

// findAffectedPackages identifies which installed packages in a market are
// touched by the given diffs. Returns a map of package index -> package.
func (a *App) findAffectedPackages(im *domain.InstalledMarket, diffs []domain.FileDiff, mc domain.MarketConfig) map[int]*domain.InstalledPackage {
	affected := make(map[int]*domain.InstalledPackage)

	for _, diff := range diffs {
		filePath := diff.To
		if filePath == "" {
			filePath = diff.From
		}

		if mc.SkillsOnly && !isSkillPath(filePath, mc.SkillsPath) {
			continue
		}

		if diff.Action == domain.DiffDelete {
			continue
		}

		for pi := range im.Packages {
			pkg := &im.Packages[pi]

			profilePath := pkg.Profile
			if profilePath != "" {
				prefix := profilePath + "/"
				if !strings.HasPrefix(filePath, prefix) && filePath != profilePath {
					if !fileInPackage(filePath, pkg.Files) {
						continue
					}
				}
			}

			if fileInPackage(filePath, pkg.Files) {
				affected[pi] = pkg
			}
		}
	}

	return affected
}

// updatePackageAtLocation performs the actual update of a single package at a
// single location: drift check, file replacement, and tool re-transform.
// updatePackageAtLocation performs the actual update of a single package at a
// single location: drift check, file replacement, and tool re-transform. It
// runs inside its own transaction so that on failure neither disk nor the
// install database mutate.
func (a *App) updatePackageAtLocation(ctx updateCtx) service.UpdateResult {
	oldVersion := domain.MctVersion(ctx.pkg.Version)

	driftFiles := a.detectDrift(*ctx.pkg, ctx.location, ctx.clonePath, ctx.mc.Branch)

	if len(driftFiles) > 0 {
		if ctx.opts.AllKeep {
			return service.UpdateResult{
				Ref:        a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
				Location:   ctx.location,
				Action:     "kept",
				OldVersion: oldVersion,
				DriftFiles: driftFiles,
			}
		}
		if !ctx.opts.AllMerge {
			return service.UpdateResult{
				Ref:        a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
				Location:   ctx.location,
				Action:     "drift",
				OldVersion: oldVersion,
				DriftFiles: driftFiles,
			}
		}
	}

	w, commit, rollback, err := a.beginWriter("update:" + ctx.mc.Name + ":" + ctx.pkg.Profile)
	if err != nil {
		return service.UpdateResult{
			Ref:      a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
			Location: ctx.location,
			Action:   "error",
			Err:      err,
		}
	}
	committed := false
	defer func() {
		if !committed {
			_ = rollback()
		}
	}()

	// Snapshot the previous claude-code file set BEFORE writing, so we can
	// diff and prune orphans (files removed in the upstream version).
	var oldClaudeFiles []domain.InstalledFile
	if loc := findLocationByPathAndType(ctx.pkg, ctx.location, domain.RuntimeTypeClaudeCode); loc != nil {
		oldClaudeFiles = append(oldClaudeFiles, loc.Files...)
	}

	newFiles, claudeFiles := a.copyUpdatedFiles(w, ctx)

	a.pruneRemovedFiles(w, ctx.location, oldClaudeFiles, claudeFiles)

	newSHA, err := a.git.RemoteHEAD(ctx.clonePath, ctx.mc.Branch)
	if err != nil {
		return service.UpdateResult{
			Ref:      a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
			Location: ctx.location,
			Action:   "error",
			Err:      err,
		}
	}

	// Replace package-level Files and the claude-code location's Files
	// with the new authoritative set.
	ctx.im.Packages[ctx.pkgIdx].Version = newSHA
	ctx.im.Packages[ctx.pkgIdx].Files = newFiles

	if loc := findLocationByPathAndType(&ctx.im.Packages[ctx.pkgIdx], ctx.location, domain.RuntimeTypeClaudeCode); loc != nil {
		loc.Files = claudeFiles
	} else {
		ctx.im.Packages[ctx.pkgIdx].Locations = append(ctx.im.Packages[ctx.pkgIdx].Locations, domain.InstalledLocation{
			Path:  ctx.location,
			Type:  domain.RuntimeTypeClaudeCode,
			Files: claudeFiles,
		})
	}

	a.reTransformToolTargets(w, ctx.mc, ctx.im, ctx.pkgIdx, newFiles, ctx.location)

	if err := a.stageDBSave(w, *ctx.db); err != nil {
		return service.UpdateResult{
			Ref:      a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
			Location: ctx.location,
			Action:   "error",
			Err:      err,
		}
	}
	if err := commit(); err != nil {
		return service.UpdateResult{
			Ref:      a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
			Location: ctx.location,
			Action:   "error",
			Err:      err,
		}
	}
	committed = true

	return service.UpdateResult{
		Ref:        a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
		Location:   ctx.location,
		Action:     "update",
		OldVersion: oldVersion,
		NewVersion: domain.MctVersion(newSHA),
		DriftFiles: driftFiles,
	}
}

// copyUpdatedFiles reads updated skill and agent files from the cached clone
// and writes them to the local project directory.
func (a *App) copyUpdatedFiles(w txWriter, ctx updateCtx) (domain.InstalledFiles, []domain.InstalledFile) {
	var newFiles domain.InstalledFiles
	var written []domain.InstalledFile

	projectRel := func(abs string) string {
		rel, err := filepath.Rel(ctx.location, abs)
		if err != nil {
			return filepath.ToSlash(abs)
		}
		return filepath.ToSlash(rel)
	}

	for _, skill := range ctx.pkg.Files.Skills {
		skillDirPath := a.skillDirRepoPath(ctx.pkg.Profile, skill)
		dirFiles, err := a.git.ListDirFiles(ctx.clonePath, ctx.mc.Branch, skillDirPath)
		if err != nil || len(dirFiles) == 0 {
			// Skill removed upstream — skip; pruning will drop the leftover
			// files from disk and the location's Files list.
			continue
		}
		localSkillDir := filepath.Join(ctx.cfg.LocalPath, "skills", skill)
		wroteAny := false
		for _, f := range dirFiles {
			content, err := a.git.ReadFileAtRef(ctx.clonePath, ctx.mc.Branch, f, "HEAD")
			if err != nil {
				continue
			}
			dest := filepath.Join(localSkillDir, filepath.Base(f))
			if err := w.WriteFile(dest, content); err != nil {
				continue
			}
			written = append(written, domain.InstalledFile{
				Path: projectRel(dest),
				XXH:  xxhashHex(content),
			})
			wroteAny = true
		}
		if wroteAny {
			newFiles.Skills = append(newFiles.Skills, skill)
		}
	}

	for _, agent := range ctx.pkg.Files.Agents {
		repoPath := a.agentFileRepoPath(ctx.pkg.Profile, agent)
		content, err := a.git.ReadFileAtRef(ctx.clonePath, ctx.mc.Branch, repoPath, "HEAD")
		if err != nil {
			// Agent removed upstream — skip.
			continue
		}
		localPath := filepath.Join(ctx.cfg.LocalPath, "agents", agent)
		if err := w.WriteFile(localPath, content); err != nil {
			continue
		}
		written = append(written, domain.InstalledFile{
			Path: projectRel(localPath),
			XXH:  xxhashHex(content),
		})
		newFiles.Agents = append(newFiles.Agents, agent)
	}

	return newFiles, written
}

// reTransformToolTargets re-transforms all files in a package to enabled tool
// targets and updates the tool checksums in the install database.
// reTransformToolTargets re-transforms all files in a package to enabled tool
// targets and updates the tool checksums in the install database.
func (a *App) reTransformToolTargets(w txWriter, mc domain.MarketConfig, im *domain.InstalledMarket, pkgIdx int, files domain.InstalledFiles, location string) {
	pkg := &im.Packages[pkgIdx]

	// Snapshot per-tool old file sets BEFORE re-writing so we can prune
	// orphans (files dropped from the new version) per tool.
	oldByTool := make(map[string][]domain.InstalledFile)
	for _, loc := range pkg.Locations {
		if loc.Path != location || loc.Type == domain.RuntimeTypeClaudeCode {
			continue
		}
		oldByTool[loc.Type] = append([]domain.InstalledFile(nil), loc.Files...)
	}

	newByTool := make(map[string][]domain.InstalledFile)
	accumulate := func(twr toolWriteResult) {
		for toolName, toolFiles := range twr.ToolFiles {
			newByTool[toolName] = append(newByTool[toolName], toolFiles...)
		}
	}

	for _, skill := range files.Skills {
		repoPath := a.skillFileRepoPath(pkg.Profile, skill, "SKILL.md")
		content, err := a.git.ReadFileAtRef(a.clonePath(mc.Name), mc.Branch, repoPath, "HEAD")
		if err != nil {
			continue
		}
		entry := domain.Entry{
			Ref:      domain.MctRef(mc.Name + "@" + repoPath),
			Market:   mc.Name,
			RelPath:  repoPath,
			Filename: "SKILL.md",
			Type:     domain.EntryTypeSkill,
			Profile:  pkg.Profile,
		}
		accumulate(a.writeToToolTargets(w, entry, content, location))
	}

	for _, agent := range files.Agents {
		repoPath := a.agentFileRepoPath(pkg.Profile, agent)
		content, err := a.git.ReadFileAtRef(a.clonePath(mc.Name), mc.Branch, repoPath, "HEAD")
		if err != nil {
			continue
		}
		entry := domain.Entry{
			Ref:      domain.MctRef(mc.Name + "@" + repoPath),
			Market:   mc.Name,
			RelPath:  repoPath,
			Filename: agent,
			Type:     domain.EntryTypeAgent,
			Profile:  pkg.Profile,
		}
		accumulate(a.writeToToolTargets(w, entry, content, location))
	}

	// Prune per-tool: files in old\new are gone from the new version.
	for toolName, oldFiles := range oldByTool {
		a.pruneRemovedFiles(w, location, oldFiles, newByTool[toolName])
	}

	// Replace each tool location's Files with the new authoritative set.
	// Drop locations whose tool produced no files this round.
	for toolName, toolFiles := range newByTool {
		loc := findLocationByPathAndType(pkg, location, toolName)
		if loc == nil {
			pkg.Locations = append(pkg.Locations, domain.InstalledLocation{
				Path:  location,
				Type:  toolName,
				Files: toolFiles,
			})
		} else {
			loc.Files = toolFiles
		}
	}
}

// mergeToolChecksums merges new checksums into a package's ToolChecksums map.
// findLocationByPathAndType returns the location entry for the given (path,
// type) tuple, or nil. Locations may share a path across runtime types.
func findLocationByPathAndType(pkg *domain.InstalledPackage, path, runtimeType string) *domain.InstalledLocation {
	for i := range pkg.Locations {
		if pkg.Locations[i].Path == path && pkg.Locations[i].Type == runtimeType {
			return &pkg.Locations[i]
		}
	}
	return nil
}

// packagePrimaryRef returns a representative MctRef for a package, using the
// first skill or agent file.
func (a *App) packagePrimaryRef(market string, pkg *domain.InstalledPackage) domain.MctRef {
	refs := a.packageFileRefs(market, *pkg)
	if len(refs) > 0 {
		return refs[0]
	}
	return domain.MctRef(market + "@" + pkg.Profile)
}

func (a *App) Sync(opts service.SyncOpts) ([]service.SyncResult, error) {
	refreshResults, err := a.Refresh(service.RefreshOpts{
		Market: opts.Market,
		DryRun: opts.DryRun,
		CI:     opts.CI,
	})
	if err != nil {
		return nil, err
	}

	updateResults, err := a.Update(service.UpdateOpts{
		Market:         opts.Market,
		DryRun:         opts.DryRun,
		CI:             opts.CI,
		AcceptBreaking: opts.AcceptBreaking,
		AllMerge:       opts.AllMerge,
		AllLocations:   opts.AllLocations,
	})
	if err != nil {
		return nil, err
	}

	updatesByMarket := make(map[string][]service.UpdateResult)
	for _, u := range updateResults {
		updatesByMarket[u.Ref.Market()] = append(updatesByMarket[u.Ref.Market()], u)
	}

	results := make([]service.SyncResult, 0, len(refreshResults))
	for _, r := range refreshResults {
		results = append(results, service.SyncResult{
			Refresh: r,
			Updates: updatesByMarket[r.Market],
		})
	}

	return results, nil
}

func (a *App) SyncState() (domain.SyncState, error) {
	return a.state.LoadSyncState(a.cacheDir)
}

// pruneRemovedFiles deletes files present in oldFiles but not newFiles
// (matched by Path). After deletion it walks the parent directory chain
// upward, removing any directory that becomes empty, stopping at the
// project root. Missing files are ignored — the on-disk truth may have
// already drifted from the recorded set.
//
// projectRoot is the project base (typically cfg.LocalPath's parent so
// we don't accidentally remove .claude/). InstalledFile.Path is rooted
// at projectRoot.
// pruneRemovedFiles deletes files present in oldFiles but not newFiles
// (matched by Path). After deletion it walks the parent directory chain
// upward, removing any directory that becomes empty, stopping at the
// project root. Missing files are ignored — the on-disk truth may have
// already drifted from the recorded set.
//
// projectRoot is the project base (typically cfg.LocalPath's parent so
// we don't accidentally remove .claude/). InstalledFile.Path is rooted
// at projectRoot.
// pruneRemovedFiles removes files that were present in the old version but
// are gone from the new version. Empty parent directories are cleaned up
// best-effort (directly via os, not via the tx — they're metadata, not
// content).
//
// projectRoot is the project base (typically cfg.LocalPath's parent so
// we don't accidentally remove .claude/). InstalledFile.Path is rooted
// at projectRoot.
func (a *App) pruneRemovedFiles(w txWriter, projectRoot string, oldFiles, newFiles []domain.InstalledFile) {
	if len(oldFiles) == 0 {
		return
	}
	keep := make(map[string]bool, len(newFiles))
	for _, f := range newFiles {
		keep[f.Path] = true
	}
	for _, f := range oldFiles {
		if keep[f.Path] {
			continue
		}
		abs := filepath.Join(projectRoot, filepath.FromSlash(f.Path))
		if err := w.DeleteFile(abs); err != nil && !os.IsNotExist(err) {
			continue
		}
		dir := filepath.Dir(abs)
		for dir != projectRoot && dir != "." && dir != string(filepath.Separator) {
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) > 0 {
				break
			}
			if err := os.Remove(dir); err != nil {
				break
			}
			dir = filepath.Dir(dir)
		}
	}
}
