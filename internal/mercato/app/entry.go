package app

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	fsrepo "github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)


func (a *App) scanInstalledEntries(cfg domain.Config) (domain.ChecksumState, error) {
	state := domain.ChecksumState{
		Version: 1,
		Entries: make(map[domain.MctRef]*domain.ChecksumEntry),
	}

	// Build a reverse lookup: cache dir name -> market name
	dirToMarket := make(map[string]string, len(cfg.Markets))
	for _, mc := range cfg.Markets {
		dirToMarket[marketDirName(mc.Name)] = mc.Name
	}

	// Scan agents: symlinked .md files
	agentsDir := filepath.Join(cfg.LocalPath, "agents")
	if fsrepo.DirExists(a.fs, agentsDir) {
		files, err := listMdFiles(a.fs, agentsDir)
		if err == nil {
			for _, filePath := range files {
				if ce := a.resolveSymlinkEntry(filePath, dirToMarket); ce != nil {
					state.Entries[ce.MctRef] = ce
				}
			}
		}
	}

	// Scan skills: symlinked directories (e.g. .claude/skills/go-architect → cache dir)
	skillsDir := filepath.Join(cfg.LocalPath, "skills")
	if fsrepo.DirExists(a.fs, skillsDir) {
		entries, err := a.fs.ListDir(skillsDir)
		if err == nil {
			for _, name := range entries {
				dirPath := filepath.Join(skillsDir, name)
				if ce := a.resolveSymlinkEntry(dirPath, dirToMarket); ce != nil {
					state.Entries[ce.MctRef] = ce
				}
			}
		}
	}

	return state, nil
}

// resolveSymlinkEntry checks if path is a symlink pointing into the cache dir
// and returns a ChecksumEntry if so.
func (a *App) resolveSymlinkEntry(path string, dirToMarket map[string]string) *domain.ChecksumEntry {
	if !a.fs.IsSymlink(path) {
		return nil
	}
	target, err := a.fs.Readlink(path)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(a.cacheDir, target)
	if err != nil {
		return nil
	}
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if len(parts) < 2 {
		return nil
	}
	marketName, ok := dirToMarket[parts[0]]
	if !ok {
		return nil
	}
	// For skill directories, the target is the dir; add /SKILL.md for the ref.
	relPath := parts[1]
	if inferEntryType(relPath) == domain.EntryTypeSkill && !strings.HasSuffix(relPath, ".md") {
		relPath += "/SKILL.md"
	}
	ref := domain.MctRef(marketName + "@" + relPath)
	return &domain.ChecksumEntry{
		LocalPath: path,
		MctRef:    ref,
	}
}

func (a *App) List(opts service.ListOpts) ([]domain.Entry, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	checksums, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return nil, err
	}

	var entries []domain.Entry
	for ref, ce := range checksums.Entries {
		if ce == nil {
			continue
		}
		entry := checksumEntryToDomainEntry(ref, ce)

		if opts.Market != "" && ref.Market() != opts.Market {
			continue
		}
		if opts.Type != "" && entry.Type != opts.Type {
			continue
		}
		if opts.Installed && (!entry.Installed || !fsrepo.Exists(a.fs, ce.LocalPath)) {
			continue
		}

		entries = append(entries, entry)
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
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return domain.Entry{}, err
	}

	checksums, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return domain.Entry{}, err
	}

	ce, ok := checksums.Entries[ref]
	if !ok || ce == nil {
		return domain.Entry{}, domain.ErrEntryNotFound
	}

	return checksumEntryToDomainEntry(ref, ce), nil
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

	installed, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return err
	}

	// Normalize skill directory refs: skills/foo → skills/foo/SKILL.md
	if isSkillDirRef(relPath) {
		relPath = relPath + "/SKILL.md"
		ref = domain.MctRef(marketName + "@" + relPath)
	}

	if isProfileRef(relPath) {
		opts.Profile = string(ref)
		return a.addProfile(marketName, relPath, mc, installed, opts)
	}

	if opts.Profile == "" {
		opts.Profile = refProfile(ref)
	}

	if existing, ok := installed.Entries[ref]; ok && existing != nil {
		return domain.ErrEntryAlreadyInstalled
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

	// Create symlink from local install path to the cached market file/dir
	localPath, err := a.resolveLocalPath(cfg, relPath)
	if err != nil {
		return err
	}

	target := filepath.Join(clonePath, relPath)
	// For skills, symlink the directory (not the file inside it).
	if inferEntryType(relPath) == domain.EntryTypeSkill {
		target = filepath.Dir(target)
	}
	if err := a.fs.Symlink(target, localPath); err != nil {
		return err
	}

	if !opts.NoDeps && len(fm.RequiresSkills) > 0 {
		for _, dep := range fm.RequiresSkills {
			depMarket := marketName
			if dep.Market != "" {
				// Cross-market dependency: resolve the market name from URL
				depMarketName, nameErr := marketNameFromURL(dep.Market)
				if nameErr != nil {
					return nameErr
				}
				depMarket = depMarketName

				// Check if the market is registered; if not, ask user to add it
				mc := findMarketConfig(cfg, depMarket)
				if mc == nil {
					if opts.ConfirmMarket == nil || !opts.ConfirmMarket(dep.Market) {
						return &domain.DomainError{
							Code:    "MARKET_NOT_REGISTERED",
							Message: fmt.Sprintf("skill dependency requires market %q which is not registered", dep.Market),
						}
					}
					if _, addErr := a.AddMarket(dep.Market, service.AddMarketOpts{}); addErr != nil {
						return addErr
					}
				}
			}

			depFile := dep.File
			// requires_skills may point to a skill directory (no .md suffix).
			// Normalize to the SKILL.md file inside it.
			if !strings.HasSuffix(depFile, ".md") {
				depFile = strings.TrimSuffix(depFile, "/") + "/SKILL.md"
			}
			skillRef := domain.MctRef(depMarket + "@" + depFile)

			existing, ok := installed.Entries[skillRef]
			if ok && existing != nil {
				continue
			}

			depOpts := service.AddOpts{
				NoDeps:        true,
				ConfirmMarket: opts.ConfirmMarket,
			}
			if err := a.Add(skillRef, depOpts); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *App) addProfile(
	marketName, relPath string,
	mc *domain.MarketConfig,
	installed domain.ChecksumState,
	opts service.AddOpts,
) error {
	clonePath := a.clonePath(marketName)
	mfiles, err := a.git.ReadMarketFiles(clonePath, mc.Branch)
	if err != nil {
		return err
	}

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
		if existing, ok := installed.Entries[fileRef]; ok && existing != nil {
			skippedCount++
			continue
		}
		if err := a.Add(fileRef, opts); err != nil {
			return err
		}
		installedCount++
	}

	if installedCount == 0 && skippedCount > 0 {
		return domain.ErrEntryAlreadyInstalled
	}
	return nil
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

func (a *App) Remove(ref domain.MctRef) error {
	// Normalize skill directory refs: skills/foo → skills/foo/SKILL.md
	if marketName, relPath, err := ref.Parse(); err == nil && isSkillDirRef(relPath) {
		ref = domain.MctRef(marketName + "@" + relPath + "/SKILL.md")
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return err
	}

	installed, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return err
	}

	ce, ok := installed.Entries[ref]
	if !ok || ce == nil {
		return domain.ErrEntryNotInstalled
	}

	return a.deleteEntryFile(ce.LocalPath)
}

func (a *App) Prune(opts service.PruneOpts) ([]service.PruneResult, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	installed, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return nil, err
	}

	var results []service.PruneResult

	for ref, ce := range installed.Entries {
		if ce == nil {
			continue
		}

		marketName, relPath, err := ref.Parse()
		if err != nil {
			continue
		}

		clonePath := a.clonePath(marketName)
		_, err = a.git.FileVersion(clonePath, relPath)
		if err == nil {
			continue
		}

		if opts.AllKeep {
			results = append(results, service.PruneResult{Ref: ref, Action: "kept"})
		} else if opts.AllRemove {
			if err := a.deleteEntryFile(ce.LocalPath); err != nil {
				results = append(results, service.PruneResult{Ref: ref, Action: "remove", Err: err})
				continue
			}
			results = append(results, service.PruneResult{Ref: ref, Action: "removed"})
		}
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
			// Symlink the skill directory, not individual files.
			// Directory-based: skills/go-architect/SKILL.md → symlink .claude/skills/go-architect
			if i+2 < len(parts) {
				skillName := parts[i+1]
				return filepath.Join(cfg.LocalPath, p, skillName), nil
			}
			// Flat file: skills/bar.md → symlink .claude/skills/bar
			stem := strings.TrimSuffix(filename, ".md")
			return filepath.Join(cfg.LocalPath, p, stem), nil
		}
	}
	return filepath.Join(cfg.LocalPath, filename), nil
}

func checksumEntryToDomainEntry(ref domain.MctRef, ce *domain.ChecksumEntry) domain.Entry {
	marketName := ref.Market()
	relPath := ref.RelPath()
	entryType := inferEntryType(relPath)

	profile := ce.MctProfile
	if profile == "" {
		profile = refProfile(ref)
	}

	return domain.Entry{
		Ref:       ref,
		Market:    marketName,
		RelPath:   relPath,
		Filename:  filepath.Base(relPath),
		Type:      entryType,
		Version:   ce.MctVersion,
		Profile:   profile,
		Installed: true,
	}
}

// refProfile extracts the profile portion from a full ref.
// "market@seg1/seg2/agents/foo.md" -> "market@seg1/seg2"
// Falls back to "market" if there are fewer than 2 path segments.
func refProfile(ref domain.MctRef) string {
	market, relPath, err := ref.Parse()
	if err != nil {
		return string(ref)
	}
	parts := strings.Split(relPath, "/")
	if len(parts) >= 2 {
		return market + "@" + parts[0] + "/" + parts[1]
	}
	return market
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

// deleteEntryFile deletes the entry file. For skills installed as
// skills/<name>/SKILL.md, it also removes the now-empty parent directory.
func (a *App) deleteEntryFile(localPath string) error {
	return a.fs.DeleteFile(localPath)
}

func listMdFiles(fsys fs.ReadDirFS, dir string) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
