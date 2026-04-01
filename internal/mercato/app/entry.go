package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
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

func (a *App) Add(ref domain.MctRef, opts service.AddOpts) error {
	marketName, relPath, err := ref.Parse()
	if err != nil {
		return err
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return err
	}

	mc := findMarketConfig(cfg, marketName)
	if mc == nil {
		return domain.ErrMarketNotFound
	}

	// Lock installdb
	if err := a.idb.Lock(a.cacheDir); err != nil {
		return err
	}
	defer a.idb.Unlock(a.cacheDir)

	// Load installdb
	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return err
	}

	visited := make(map[domain.MctRef]bool)
	return a.addInternal(ref, marketName, relPath, cfg, mc, &db, visited, opts)
}

func (a *App) addInternal(
	ref domain.MctRef,
	marketName, relPath string,
	cfg domain.Config,
	mc *domain.MarketConfig,
	db *domain.InstallDatabase,
	visited map[domain.MctRef]bool,
	opts service.AddOpts,
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
		return a.addProfile(marketName, relPath, mc, cfg, db, visited, opts)
	}

	profile := opts.Profile
	if profile == "" {
		profile = refProfile(ref)
	}

	// Determine project path from cfg.LocalPath
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

	// Verify the file exists in the market
	content, err := a.git.ReadFileAtRef(clonePath, mc.Branch, relPath, "HEAD")
	if err != nil {
		return err
	}

	fm, err := domain.ParseFrontmatter(content)
	if err != nil {
		return domain.ErrInvalidFrontmatter.Wrap(err)
	}

	if opts.DryRun {
		return nil
	}

	// Resolve local install path
	localPath, err := a.resolveLocalPath(cfg, relPath)
	if err != nil {
		return err
	}

	// Build InstalledFiles
	var files domain.InstalledFiles
	entryType := inferEntryType(relPath)

	if entryType == domain.EntryTypeSkill && isDirBasedSkill(relPath) {
		// Directory-based skill (e.g. skills/foo/SKILL.md): enumerate all files
		// in the skill directory and copy them.
		skillDirPath := filepath.Dir(relPath)
		dirFiles, err := a.git.ListDirFiles(clonePath, mc.Branch, skillDirPath)
		if err != nil {
			return err
		}
		for _, f := range dirFiles {
			fileContent, err := a.git.ReadFileAtRef(clonePath, mc.Branch, f, "HEAD")
			if err != nil {
				return err
			}
			// localPath is <cfg.LocalPath>/skills/<skillname>, so each file goes under it
			fileName := filepath.Base(f)
			fileDest := filepath.Join(localPath, fileName)
			if err := a.fs.WriteFile(fileDest, fileContent); err != nil {
				return err
			}
		}
		skillName := filepath.Base(skillDirPath)
		files.Skills = append(files.Skills, skillName)
	} else if entryType == domain.EntryTypeSkill {
		// Flat skill file (e.g. skills/bar.md): copy the single file.
		if err := a.fs.WriteFile(localPath, content); err != nil {
			return err
		}
		stem := strings.TrimSuffix(filepath.Base(relPath), ".md")
		files.Skills = append(files.Skills, stem)
	} else {
		// For agents: write the .md file directly
		if err := a.fs.WriteFile(localPath, content); err != nil {
			return err
		}
		files.Agents = append(files.Agents, filepath.Base(relPath))
	}

	// Get current HEAD SHA for version
	headSHA, err := a.git.RemoteHEAD(clonePath, mc.Branch)
	if err != nil {
		return err
	}

	// Update installdb
	db.AddOrUpdatePackage(marketName, profile, headSHA, files, projectPath)

	// Save installdb
	if err := a.idb.Save(a.cacheDir, *db); err != nil {
		return err
	}

	// Resolve dependencies with cycle detection
	if !opts.NoDeps && len(fm.RequiresSkills) > 0 {
		for _, dep := range fm.RequiresSkills {
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
					// Reload config to pick up the new market
					newCfg, err := a.cfg.Load(a.configPath)
					if err != nil {
						return err
					}
					cfg = newCfg
				}
			}

			depFile := dep.File
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
			if err := a.addInternal(skillRef, depMarket, depRelPath, cfg, depMc, db, visited, depOpts); err != nil {
				return err
			}
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
		if err := a.addInternal(fileRef, marketName, fileRelPath, cfg, mc, db, visited, opts); err != nil {
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

func (a *App) Remove(ref domain.MctRef, opts service.RemoveOpts) error {
	// Normalize skill directory refs: skills/foo → skills/foo/SKILL.md
	marketName, relPath, err := ref.Parse()
	if err != nil {
		return err
	}
	if isSkillDirRef(relPath) {
		relPath = relPath + "/SKILL.md"
		ref = domain.MctRef(marketName + "@" + relPath)
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return err
	}

	// Lock installdb
	if err := a.idb.Lock(a.cacheDir); err != nil {
		return err
	}
	defer a.idb.Unlock(a.cacheDir)

	// Load installdb
	db, err := a.idb.Load(a.cacheDir)
	if err != nil {
		return err
	}

	profile := refProfile(ref)

	// Find the package in installdb
	pkg := db.FindPackage(marketName, profile)
	if pkg == nil {
		return domain.ErrEntryNotInstalled
	}

	projectPath := projectPath(cfg.LocalPath)

	if opts.AllLocations {
		// Delete files from every location
		for _, loc := range pkg.Locations {
			localPath := filepath.Join(loc, filepath.Base(cfg.LocalPath))
			if err := a.deleteInstalledFiles(localPath, pkg.Files); err != nil {
				return err
			}
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
			return domain.ErrEntryNotInstalled
		}

		// Delete files from current project
		if err := a.deleteInstalledFiles(cfg.LocalPath, pkg.Files); err != nil {
			return err
		}

		// Update installdb
		db.RemoveLocation(marketName, profile, projectPath)
	}

	// Save installdb
	return a.idb.Save(a.cacheDir, db)
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
	defer a.idb.Unlock(a.cacheDir)

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

	if err := a.fs.MkdirAll(a.cacheDir); err != nil {
		return err
	}

	cfg := domain.Config{
		LocalPath: localPath,
		Markets:   []domain.MarketConfig{},
	}

	for _, url := range opts.Markets {
		name, err := marketNameFromURL(url)
		if err != nil {
			return err
		}
		clonePath := filepath.Join(a.cacheDir, marketDirName(name))
		if err := a.git.Clone(url, clonePath); err != nil {
			return err
		}
		mc := domain.MarketConfig{
			Name:   name,
			URL:    url,
			Branch: "main",
		}
		cfg.Markets = append(cfg.Markets, mc)

		sha, err := a.git.RemoteHEAD(clonePath, "main")
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

	return nil
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

