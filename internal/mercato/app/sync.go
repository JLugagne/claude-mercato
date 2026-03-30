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

	checksums, err := a.scanInstalledEntries(cfg)
	if err != nil {
		return nil, err
	}

	branchByMarket := make(map[string]string, len(cfg.Markets))
	for _, mc := range cfg.Markets {
		branchByMarket[mc.Name] = mc.Branch
	}

	var statuses []domain.EntryStatus

	for ref, entry := range checksums.Entries {
		if entry == nil {
			continue
		}

		marketName := ref.Market()
		relPath := ref.RelPath()

		if opts.Market != "" && marketName != opts.Market {
			continue
		}

		content, err := a.fs.ReadFile(entry.LocalPath)
		if err != nil {
			statuses = append(statuses, domain.EntryStatus{
				Ref:   ref,
				State: domain.StateOrphaned,
			})
			continue
		}

		currentChecksum := a.fs.MD5Checksum(content)
		drifted := currentChecksum != entry.ChecksumAtInstall

		clonePath := a.clonePath(marketName)

		newVersion, err := a.git.FileVersion(clonePath, relPath)
		if err != nil {
			newVersion = entry.MctVersion
		}

		versionChanged := newVersion != entry.MctVersion

		var state domain.EntryState
		switch {
		case drifted && versionChanged:
			state = domain.StateUpdateAndDrift
		case drifted:
			state = domain.StateDrift
		case versionChanged:
			state = domain.StateUpdateAvailable
		default:
			state = domain.StateClean
		}

		statuses = append(statuses, domain.EntryStatus{
			Ref:        ref,
			State:      state,
			NewVersion: newVersion,
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
				changedFiles = len(diffs)
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

	checksums, err := a.scanInstalledEntries(cfg)
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

			ref := domain.MctRef(mc.Name + "/" + filePath)

			if opts.Ref != "" && ref != opts.Ref {
				continue
			}

			entry, installed := checksums.Entries[ref]
			if !installed || entry == nil {
				continue
			}

			if diff.Action == domain.DiffDelete {
				continue
			}

			content, err := a.fs.ReadFile(entry.LocalPath)
			if err != nil {
				results = append(results, service.UpdateResult{
					Ref:        ref,
					Action:     "error",
					OldVersion: entry.MctVersion,
					Err:        err,
				})
				continue
			}

			currentChecksum := a.fs.MD5Checksum(content)
			drifted := currentChecksum != entry.ChecksumAtInstall

			newVersion, err := a.git.FileVersion(clonePath, filePath)
			if err != nil {
				results = append(results, service.UpdateResult{
					Ref:        ref,
					Action:     "error",
					OldVersion: entry.MctVersion,
					Err:        err,
				})
				continue
			}

			oldVersion := entry.MctVersion

			if drifted && !opts.CI {
				results = append(results, service.UpdateResult{
					Ref:        ref,
					Action:     "conflict",
					OldVersion: oldVersion,
					NewVersion: newVersion,
				})
				continue
			}

			if opts.DryRun {
				results = append(results, service.UpdateResult{
					Ref:        ref,
					Action:     "update",
					OldVersion: oldVersion,
					NewVersion: newVersion,
				})
				continue
			}

			newContent, err := a.git.ReadFileAtRef(clonePath, mc.Branch, filePath, "")
			if err != nil {
				results = append(results, service.UpdateResult{
					Ref:        ref,
					Action:     "error",
					OldVersion: oldVersion,
					Err:        err,
				})
				continue
			}

			newChecksum := a.fs.MD5Checksum(newContent)

			injected, err := domain.InjectMctFields(newContent, ref, newVersion, ref.Market(), newChecksum)
			if err != nil {
				injected, err = domain.PatchMctVersion(newContent, newVersion)
				if err != nil {
					injected = newContent
				}
			}

			if err := a.fs.WriteFile(entry.LocalPath, injected); err != nil {
				results = append(results, service.UpdateResult{
					Ref:        ref,
					Action:     "error",
					OldVersion: oldVersion,
					Err:        err,
				})
				continue
			}

			results = append(results, service.UpdateResult{
				Ref:        ref,
				Action:     "update",
				OldVersion: oldVersion,
				NewVersion: newVersion,
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
