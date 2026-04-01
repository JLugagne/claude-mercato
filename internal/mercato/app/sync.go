package app

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

func (a *App) Check(opts service.CheckOpts) ([]domain.EntryStatus, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	installed, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return nil, err
	}

	var statuses []domain.EntryStatus

	for ref, entry := range installed.Entries {
		if entry == nil {
			continue
		}

		marketName := ref.Market()

		if opts.Market != "" && marketName != opts.Market {
			continue
		}

		// With symlinks, the file is always up to date if the symlink is valid.
		// Check if the symlink target still exists.
		if !a.fs.IsSymlink(entry.LocalPath) {
			statuses = append(statuses, domain.EntryStatus{
				Ref:   ref,
				State: domain.StateDrift,
			})
			continue
		}

		statuses = append(statuses, domain.EntryStatus{
			Ref:   ref,
			State: domain.StateClean,
		})
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
			Market:       mc.Name,
			OldSHA:       oldSHA,
			NewSHA:       newSHA,
			ChangedFiles: changedFiles,
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

	installed, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return nil, err
	}

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

		for _, diff := range diffs {
			filePath := diff.To
			if filePath == "" {
				filePath = diff.From
			}

			if mc.SkillsOnly && !isSkillPath(filePath, mc.SkillsPath) {
				continue
			}

			ref := domain.MctRef(mc.Name + "@" + filePath)

			if opts.Ref != "" && ref != opts.Ref {
				continue
			}

			entry, isInstalled := installed.Entries[ref]
			if !isInstalled || entry == nil {
				continue
			}

			if diff.Action == domain.DiffDelete {
				continue
			}

			// With symlinks, the file is already updated via the cache.
			// Just report the change.
			results = append(results, service.UpdateResult{
				Ref:    ref,
				Action: "update",
			})
		}
	}

	return results, nil
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
