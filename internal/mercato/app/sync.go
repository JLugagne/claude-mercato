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
// Profile "mkt@dev/go" + skill "bar" + file "SKILL.md" -> "dev/go/skills/bar/SKILL.md"
// Profile "mkt" + skill "bar" + file "SKILL.md" -> "skills/bar/SKILL.md"
func (a *App) skillFileRepoPath(profile, skill, filename string) string {
	_, profilePath := splitProfile(profile)
	if profilePath != "" {
		return profilePath + "/skills/" + skill + "/" + filename
	}
	return "skills/" + skill + "/" + filename
}

// agentFileRepoPath derives the repo-relative path for an agent file given a
// package profile and the agent filename.
// Profile "mkt@dev/go" + agent "foo.md" -> "dev/go/agents/foo.md"
// Profile "mkt" + agent "foo.md" -> "agents/foo.md"
func (a *App) agentFileRepoPath(profile, agent string) string {
	_, profilePath := splitProfile(profile)
	if profilePath != "" {
		return profilePath + "/agents/" + agent
	}
	return "agents/" + agent
}

// splitProfile splits a profile string "market@path" into market and path.
// Returns ("market", "") for profiles without a path component.
func splitProfile(profile string) (market, path string) {
	idx := strings.Index(profile, "@")
	if idx < 0 {
		return profile, ""
	}
	return profile[:idx], profile[idx+1:]
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
			for _, skill := range pkg.Files.Skills {
				repoPath := a.skillFileRepoPath(pkg.Profile, skill, "SKILL.md")
				ref := domain.MctRef(im.Market + "@" + repoPath)

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

				statuses = append(statuses, domain.EntryStatus{
					Ref:   ref,
					State: state,
				})
			}
			for _, agent := range pkg.Files.Agents {
				repoPath := a.agentFileRepoPath(pkg.Profile, agent)
				ref := domain.MctRef(im.Market + "@" + repoPath)

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

				statuses = append(statuses, domain.EntryStatus{
					Ref:   ref,
					State: state,
				})
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

	syncState, err := a.state.LoadSyncState(a.cacheDir)
	if err != nil {
		return nil, err
	}

	// Lock installdb
	if err := a.idb.Lock(a.cacheDir); err != nil {
		return nil, err
	}
	defer a.idb.Unlock(a.cacheDir)

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

		lastSyncedSHA := ""
		if ms, ok := syncState.Markets[mc.Name]; ok {
			lastSyncedSHA = ms.LastSyncedSHA
		}

		if lastSyncedSHA == "" {
			continue
		}

		diffs, err := a.git.DiffSinceCommit(clonePath, mc.Branch, lastSyncedSHA)
		if err != nil {
			continue
		}

		// Find which installed packages are affected by the diffs
		im := db.FindMarket(mc.Name)
		if im == nil {
			continue
		}

		// Build a set of affected package indices
		type affectedPkg struct {
			pkgIdx int
			pkg    *domain.InstalledPackage
		}
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

			// Match diff path against installed packages
			for pi := range im.Packages {
				pkg := &im.Packages[pi]

				// Check if the file belongs to a skill or agent in this package.
				// fileInPackage strips the path to just the leaf name, so we also
				// need to verify the diff path could belong to this profile.
				_, profilePath := splitProfile(pkg.Profile)
				if profilePath != "" {
					// Profile path can be a directory (e.g. "dev/go") or a
					// leaf path (e.g. "agents/foo.md"). Match if the diff path
					// starts with the profile prefix OR equals the profile path.
					prefix := profilePath + "/"
					if !strings.HasPrefix(filePath, prefix) && filePath != profilePath {
						// Also check if the diff is directly an agent/skill
						// file that matches this package without prefix.
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

		// Apply ref filter if specified
		if opts.Ref != "" {
			refMarket, refRelPath, refErr := opts.Ref.Parse()
			if refErr != nil || refMarket != mc.Name {
				continue
			}
			// Filter to only the package containing this ref
			filtered := make(map[int]*domain.InstalledPackage)
			for pi, pkg := range affected {
				if fileInPackage(refRelPath, pkg.Files) {
					filtered[pi] = pkg
				}
			}
			affected = filtered
		}

		// Process each affected package
		for pi, pkg := range affected {
			oldVersion := domain.MctVersion(pkg.Version)

			// Process each location
			for _, location := range pkg.Locations {
				if location != projectPath {
					continue
				}

				// Drift detection
				driftFiles := a.detectDrift(*pkg, location, clonePath, mc.Branch)

				if len(driftFiles) > 0 {
					if opts.AllKeep {
						// Build ref for reporting
						ref := a.packagePrimaryRef(mc.Name, pkg)
						results = append(results, service.UpdateResult{
							Ref:        ref,
							Location:   location,
							Action:     "kept",
							OldVersion: oldVersion,
							DriftFiles: driftFiles,
						})
						continue
					}
					if !opts.AllMerge {
						// Report drift without updating
						ref := a.packagePrimaryRef(mc.Name, pkg)
						results = append(results, service.UpdateResult{
							Ref:        ref,
							Location:   location,
							Action:     "drift",
							OldVersion: oldVersion,
							DriftFiles: driftFiles,
						})
						continue
					}
					// AllMerge: fall through to overwrite
				}

				// Delete old files
				if err := a.deleteInstalledFiles(cfg.LocalPath, pkg.Files); err != nil {
					ref := a.packagePrimaryRef(mc.Name, pkg)
					results = append(results, service.UpdateResult{
						Ref:      ref,
						Location: location,
						Action:   "error",
						Err:      err,
					})
					continue
				}

				// Copy new files from cached clone
				var newFiles domain.InstalledFiles
				for _, skill := range pkg.Files.Skills {
					_, profilePath := splitProfile(pkg.Profile)
					var skillDirPath string
					if profilePath != "" {
						skillDirPath = profilePath + "/skills/" + skill
					} else {
						skillDirPath = "skills/" + skill
					}

					dirFiles, err := a.git.ListDirFiles(clonePath, mc.Branch, skillDirPath)
					if err != nil {
						// Skill may have been removed; skip
						continue
					}
					localSkillDir := filepath.Join(cfg.LocalPath, "skills", skill)
					for _, f := range dirFiles {
						content, err := a.git.ReadFileAtRef(clonePath, mc.Branch, f, "HEAD")
						if err != nil {
							continue
						}
						fileName := filepath.Base(f)
						if err := a.fs.WriteFile(filepath.Join(localSkillDir, fileName), content); err != nil {
							continue
						}
					}
					newFiles.Skills = append(newFiles.Skills, skill)
				}
				for _, agent := range pkg.Files.Agents {
					repoPath := a.agentFileRepoPath(pkg.Profile, agent)
					content, err := a.git.ReadFileAtRef(clonePath, mc.Branch, repoPath, "HEAD")
					if err != nil {
						continue
					}
					localPath := filepath.Join(cfg.LocalPath, "agents", agent)
					if err := a.fs.WriteFile(localPath, content); err != nil {
						continue
					}
					newFiles.Agents = append(newFiles.Agents, agent)
				}

				// Get new HEAD
				newSHA, err := a.git.RemoteHEAD(clonePath, mc.Branch)
				if err != nil {
					ref := a.packagePrimaryRef(mc.Name, pkg)
					results = append(results, service.UpdateResult{
						Ref:      ref,
						Location: location,
						Action:   "error",
						Err:      err,
					})
					continue
				}

				// Update package version in db
				im.Packages[pi].Version = newSHA
				im.Packages[pi].Files = newFiles

				ref := a.packagePrimaryRef(mc.Name, pkg)
				results = append(results, service.UpdateResult{
					Ref:        ref,
					Location:   location,
					Action:     "update",
					OldVersion: oldVersion,
					NewVersion: domain.MctVersion(newSHA),
					DriftFiles: driftFiles,
				})
			}
		}
	}

	// Save installdb
	if err := a.idb.Save(a.cacheDir, db); err != nil {
		return results, err
	}

	return results, nil
}

// packagePrimaryRef returns a representative MctRef for a package, using the
// first skill or agent file.
func (a *App) packagePrimaryRef(market string, pkg *domain.InstalledPackage) domain.MctRef {
	if len(pkg.Files.Skills) > 0 {
		repoPath := a.skillFileRepoPath(pkg.Profile, pkg.Files.Skills[0], "SKILL.md")
		return domain.MctRef(market + "@" + repoPath)
	}
	if len(pkg.Files.Agents) > 0 {
		repoPath := a.agentFileRepoPath(pkg.Profile, pkg.Files.Agents[0])
		return domain.MctRef(market + "@" + repoPath)
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
