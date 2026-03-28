package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/filesystem"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

var marketNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

type App struct {
	git        gitrepo.GitRepo
	fs         filesystem.Filesystem
	cfg        configstore.ConfigStore
	state      statestore.StateStore
	configPath string
	cacheDir   string
}

func New(git gitrepo.GitRepo, fs filesystem.Filesystem, cfg configstore.ConfigStore, state statestore.StateStore, configPath, cacheDir string) *App {
	return &App{
		git:        git,
		fs:         fs,
		cfg:        cfg,
		state:      state,
		configPath: configPath,
		cacheDir:   cacheDir,
	}
}

func validateMarketName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return domain.ErrInvalidMarketName
	}
	if !marketNameRegexp.MatchString(name) {
		return domain.ErrInvalidMarketName
	}
	return nil
}

// normalizeURL strips protocol prefixes, trailing .git, and trailing slashes
// so that "git@github.com:org/repo.git", "https://github.com/org/repo", and
// "https://github.com/org/repo.git" all compare as equal.
func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	// SSH shorthand: git@host:path -> host/path
	if strings.Contains(u, "://") {
		u = u[strings.Index(u, "://")+3:]
	} else if at := strings.Index(u, "@"); at >= 0 {
		u = u[at+1:]
		u = strings.Replace(u, ":", "/", 1)
	}
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimSuffix(u, "/")
	return strings.ToLower(u)
}

func (a *App) clonePath(marketName string) string {
	return filepath.Join(a.cacheDir, marketName)
}

func findMarketConfig(cfg domain.Config, name string) *domain.MarketConfig {
	for i := range cfg.Markets {
		if cfg.Markets[i].Name == name {
			return &cfg.Markets[i]
		}
	}
	return nil
}

func marketConfigToMarket(mc domain.MarketConfig, cloneDir string) domain.Market {
	return domain.Market{
		Name:      mc.Name,
		URL:       mc.URL,
		Branch:    mc.Branch,
		ClonePath: filepath.Join(cloneDir, mc.Name),
		Trusted:   mc.Trusted,
		ReadOnly:  mc.ReadOnly,
	}
}

func (a *App) ListMarkets() ([]domain.Market, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}
	markets := make([]domain.Market, len(cfg.Markets))
	for i, mc := range cfg.Markets {
		markets[i] = marketConfigToMarket(mc, a.cacheDir)
	}
	return markets, nil
}

func (a *App) GetMarket(name string) (domain.Market, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return domain.Market{}, err
	}
	for _, mc := range cfg.Markets {
		if mc.Name == name {
			return marketConfigToMarket(mc, a.cacheDir), nil
		}
	}
	return domain.Market{}, domain.ErrMarketNotFound
}

func (a *App) MarketInfo(name string) (service.MarketInfoResult, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return service.MarketInfoResult{}, err
	}

	found := findMarketConfig(cfg, name)
	if found == nil {
		return service.MarketInfoResult{}, domain.ErrMarketNotFound
	}

	market := marketConfigToMarket(*found, a.cacheDir)

	syncState, err := a.state.LoadSyncState(a.cacheDir)
	if err != nil {
		return service.MarketInfoResult{}, err
	}

	checksums, err := a.state.LoadChecksums(a.cacheDir)
	if err != nil {
		return service.MarketInfoResult{}, err
	}

	files, err := a.git.ListFiles(market.ClonePath, found.Branch)
	if err != nil {
		files = nil
	}

	installedCount := 0
	for ref, entry := range checksums.Entries {
		if entry == nil {
			continue
		}
		if ref.Market() == name {
			installedCount++
		}
	}

	result := service.MarketInfoResult{
		Market:         market,
		EntryCount:     len(files),
		InstalledCount: installedCount,
	}

	if ms, ok := syncState.Markets[name]; ok {
		result.LastSynced = ms.LastSyncedAt
		result.Status = ms.Status
	}

	return result, nil
}

func (a *App) AddMarket(name, url string, opts service.AddMarketOpts) (service.AddMarketResult, error) {
	var result service.AddMarketResult

	if err := validateMarketName(name); err != nil {
		return result, err
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return result, err
	}

	for _, mc := range cfg.Markets {
		if mc.Name == name {
			return result, domain.ErrMarketAlreadyExists
		}
		if normalizeURL(mc.URL) == normalizeURL(url) {
			return result, &domain.DomainError{
				Code:    domain.ErrMarketURLExists.Code,
				Message: fmt.Sprintf("market %q already uses URL %s", mc.Name, mc.URL),
			}
		}
	}

	clonePath := filepath.Join(a.cacheDir, name)
	if a.fs.DirExists(clonePath) {
		return result, domain.ErrCloneExists
	}

	if !opts.NoClone {
		if err := a.git.ValidateRemote(url); err != nil {
			return result, domain.ErrMarketUnreachable.Wrap(err)
		}

		if err := a.git.Clone(url, clonePath); err != nil {
			return result, err
		}
	}

	branch := opts.Branch
	if branch == "" {
		branch = "main"
	}

	if !opts.NoClone {
		sha, err := a.git.RemoteHEAD(clonePath, branch)
		if err != nil {
			return result, err
		}

		if err := a.state.SetMarketSyncClean(a.cacheDir, name, sha); err != nil {
			return result, err
		}

		result = countMarketEntries(a.git, clonePath, branch)
	}

	mc := domain.MarketConfig{
		Name:     name,
		URL:      url,
		Branch:   branch,
		Trusted:  opts.Trusted,
		ReadOnly: opts.ReadOnly,
	}
	return result, a.cfg.AddMarket(a.configPath, mc)
}

func countMarketEntries(git gitrepo.GitRepo, clonePath, branch string) service.AddMarketResult {
	files, err := git.ListFiles(clonePath, branch)
	if err != nil {
		return service.AddMarketResult{}
	}

	profiles := make(map[string]struct{})
	var result service.AddMarketResult

	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")
		if len(parts) >= 2 {
			profiles[parts[0]+"/"+parts[1]] = struct{}{}
		}
		for _, p := range parts {
			switch p {
			case "agents":
				result.Agents++
			case "skills":
				result.Skills++
			}
		}
	}
	result.Profiles = len(profiles)
	return result
}

func (a *App) RemoveMarket(name string, opts service.RemoveMarketOpts) error {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return err
	}

	if findMarketConfig(cfg, name) == nil {
		return domain.ErrMarketNotFound
	}

	if !opts.Force {
		checksums, err := a.state.LoadChecksums(a.cacheDir)
		if err != nil {
			return err
		}

		var installedRefs []string
		for ref, entry := range checksums.Entries {
			if entry == nil {
				continue
			}
			if ref.Market() == name {
				installedRefs = append(installedRefs, string(ref))
			}
		}

		if len(installedRefs) > 0 {
			return &domain.DomainError{
				Code:    "MARKET_HAS_INSTALLED_ENTRIES",
				Message: fmt.Sprintf("market %q has installed entries: %s", name, strings.Join(installedRefs, ", ")),
			}
		}
	}

	if err := a.cfg.RemoveMarket(a.configPath, name); err != nil {
		return err
	}

	syncState, err := a.state.LoadSyncState(a.cacheDir)
	if err != nil {
		return err
	}
	delete(syncState.Markets, name)
	if err := a.state.SaveSyncState(a.cacheDir, syncState); err != nil {
		return err
	}

	if !opts.KeepCache {
		clonePath := filepath.Join(a.cacheDir, name)
		if err := a.fs.RemoveAll(clonePath); err != nil {
			return err
		}
	}

	checksums, err := a.state.LoadChecksums(a.cacheDir)
	if err != nil {
		return err
	}

	for ref, entry := range checksums.Entries {
		if entry == nil {
			continue
		}
		if ref.Market() == name {
			entry.Status = "orphaned"
			if err := a.state.UpdateChecksum(a.cacheDir, ref, *entry); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *App) RenameMarket(oldName, newName string) error {
	if err := validateMarketName(newName); err != nil {
		return err
	}

	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return err
	}

	if findMarketConfig(cfg, oldName) == nil {
		return domain.ErrMarketNotFound
	}

	if err := a.cfg.SetMarketProperty(a.configPath, oldName, "name", newName); err != nil {
		return err
	}

	syncState, err := a.state.LoadSyncState(a.cacheDir)
	if err != nil {
		return err
	}
	if ms, ok := syncState.Markets[oldName]; ok {
		syncState.Markets[newName] = ms
		delete(syncState.Markets, oldName)
		if err := a.state.SaveSyncState(a.cacheDir, syncState); err != nil {
			return err
		}
	}

	checksums, err := a.state.LoadChecksums(a.cacheDir)
	if err != nil {
		return err
	}
	for ref, entry := range checksums.Entries {
		if entry == nil {
			continue
		}
		if ref.Market() == oldName {
			newRef := domain.MctRef(newName + "/" + ref.RelPath())
			if err := a.state.UpdateChecksum(a.cacheDir, newRef, *entry); err != nil {
				return err
			}
			if err := a.state.RemoveChecksum(a.cacheDir, ref); err != nil {
				return err
			}
		}
	}

	cfg2, err := a.cfg.Load(a.configPath)
	if err != nil {
		return err
	}
	for _, skill := range cfg2.ManagedSkills {
		if skill.Ref.Market() == oldName {
			newRef := domain.MctRef(newName + "/" + skill.Ref.RelPath())
			if err := a.cfg.RemoveManagedSkill(a.configPath, skill.Ref); err != nil {
				return err
			}
			skill.Ref = newRef
			if err := a.cfg.AddManagedSkill(a.configPath, skill); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *App) SetMarketProperty(name, key, value string) error {
	return a.cfg.SetMarketProperty(a.configPath, name, key, value)
}

func (a *App) SetConfigField(key, value string) error {
	return a.cfg.SetConfigField(a.configPath, key, value)
}

func (a *App) GetConfig() (domain.Config, error) {
	return a.cfg.Load(a.configPath)
}
