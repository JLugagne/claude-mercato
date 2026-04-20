package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash/v2"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// detectDrift compares installed files at a location against the cached clone
// at the version recorded in installdb. Returns list of files that differ.
func (a *App) detectDrift(pkg domain.InstalledPackage, location, clonePath, branch string) []string {
	var drifted []string
	localBase := filepath.Join(location, ".claude")

	for _, skill := range pkg.Files.Skills {
		// First check if we can read the original SKILL.md at the recorded version.
		// If the original doesn't exist at that ref, skip drift check for this skill.
		repoRelPath := a.skillFileRepoPath(pkg.Profile, skill, "SKILL.md")
		_, origErr := a.git.ReadFileAtRef(clonePath, branch, repoRelPath, pkg.Version)
		if origErr != nil {
			// Skill didn't exist at that version, skip drift check
			continue
		}

		skillDir := filepath.Join(localBase, "skills", skill)
		entries, err := os.ReadDir(skillDir)
		if err != nil {
			// Local directory doesn't exist but original does => drift
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
				// File didn't exist at that version, skip drift check
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
			// File didn't exist at that version, skip drift check
			continue
		}

		localPath := filepath.Join(localBase, "agents", agent)
		localContent, err := os.ReadFile(localPath)
		if err != nil {
			// Local file missing but original exists => drift
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
		// Save cleaned db
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
			atLocation := false
			for _, loc := range pkg.Locations {
				if loc == projectPath {
					atLocation = true
					break
				}
			}
			if !atLocation {
				continue
			}

			// Check for drift
			driftFiles := a.detectDrift(pkg, projectPath, clonePath, mc.Branch)
			hasDrift := len(driftFiles) > 0

			// Check for updates
			headSHA, err := a.git.RemoteHEAD(clonePath, mc.Branch)
			hasUpdate := err == nil && headSHA != pkg.Version

			// Build a ref for each file in the package
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

				// Per-tool drift detection
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

	// Lock installdb
	if err := a.idb.Lock(a.cacheDir); err != nil {
		return nil, err
	}
	defer func() { _ = a.idb.Unlock(a.cacheDir) }()

	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return nil, err
	}

	projectPath := projectPath(cfg.LocalPath)

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

		// Resolve ref filter once per market
		var refRelPath string
		if opts.Ref != "" {
			refMarket, rp, refErr := opts.Ref.Parse()
			if refErr != nil || refMarket != mc.Name {
				continue
			}
			refRelPath = rp
		}

		// Process each package independently, diffing from its installed version
		// to the current HEAD so that Update works correctly after Refresh.
		for pi := range im.Packages {
			pkg := &im.Packages[pi]

			if pkg.Version == "" {
				continue
			}

			// Apply ref filter: skip packages that don't contain the target file
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

			for _, location := range pkg.Locations {
				if location != projectPath {
					continue
				}
				r := a.updatePackageAtLocation(updateCtx{
					mc:        mc,
					im:        im,
					pkg:       pkg,
					pkgIdx:    pi,
					location:  location,
					clonePath: clonePath,
					cfg:       cfg,
					opts:      opts,
				})
				results = append(results, r)
			}
		}
	}

	// Save installdb
	if err := a.idb.Save(a.cacheDir, db); err != nil {
		return results, err
	}

	return results, nil
}

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

	// Delete old files
	if err := a.deleteInstalledFiles(ctx.cfg.LocalPath, ctx.pkg.Files); err != nil {
		return service.UpdateResult{
			Ref:      a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
			Location: ctx.location,
			Action:   "error",
			Err:      err,
		}
	}

	// Copy new files from cached clone
	newFiles := a.copyUpdatedFiles(ctx)

	// Get new HEAD
	newSHA, err := a.git.RemoteHEAD(ctx.clonePath, ctx.mc.Branch)
	if err != nil {
		return service.UpdateResult{
			Ref:      a.packagePrimaryRef(ctx.mc.Name, ctx.pkg),
			Location: ctx.location,
			Action:   "error",
			Err:      err,
		}
	}

	// Update package version in db
	ctx.im.Packages[ctx.pkgIdx].Version = newSHA
	ctx.im.Packages[ctx.pkgIdx].Files = newFiles

	// Re-transform to all enabled tool targets
	a.reTransformToolTargets(ctx.mc, ctx.im, ctx.pkgIdx, newFiles, ctx.location)

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
func (a *App) copyUpdatedFiles(ctx updateCtx) domain.InstalledFiles {
	var newFiles domain.InstalledFiles

	for _, skill := range ctx.pkg.Files.Skills {
		skillDirPath := a.skillDirRepoPath(ctx.pkg.Profile, skill)
		dirFiles, err := a.git.ListDirFiles(ctx.clonePath, ctx.mc.Branch, skillDirPath)
		if err != nil {
			continue
		}
		localSkillDir := filepath.Join(ctx.cfg.LocalPath, "skills", skill)
		for _, f := range dirFiles {
			content, err := a.git.ReadFileAtRef(ctx.clonePath, ctx.mc.Branch, f, "HEAD")
			if err != nil {
				continue
			}
			if err := a.fs.WriteFile(filepath.Join(localSkillDir, filepath.Base(f)), content); err != nil {
				continue
			}
		}
		newFiles.Skills = append(newFiles.Skills, skill)
	}

	for _, agent := range ctx.pkg.Files.Agents {
		repoPath := a.agentFileRepoPath(ctx.pkg.Profile, agent)
		content, err := a.git.ReadFileAtRef(ctx.clonePath, ctx.mc.Branch, repoPath, "HEAD")
		if err != nil {
			continue
		}
		localPath := filepath.Join(ctx.cfg.LocalPath, "agents", agent)
		if err := a.fs.WriteFile(localPath, content); err != nil {
			continue
		}
		newFiles.Agents = append(newFiles.Agents, agent)
	}

	return newFiles
}

// reTransformToolTargets re-transforms all files in a package to enabled tool
// targets and updates the tool checksums in the install database.
func (a *App) reTransformToolTargets(mc domain.MarketConfig, im *domain.InstalledMarket, pkgIdx int, files domain.InstalledFiles, location string) {
	pkg := &im.Packages[pkgIdx]

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
		twr := a.writeToToolTargets(entry, content, location)
		a.mergeToolChecksums(pkg, twr.Checksums)
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
		twr := a.writeToToolTargets(entry, content, location)
		a.mergeToolChecksums(pkg, twr.Checksums)
	}
}

// mergeToolChecksums merges new checksums into a package's ToolChecksums map.
func (a *App) mergeToolChecksums(pkg *domain.InstalledPackage, checksums map[string]string) {
	if len(checksums) == 0 {
		return
	}
	if pkg.ToolChecksums == nil {
		pkg.ToolChecksums = make(map[string]string)
	}
	for k, v := range checksums {
		pkg.ToolChecksums[k] = v
	}
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
