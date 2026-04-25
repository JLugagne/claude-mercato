package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JLugagne/agents-mercato/assets"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

func (a *App) List(opts service.ListOpts) ([]domain.Entry, error) {
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

	var entries []domain.Entry

	for _, im := range db.Markets {
		if opts.Market != "" && im.Market != opts.Market {
			continue
		}

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

			for _, skill := range pkg.Files.Skills {
				repoPath := a.skillFileRepoPath(pkg.Profile, skill, "SKILL.md")
				ref := domain.MctRef(im.Market + "@" + repoPath)
				entry := domain.Entry{
					Ref:       ref,
					Market:    im.Market,
					RelPath:   repoPath,
					Filename:  "SKILL.md",
					Type:      domain.EntryTypeSkill,
					Version:   domain.MctVersion(pkg.Version),
					Profile:   pkg.Profile,
					Installed: true,
				}
				if opts.Type != "" && entry.Type != opts.Type {
					continue
				}
				entries = append(entries, entry)
			}
			for _, agent := range pkg.Files.Agents {
				repoPath := a.agentFileRepoPath(pkg.Profile, agent)
				ref := domain.MctRef(im.Market + "@" + repoPath)
				entry := domain.Entry{
					Ref:       ref,
					Market:    im.Market,
					RelPath:   repoPath,
					Filename:  agent,
					Type:      domain.EntryTypeAgent,
					Version:   domain.MctVersion(pkg.Version),
					Profile:   pkg.Profile,
					Installed: true,
				}
				if opts.Type != "" && entry.Type != opts.Type {
					continue
				}
				entries = append(entries, entry)
			}
		}
	}
	return entries, nil
}

func (a *App) ReadEntryContent(market, relPath string) ([]byte, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	mc := findMarketConfig(cfg, market)
	if mc == nil {
		return nil, domain.ErrMarketNotFound
	}

	clonePath := a.clonePath(market)
	return a.git.ReadFileAtRef(clonePath, mc.Branch, relPath, "HEAD")
}

// ListSkillDirFiles lists all files under a skill directory in the market repo
// and reads the content of .md files.
func (a *App) ListSkillDirFiles(market, dirPrefix string) ([]domain.SkillDirFile, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	mc := findMarketConfig(cfg, market)
	if mc == nil {
		return nil, domain.ErrMarketNotFound
	}

	clonePath := a.clonePath(market)
	files, err := a.git.ListDirFiles(clonePath, mc.Branch, dirPrefix)
	if err != nil {
		return nil, err
	}

	result := make([]domain.SkillDirFile, 0, len(files))
	for _, f := range files {
		sf := domain.SkillDirFile{
			Path: f,
			Name: filepath.Base(f),
		}
		if strings.HasSuffix(f, ".md") {
			content, err := a.git.ReadFileAtRef(clonePath, mc.Branch, f, "HEAD")
			if err == nil {
				sf.Content = string(content)
			}
		}
		result = append(result, sf)
	}
	return result, nil
}

func (a *App) GetEntry(ref domain.MctRef) (domain.Entry, error) {
	marketName, relPath, err := ref.Parse()
	if err != nil {
		return domain.Entry{}, err
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return domain.Entry{}, err
	}

	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return domain.Entry{}, err
	}

	projectPath := projectPath(cfg.LocalPath)

	im := db.FindMarket(marketName)
	if im == nil {
		return domain.Entry{}, domain.ErrEntryNotFound
	}

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

		if !fileInPackage(relPath, pkg.Files) {
			continue
		}

		entryType := inferEntryType(relPath)
		return domain.Entry{
			Ref:       ref,
			Market:    marketName,
			RelPath:   relPath,
			Filename:  filepath.Base(relPath),
			Type:      entryType,
			Version:   domain.MctVersion(pkg.Version),
			Profile:   pkg.Profile,
			Installed: true,
		}, nil
	}

	return domain.Entry{}, domain.ErrEntryNotFound
}

func (a *App) Add(ref domain.MctRef, opts service.AddOpts) (service.AddResult, error) {
	marketName, relPath, err := ref.Parse()
	if err != nil {
		return service.AddResult{}, err
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return service.AddResult{}, err
	}

	mc := findMarketConfig(cfg, marketName)
	if mc == nil {
		return service.AddResult{}, domain.ErrMarketNotFound
	}

	// Lock installdb
	if err := a.idb.Lock(a.cacheDir); err != nil {
		return service.AddResult{}, err
	}
	defer func() { _ = a.idb.Unlock(a.cacheDir) }()

	// Load installdb
	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return service.AddResult{}, err
	}

	visited := make(map[domain.MctRef]bool)
	var result service.AddResult
	err = a.addInternal(ref, marketName, relPath, cfg, mc, &db, visited, opts, &result)
	return result, err
}

func (a *App) addInternal(
	ref domain.MctRef,
	marketName, relPath string,
	cfg domain.Config,
	mc *domain.MarketConfig,
	db *domain.InstallDatabase,
	visited map[domain.MctRef]bool,
	opts service.AddOpts,
	result *service.AddResult,
) error {
	// Normalize skill directory refs: skills/foo → skills/foo/SKILL.md
	if isSkillDirRef(relPath) {
		relPath = relPath + "/SKILL.md"
		ref = domain.MctRef(marketName + "@" + relPath)
	}

	// Cycle detection
	if visited[ref] {
		return nil
	}
	visited[ref] = true

	if isProfileRef(relPath) {
		opts.Profile = relPath
		return a.addProfile(marketName, relPath, mc, cfg, db, visited, opts, result)
	}

	profile := opts.Profile
	if profile == "" {
		profile = refProfile(ref)
	}

	projectPath := projectPath(cfg.LocalPath)

	// Check if already installed at this location
	pkg := db.FindPackage(marketName, profile)
	if pkg != nil {
		atLocation := false
		for _, loc := range pkg.Locations {
			if loc == projectPath {
				atLocation = true
				break
			}
		}
		if atLocation && fileInPackage(relPath, pkg.Files) {
			return domain.ErrEntryAlreadyInstalled
		}
	}

	clonePath := a.clonePath(marketName)

	content, err := a.git.ReadFileAtRef(clonePath, mc.Branch, relPath, "HEAD")
	if err != nil {
		return err
	}

	fm, err := domain.ParseFrontmatter(content)
	if err != nil {
		return domain.ErrInvalidFrontmatter.Wrap(err)
	}

	entryType := inferEntryType(relPath)
	if entryType == domain.EntryTypeAgent {
		content = domain.StripRequiresSkills(content)
	}

	if opts.DryRun {
		return nil
	}

	localPath, err := a.resolveLocalPath(cfg, relPath)
	if err != nil {
		return err
	}

	// Install files locally and get tool-target write results
	files, err := a.installEntryFiles(clonePath, mc.Branch, relPath, localPath, content)
	if err != nil {
		return err
	}

	headSHA, err := a.git.RemoteHEAD(clonePath, mc.Branch)
	if err != nil {
		return err
	}

	// Write to additional tool targets (non-Claude)
	entryForTransform := domain.Entry{
		Ref:      ref,
		Market:   marketName,
		RelPath:  relPath,
		Filename: filepath.Base(relPath),
		Type:     entryType,
		Profile:  profile,
	}
	twr := a.writeToToolTargets(entryForTransform, content, projectPath)

	// Populate AddResult with tool write info
	if result != nil {
		mergeAddResult(result, twr)
	}

	// Update installdb
	db.AddOrUpdatePackage(marketName, profile, headSHA, files, projectPath)

	// Store tool checksums
	if len(twr.Checksums) > 0 {
		pkg := db.FindPackage(marketName, profile)
		if pkg != nil {
			a.mergeToolChecksums(pkg, twr.Checksums)
		}
	}

	if err := a.idb.Save(a.cacheDir, *db); err != nil {
		return err
	}

	// Resolve dependencies with cycle detection
	if !opts.NoDeps && len(fm.RequiresSkills) > 0 {
		if err := a.resolveDependencies(fm.RequiresSkills, marketName, profile, mc, cfg, db, visited, opts, result); err != nil {
			return err
		}
	}

	// When installing an agent from a hierarchical market (profile-based layout),
	// also install all sibling skills from the same profile.
	if !opts.NoDeps && entryType == domain.EntryTypeAgent && isProfileRef(profile) {
		if err := a.installProfileSkills(marketName, profile, mc, cfg, db, visited, opts, result); err != nil {
			return err
		}
	}

	return nil
}

// installEntryFiles writes skill or agent files to the local .claude/ directory
// and returns the InstalledFiles record.
func (a *App) installEntryFiles(clonePath, branch, relPath, localPath string, content []byte) (domain.InstalledFiles, error) {
	var files domain.InstalledFiles
	entryType := inferEntryType(relPath)

	if entryType == domain.EntryTypeSkill && isDirBasedSkill(relPath) {
		skillDirPath := filepath.Dir(relPath)
		dirFiles, err := a.git.ListDirFiles(clonePath, branch, skillDirPath)
		if err != nil {
			return files, err
		}
		for _, f := range dirFiles {
			fileContent, err := a.git.ReadFileAtRef(clonePath, branch, f, "HEAD")
			if err != nil {
				return files, err
			}
			fileDest := filepath.Join(localPath, filepath.Base(f))
			if err := a.fs.WriteFile(fileDest, fileContent); err != nil {
				return files, err
			}
		}
		files.Skills = append(files.Skills, filepath.Base(skillDirPath))
	} else if entryType == domain.EntryTypeSkill {
		if err := a.fs.WriteFile(localPath, content); err != nil {
			return files, err
		}
		stem := strings.TrimSuffix(filepath.Base(relPath), ".md")
		files.Skills = append(files.Skills, stem)
	} else {
		if err := a.fs.WriteFile(localPath, content); err != nil {
			return files, err
		}
		files.Agents = append(files.Agents, filepath.Base(relPath))
	}

	return files, nil
}

// mergeAddResult populates an AddResult with tool write paths and warnings.
func mergeAddResult(result *service.AddResult, twr toolWriteResult) {
	if len(twr.ToolWrites) > 0 {
		if result.ToolWrites == nil {
			result.ToolWrites = make(map[string]string)
		}
		for k, v := range twr.ToolWrites {
			result.ToolWrites[k] = v
		}
	}
	result.Warnings = append(result.Warnings, twr.Warnings...)
}

// resolveDependencies installs all required skills for an entry, registering
// cross-market dependencies as needed.
func (a *App) resolveDependencies(
	deps []domain.SkillDep,
	marketName string,
	callerProfile string,
	callerMc *domain.MarketConfig,
	cfg domain.Config,
	db *domain.InstallDatabase,
	visited map[domain.MctRef]bool,
	opts service.AddOpts,
	result *service.AddResult,
) error {
	for _, dep := range deps {
		depMarket := marketName
		if dep.Market != "" {
			depMarketName, nameErr := marketNameFromURL(dep.Market)
			if nameErr != nil {
				return nameErr
			}
			depMarket = depMarketName

			depMc := findMarketConfig(cfg, depMarket)
			if depMc == nil {
				if opts.ConfirmMarket == nil || !opts.ConfirmMarket(dep.Market) {
					return &domain.DomainError{
						Code:    "MARKET_NOT_REGISTERED",
						Message: fmt.Sprintf("skill dependency requires market %q which is not registered", dep.Market),
					}
				}
				if _, addErr := a.AddMarket(dep.Market, service.AddMarketOpts{}); addErr != nil {
					return addErr
				}
				newCfg, err := a.cfg.Load(a.configPath)
				if err != nil {
					return err
				}
				cfg = newCfg
			}
		}

		depFile := dep.File
		// For same-market deps in a hierarchical market, paths in requires_skills
		// are relative to the caller's profile (e.g. "skills/foo/SKILL.md" inside
		// agent at profile "dev/agile-team" resolves to
		// "dev/agile-team/skills/foo/SKILL.md"). Skip when:
		//   - cross-market dep (it carries its own market context),
		//   - market is flat (skills_only),
		//   - caller is not actually under a profile directory (isProfileRef false),
		//   - dep path is already profile-rooted.
		if dep.Market == "" && callerMc != nil && !callerMc.SkillsOnly &&
			isProfileRef(callerProfile) && !strings.HasPrefix(depFile, callerProfile+"/") {
			depFile = callerProfile + "/" + depFile
		}
		if !strings.HasSuffix(depFile, ".md") {
			depFile = strings.TrimSuffix(depFile, "/") + "/SKILL.md"
		}
		skillRef := domain.MctRef(depMarket + "@" + depFile)

		depMc := findMarketConfig(cfg, depMarket)
		if depMc == nil {
			continue
		}

		_, depRelPath, err := skillRef.Parse()
		if err != nil {
			return err
		}

		depOpts := service.AddOpts{
			NoDeps:        true,
			ConfirmMarket: opts.ConfirmMarket,
		}
		if err := a.addInternal(skillRef, depMarket, depRelPath, cfg, depMc, db, visited, depOpts, result); err != nil {
			return err
		}
	}
	return nil
}

// installProfileSkills enumerates all skill entries in the given profile
// and installs any that are not already installed. This ensures that when an
// agent is installed, all sibling skills from the same profile are included.
func (a *App) installProfileSkills(
	marketName, profile string,
	mc *domain.MarketConfig,
	cfg domain.Config,
	db *domain.InstallDatabase,
	visited map[domain.MctRef]bool,
	opts service.AddOpts,
	result *service.AddResult,
) error {
	if profile == "" {
		return nil
	}

	clonePath := a.clonePath(marketName)
	mfiles, err := a.git.ReadMarketFiles(clonePath, mc.Branch)
	if err != nil {
		return err
	}

	prefix := profile + "/"
	for _, mf := range mfiles {
		if !strings.HasPrefix(mf.Path, prefix) {
			continue
		}
		if inferEntryType(mf.Path) != domain.EntryTypeSkill {
			continue
		}
		if isReadme(mf.Path) {
			continue
		}
		if !isSkillEntryPoint(mf.Path) {
			continue
		}

		fileRef := domain.MctRef(marketName + "@" + mf.Path)
		_, fileRelPath, err := fileRef.Parse()
		if err != nil {
			return err
		}

		skillOpts := service.AddOpts{
			Profile:       profile,
			NoDeps:        true,
			ConfirmMarket: opts.ConfirmMarket,
			DryRun:        opts.DryRun,
		}
		if err := a.addInternal(fileRef, marketName, fileRelPath, cfg, mc, db, visited, skillOpts, result); err != nil {
			if err == domain.ErrEntryAlreadyInstalled {
				continue
			}
			return err
		}
	}

	return nil
}

func (a *App) addProfile(
	marketName, relPath string,
	mc *domain.MarketConfig,
	cfg domain.Config,
	db *domain.InstallDatabase,
	visited map[domain.MctRef]bool,
	opts service.AddOpts,
	result *service.AddResult,
) error {
	clonePath := a.clonePath(marketName)
	mfiles, err := a.git.ReadMarketFiles(clonePath, mc.Branch)
	if err != nil {
		return err
	}

	projectPath := projectPath(cfg.LocalPath)

	prefix := relPath + "/"
	installedCount := 0
	skippedCount := 0

	for _, mf := range mfiles {
		if !strings.HasPrefix(mf.Path, prefix) {
			continue
		}
		if isReadme(mf.Path) {
			continue
		}
		if mc.SkillsOnly && !isSkillPath(mf.Path, mc.SkillsPath) {
			continue
		}
		fileRef := domain.MctRef(marketName + "@" + mf.Path)
		fileProfile := refProfile(fileRef)

		// Check if already installed at this location via installdb
		pkg := db.FindPackage(marketName, fileProfile)
		if pkg != nil {
			atLocation := false
			for _, loc := range pkg.Locations {
				if loc == projectPath {
					atLocation = true
					break
				}
			}
			if atLocation && fileInPackage(mf.Path, pkg.Files) {
				skippedCount++
				continue
			}
		}

		_, fileRelPath, err := fileRef.Parse()
		if err != nil {
			return err
		}
		if err := a.addInternal(fileRef, marketName, fileRelPath, cfg, mc, db, visited, opts, result); err != nil {
			return err
		}
		installedCount++
	}

	if installedCount == 0 && skippedCount > 0 {
		return domain.ErrEntryAlreadyInstalled
	}
	return nil
}

// fileInPackage checks whether the given relPath corresponds to a file already
// listed in the InstalledFiles struct.
func fileInPackage(relPath string, files domain.InstalledFiles) bool {
	entryType := inferEntryType(relPath)
	switch entryType {
	case domain.EntryTypeAgent:
		name := filepath.Base(relPath)
		for _, a := range files.Agents {
			if a == name {
				return true
			}
		}
	case domain.EntryTypeSkill:
		var name string
		if isDirBasedSkill(relPath) {
			// e.g. "dev/go/skills/bar/SKILL.md" → skill name "bar"
			name = filepath.Base(filepath.Dir(relPath))
		} else {
			// e.g. "dev/go/skills/bar.md" → skill name "bar"
			name = strings.TrimSuffix(filepath.Base(relPath), ".md")
		}
		for _, s := range files.Skills {
			if s == name {
				return true
			}
		}
	}
	return false
}

// isSkillEntryPoint returns true when relPath is a skill's main entry file:
// either a flat skill (e.g. "skills/foo.md") or a dir-based SKILL.md
// (e.g. "skills/foo/SKILL.md"). Support files like "skills/foo/prompt.md"
// return false.
func isSkillEntryPoint(relPath string) bool {
	base := filepath.Base(relPath)
	if isDirBasedSkill(relPath) {
		return base == "SKILL.md"
	}
	return strings.HasSuffix(relPath, ".md")
}

// isDirBasedSkill returns true when relPath is a directory-based skill file
// (e.g. "skills/foo/SKILL.md" or "dev/go/skills/bar/SKILL.md") rather than a
// flat skill .md file (e.g. "skills/bar.md").
func isDirBasedSkill(relPath string) bool {
	parts := strings.Split(relPath, "/")
	for i, p := range parts {
		if p == "skills" && i+2 < len(parts) {
			// There's at least one more directory level between "skills" and the file
			return true
		}
	}
	return false
}

// isSkillDirRef returns true when relPath points to a skill directory
// (e.g. "skills/azure-ai") rather than a concrete file (e.g. "skills/azure-ai/SKILL.md").
func isSkillDirRef(relPath string) bool {
	if strings.HasSuffix(relPath, ".md") {
		return false
	}
	return inferEntryType(relPath) == domain.EntryTypeSkill
}

// isProfileRef returns true when relPath is a profile directory path
// (e.g. "dev/go-hexagonal") rather than a concrete file path.
// A profile path never ends in ".md" and has no "agents" or "skills" segment.
func isProfileRef(relPath string) bool {
	if strings.HasSuffix(relPath, ".md") {
		return false
	}
	for _, seg := range strings.Split(relPath, "/") {
		if seg == "agents" || seg == "skills" {
			return false
		}
	}
	return true
}

func (a *App) Remove(ref domain.MctRef, opts service.RemoveOpts) (service.RemoveResult, error) {
	// Normalize skill directory refs: skills/foo → skills/foo/SKILL.md
	marketName, relPath, err := ref.Parse()
	if err != nil {
		return service.RemoveResult{}, err
	}
	if isSkillDirRef(relPath) {
		relPath = relPath + "/SKILL.md"
		ref = domain.MctRef(marketName + "@" + relPath)
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return service.RemoveResult{}, err
	}

	// Lock installdb
	if err := a.idb.Lock(a.cacheDir); err != nil {
		return service.RemoveResult{}, err
	}
	defer func() { _ = a.idb.Unlock(a.cacheDir) }()

	// Load installdb
	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return service.RemoveResult{}, err
	}

	profile := refProfile(ref)

	// Find the package in installdb
	pkg := db.FindPackage(marketName, profile)
	if pkg == nil {
		return service.RemoveResult{}, domain.ErrEntryNotInstalled
	}

	projectPath := projectPath(cfg.LocalPath)

	// Build an entry for tool removal
	entryForRemove := domain.Entry{
		Ref:      ref,
		Market:   marketName,
		RelPath:  relPath,
		Filename: filepath.Base(relPath),
		Type:     inferEntryType(relPath),
		Profile:  profile,
	}

	var removeResult service.RemoveResult
	// Always include "claude" as a removed tool (the main .claude/ files)
	removeResult.ToolsRemoved = append(removeResult.ToolsRemoved, "claude")

	if opts.AllLocations {
		// Delete files from every location
		for _, loc := range pkg.Locations {
			localPath := filepath.Join(loc, filepath.Base(cfg.LocalPath))
			if err := a.deleteInstalledFiles(localPath, pkg.Files); err != nil {
				return service.RemoveResult{}, err
			}
			tr := a.removeFromToolTargets(entryForRemove, loc)
			removeResult.ToolsRemoved = append(removeResult.ToolsRemoved, tr.Removed...)
		}
		// Remove all locations (remove the package entirely)
		locs := make([]string, len(pkg.Locations))
		copy(locs, pkg.Locations)
		for _, loc := range locs {
			db.RemoveLocation(marketName, profile, loc)
		}
	} else {
		// Check that the current project path is in the package's Locations
		atLocation := false
		for _, loc := range pkg.Locations {
			if loc == projectPath {
				atLocation = true
				break
			}
		}
		if !atLocation {
			return service.RemoveResult{}, domain.ErrEntryNotInstalled
		}

		// Delete files from current project
		if err := a.deleteInstalledFiles(cfg.LocalPath, pkg.Files); err != nil {
			return service.RemoveResult{}, err
		}

		// Remove tool-specific files
		tr := a.removeFromToolTargets(entryForRemove, projectPath)
		removeResult.ToolsRemoved = append(removeResult.ToolsRemoved, tr.Removed...)

		// Update installdb
		db.RemoveLocation(marketName, profile, projectPath)
	}

	// Save installdb
	return removeResult, a.idb.Save(a.cacheDir, db)
}

func (a *App) deleteInstalledFiles(localPath string, files domain.InstalledFiles) error {
	for _, skill := range files.Skills {
		if err := a.fs.RemoveAll(filepath.Join(localPath, "skills", skill)); err != nil {
			return err
		}
	}
	for _, agent := range files.Agents {
		if err := a.fs.DeleteFile(filepath.Join(localPath, "agents", agent)); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) Prune(opts service.PruneOpts) ([]service.PruneResult, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	// Lock installdb
	if err := a.idb.Lock(a.cacheDir); err != nil {
		return nil, err
	}
	defer func() { _ = a.idb.Unlock(a.cacheDir) }()

	// Load installdb
	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return nil, err
	}

	projectPath := projectPath(cfg.LocalPath)

	var results []service.PruneResult

	for _, im := range db.Markets {
		mc := findMarketConfig(cfg, im.Market)
		if mc == nil {
			continue
		}
		clonePath := a.clonePath(im.Market)

		for _, pkg := range im.Packages {
			// Check if this package is installed at the current project path
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

			// Check if any of the files still exist in the cached clone
			allGone := true
			for _, skill := range pkg.Files.Skills {
				// Try to find the skill file in the clone
				_, err := a.git.FileVersion(clonePath, skill)
				if err == nil {
					allGone = false
					break
				}
			}
			if allGone {
				for _, agent := range pkg.Files.Agents {
					_, err := a.git.FileVersion(clonePath, agent)
					if err == nil {
						allGone = false
						break
					}
				}
			}

			if !allGone {
				continue
			}

			ref := domain.MctRef(pkg.Profile)

			if opts.AllKeep {
				results = append(results, service.PruneResult{Ref: ref, Action: "kept"})
			} else if opts.AllRemove {
				if err := a.deleteInstalledFiles(cfg.LocalPath, pkg.Files); err != nil {
					results = append(results, service.PruneResult{Ref: ref, Action: "remove", Err: err})
					continue
				}
				db.RemoveLocation(im.Market, pkg.Profile, projectPath)
				results = append(results, service.PruneResult{Ref: ref, Action: "removed"})
			}
		}
	}

	// Save installdb
	if err := a.idb.Save(a.cacheDir, db); err != nil {
		return nil, err
	}

	return results, nil
}

func (a *App) Init(opts service.InitOpts) error {
	localPath := opts.LocalPath
	if localPath == "" {
		localPath = "."
	}

	if a.cfg.Exists(a.configPath) {
		return domain.ErrAlreadyInitialized
	}

	// Ensure cache and config directories exist.
	if err := a.fs.MkdirAll(a.cacheDir); err != nil {
		return err
	}
	configDir := filepath.Dir(a.configPath)
	if configDir != "" && configDir != "." {
		if err := a.fs.MkdirAll(configDir); err != nil {
			return err
		}
	}

	markets := opts.Markets
	if markets == nil {
		defaults, err := loadDefaultSkillMarkets()
		if err != nil {
			return err
		}
		markets = defaults
	}

	cfg := domain.Config{
		LocalPath: localPath,
		Tools:     map[string]bool{"claude": true},
		Markets:   []domain.MarketConfig{},
	}

	for _, url := range markets {
		name, err := marketNameFromURL(url)
		if err != nil {
			return err
		}
		clonePath := filepath.Join(a.cacheDir, marketDirName(name))
		if _, statErr := os.Stat(clonePath); statErr == nil {
			// Clone already exists (e.g. previous failed init), just fetch.
			branch, brErr := a.git.DefaultBranch(clonePath)
			if brErr != nil {
				return brErr
			}
			if _, fetchErr := a.git.Fetch(clonePath, branch); fetchErr != nil {
				return fetchErr
			}
		} else {
			if err := a.git.Clone(url, clonePath); err != nil {
				return err
			}
		}

		branch, err := a.git.DefaultBranch(clonePath)
		if err != nil {
			return err
		}
		mc := domain.MarketConfig{
			Name:   name,
			URL:    url,
			Branch: branch,
		}
		cfg.Markets = append(cfg.Markets, mc)

		sha, err := a.git.RemoteHEAD(clonePath, branch)
		if err != nil {
			return err
		}
		if err := a.state.SetMarketSyncClean(a.cacheDir, name, sha); err != nil {
			return err
		}
	}

	if err := a.cfg.Save(a.configPath, cfg); err != nil {
		return err
	}

	// Write default tool-mappings.yml if it doesn't already exist.
	if a.toolMappings != nil {
		mappingsPath := filepath.Join(configDir, "tool-mappings.yml")
		if !a.toolMappings.ToolMappingsExist(mappingsPath) {
			defaults := a.toolMappings.DefaultToolMappings()
			if err := a.toolMappings.SaveToolMappings(mappingsPath, defaults); err != nil {
				return err
			}
		}
	}

	return nil
}

type defaultMarket struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func loadDefaultSkillMarkets() ([]string, error) {
	data, err := assets.FS.ReadFile("skills.json")
	if err != nil {
		return nil, fmt.Errorf("reading embedded skills.json: %w", err)
	}
	var markets []defaultMarket
	if err := json.Unmarshal(data, &markets); err != nil {
		return nil, fmt.Errorf("parsing embedded skills.json: %w", err)
	}
	urls := make([]string, len(markets))
	for i, m := range markets {
		urls[i] = m.URL
	}
	return urls, nil
}

func (a *App) resolveLocalPath(cfg domain.Config, relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
		return "", &domain.DomainError{
			Code:    "UNSAFE_PATH",
			Message: "entry path must be relative and cannot traverse upward: " + relPath,
		}
	}

	filename := filepath.Base(cleaned)
	parts := strings.Split(cleaned, string(filepath.Separator))
	for i, p := range parts {
		if p == "agents" {
			return filepath.Join(cfg.LocalPath, p, filename), nil
		}
		if p == "skills" {
			// For directory-based skills: return <localPath>/skills/<skillname>/
			if i+2 < len(parts) {
				skillName := parts[i+1]
				return filepath.Join(cfg.LocalPath, p, skillName), nil
			}
			// Flat file: skills/bar.md → <localPath>/skills/bar
			stem := strings.TrimSuffix(filename, ".md")
			return filepath.Join(cfg.LocalPath, p, stem), nil
		}
	}
	return filepath.Join(cfg.LocalPath, filename), nil
}

// refProfile extracts the profile portion from a full ref (without the market prefix).
// "market@seg1/seg2/agents/foo.md" -> "seg1/seg2"
// "market@skills/foo/SKILL.md"     -> "skills/foo"
// Falls back to "" if there are fewer than 2 path segments.
func refProfile(ref domain.MctRef) string {
	_, relPath, err := ref.Parse()
	if err != nil {
		return string(ref)
	}
	parts := strings.Split(relPath, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return ""
}

func inferEntryType(relPath string) domain.EntryType {
	parts := strings.Split(relPath, "/")
	for _, p := range parts {
		switch p {
		case "agents":
			return domain.EntryTypeAgent
		case "skills":
			return domain.EntryTypeSkill
		}
	}
	return ""
}
