package app

import (
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
				MctProfile:        fm.MctProfile,
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

	version, err := a.git.FileVersion(clonePath, relPath)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	checksum := a.fs.MD5Checksum(content)

	injected, err := domain.InjectMctFields(content, ref, version, marketName, checksum, opts.Profile)
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

	if !opts.NoDeps && len(fm.RequiresSkills) > 0 {
		for _, dep := range fm.RequiresSkills {
			skillRef := domain.MctRef(marketName + "/" + dep.File)

			existing, ok := installed.Entries[skillRef]
			if ok && existing != nil {
				continue
			}

			depOpts := service.AddOpts{
				NoDeps: true,
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
	clonePath := filepath.Join(a.cacheDir, marketName)
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
		fileRef := domain.MctRef(marketName + "/" + mf.Path)
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
// "market/seg1/seg2/agents/foo.md" -> "market/seg1/seg2"
// Falls back to "market" if there are fewer than 3 segments.
func refProfile(ref domain.MctRef) string {
	parts := strings.Split(string(ref), "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return parts[0]
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
