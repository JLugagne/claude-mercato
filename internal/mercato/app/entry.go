package app

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	fsrepo "github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

type DiffInfo struct {
	LeftPath  string
	RightPath string
	Tool      string
}

func (a *App) scanInstalledEntries(cfg domain.Config) (domain.ChecksumState, error) {
	state := domain.ChecksumState{
		Version: 1,
		Entries: make(map[domain.MctRef]*domain.ChecksumEntry),
	}

	for _, subdir := range []string{"agents", "skills"} {
		dir := filepath.Join(cfg.LocalPath, subdir)
		if !fsrepo.DirExists(a.fs, dir) {
			continue
		}
		files, err := listMdFiles(a.fs, dir)
		if err != nil {
			continue
		}
		for _, filePath := range files {
			content, err := a.fs.ReadFile(filePath)
			if err != nil {
				continue
			}
			fm, err := domain.ParseFrontmatter(content)
			if err != nil {
				continue
			}
			if fm.MctRef == "" {
				continue
			}
			ce := &domain.ChecksumEntry{
				LocalPath:         filePath,
				MctRef:            fm.MctRef,
				MctVersion:        fm.MctVersion,
				InstalledAt:       fm.MctInstalledAt,
				ChecksumAtInstall: fm.MctChecksum,
			}
			state.Entries[fm.MctRef] = ce
		}
	}

	return state, nil
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
		if opts.Installed && (!entry.Installed || !fsrepo.FileExists(a.fs, ce.LocalPath)) {
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

	clonePath := filepath.Join(a.cacheDir, market)
	return a.git.ReadFileAtRef(clonePath, mc.Branch, relPath, "HEAD")
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
	if existing, ok := installed.Entries[ref]; ok && existing != nil {
		return domain.ErrEntryAlreadyInstalled
	}

	clonePath := filepath.Join(a.cacheDir, marketName)

	content, err := a.git.ReadFileAtRef(clonePath, mc.Branch, relPath, "HEAD")
	if err != nil {
		return err
	}

	fm, err := domain.ParseFrontmatter(content)
	if err != nil {
		return domain.ErrInvalidFrontmatter.Wrap(err)
	}

	if fm.MctRef != "" || fm.MctVersion != "" || fm.MctMarket != "" {
		return domain.ErrMctFieldsInRepo
	}

	if err := validateEntryType(fm.Type, relPath); err != nil {
		return err
	}

	version, err := a.git.FileVersion(clonePath, relPath)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	checksum := a.fs.MD5Checksum(content)

	injected, err := domain.InjectMctFields(content, ref, version, marketName, checksum)
	if err != nil {
		return err
	}

	localPath, err := a.resolveLocalPath(cfg, relPath)
	if err != nil {
		return err
	}

	if err := a.fs.WriteFile(localPath, injected); err != nil {
		return err
	}

	ec := domain.EntryConfig{
		Ref: ref,
		Pin: opts.Pin,
	}
	if err := a.cfg.AddEntry(a.configPath, ec); err != nil {
		return err
	}

	if !opts.NoDeps && len(fm.RequiresSkills) > 0 {
		for _, dep := range fm.RequiresSkills {
			skillRef := domain.MctRef(marketName + "/" + dep.File)

			existing, ok := installed.Entries[skillRef]
			if ok && existing != nil {
				continue
			}

			depOpts := service.AddOpts{
				NoDeps: true,
				Pin:    dep.Pin,
			}
			if err := a.Add(skillRef, depOpts); err != nil {
				return err
			}

			managedSkill := domain.ManagedSkillConfig{
				Ref:        skillRef,
				ManagedBy:  ref,
				MctVersion: version,
			}
			if err := a.cfg.AddManagedSkill(a.configPath, managedSkill); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *App) Remove(ref domain.MctRef) error {
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

	if err := a.deleteEntryFile(ce.LocalPath); err != nil {
		return err
	}

	if err := a.cfg.RemoveEntry(a.configPath, ref); err != nil {
		return err
	}

	if err := a.cfg.RemoveManagedSkill(a.configPath, ref); err != nil {
		return err
	}

	return nil
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
			if err := a.cfg.RemoveEntry(a.configPath, ref); err != nil {
				results = append(results, service.PruneResult{Ref: ref, Action: "remove", Err: err})
				continue
			}
			results = append(results, service.PruneResult{Ref: ref, Action: "removed"})
		}
	}

	return results, nil
}

func (a *App) Pin(ref domain.MctRef, sha string) error {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return err
	}

	installed, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return err
	}

	if _, ok := installed.Entries[ref]; !ok {
		return domain.ErrEntryNotInstalled
	}

	return a.cfg.SetEntryPin(a.configPath, ref, sha)
}

func (a *App) Diff(ref domain.MctRef) error {
	_, _, _, err := a.PrepareDiff(ref)
	return err
}

func (a *App) PrepareDiff(ref domain.MctRef) (leftTmpPath, rightPath, tool string, err error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return "", "", "", err
	}

	installed, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return "", "", "", err
	}

	ce, ok := installed.Entries[ref]
	if !ok || ce == nil {
		return "", "", "", domain.ErrEntryNotFound
	}

	marketName, relPath, err := ref.Parse()
	if err != nil {
		return "", "", "", err
	}

	marketCfg := findMarketConfig(cfg, marketName)
	if marketCfg == nil {
		return "", "", "", domain.ErrMarketNotFound
	}

	clonePath := filepath.Join(a.cacheDir, marketName)
	registryContent, err := a.git.ReadFileAtRef(clonePath, marketCfg.Branch, relPath, "HEAD")
	if err != nil {
		return "", "", "", err
	}

	filename := filepath.Base(relPath)
	tmpPath, err := a.fs.TempFile(filename, registryContent)
	if err != nil {
		return "", "", "", err
	}

	resolvedTool := resolveDifftool(cfg.Difftool, a.git)

	return tmpPath, ce.LocalPath, resolvedTool, nil
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
		Entries:   []domain.EntryConfig{},
	}

	for name, url := range opts.Markets {
		clonePath := filepath.Join(a.cacheDir, name)
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

	agentsDir := filepath.Join(localPath, "agents")
	skillsDir := filepath.Join(localPath, "skills")

	for _, dir := range []string{agentsDir, skillsDir} {
		if !fsrepo.DirExists(a.fs, dir) {
			continue
		}
		files, err := listMdFiles(a.fs, dir)
		if err != nil {
			continue
		}
		for _, filePath := range files {
			content, err := a.fs.ReadFile(filePath)
			if err != nil {
				continue
			}
			fm, err := domain.ParseFrontmatter(content)
			if err != nil {
				continue
			}
			if fm.MctRef == "" {
				continue
			}
			cfg.Entries = append(cfg.Entries, domain.EntryConfig{Ref: fm.MctRef})
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
	stem := strings.TrimSuffix(filename, ".md")
	parts := strings.Split(cleaned, string(filepath.Separator))
	for _, p := range parts {
		if p == "agents" {
			return filepath.Join(cfg.LocalPath, p, filename), nil
		}
		if p == "skills" {
			return filepath.Join(cfg.LocalPath, p, stem, "SKILL.md"), nil
		}
	}
	return filepath.Join(cfg.LocalPath, filename), nil
}

func checksumEntryToDomainEntry(ref domain.MctRef, ce *domain.ChecksumEntry) domain.Entry {
	marketName := ref.Market()
	relPath := ref.RelPath()
	entryType := inferEntryType(relPath)

	return domain.Entry{
		Ref:       ref,
		Market:    marketName,
		RelPath:   relPath,
		Filename:  filepath.Base(relPath),
		Type:      entryType,
		Version:   ce.MctVersion,
		Installed: true,
	}
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

func validateEntryType(t domain.EntryType, relPath string) error {
	parts := strings.Split(relPath, "/")
	for _, p := range parts {
		switch p {
		case "agents":
			if t != domain.EntryTypeAgent {
				return domain.ErrInvalidEntryType
			}
			return nil
		case "skills":
			if t != domain.EntryTypeSkill {
				return domain.ErrInvalidEntryType
			}
			return nil
		}
	}
	return nil
}

func resolveDifftool(configuredTool string, git interface{ ReadGlobalDifftool() (string, error) }) string {
	if configuredTool != "" {
		return configuredTool
	}
	if tool, err := git.ReadGlobalDifftool(); err == nil && tool != "" {
		return tool
	}
	return "diff"
}

// deleteEntryFile deletes the entry file. For skills installed as
// skills/<name>/SKILL.md, it also removes the now-empty parent directory.
func (a *App) deleteEntryFile(localPath string) error {
	if err := a.fs.DeleteFile(localPath); err != nil {
		return err
	}
	if filepath.Base(localPath) == "SKILL.md" {
		_ = a.fs.RemoveAll(filepath.Dir(localPath))
	}
	return nil
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
