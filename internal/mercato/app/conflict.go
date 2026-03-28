package app

import (
	"fmt"
	"path/filepath"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

func (a *App) Conflicts() ([]domain.Conflict, error) {
	cfg, err := a.cfg.Load(a.configPath)
	if err != nil {
		return nil, err
	}

	var conflicts []domain.Conflict

	filesByName := make(map[string][]domain.MctRef)
	for _, mc := range cfg.Markets {
		clonePath := a.clonePath(mc.Name)
		files, err := a.git.ListFiles(clonePath, mc.Branch)
		if err != nil {
			continue
		}
		for _, f := range files {
			base := filepath.Base(f)
			ref := domain.MctRef(mc.Name + "/" + f)
			filesByName[base] = append(filesByName[base], ref)
		}
	}
	for name, refs := range filesByName {
		if len(refs) > 1 {
			conflicts = append(conflicts, domain.Conflict{
				Type:        "ref-collision",
				Refs:        refs,
				Description: fmt.Sprintf("filename %q exists in %d markets", name, len(refs)),
				Severity:    "error",
			})
		}
	}

	skillPins := make(map[domain.MctRef]map[string]domain.MctRef)
	for _, ms := range cfg.ManagedSkills {
		if skillPins[ms.Ref] == nil {
			skillPins[ms.Ref] = make(map[string]domain.MctRef)
		}
		skillPins[ms.Ref][string(ms.MctVersion)] = ms.ManagedBy
	}
	for skillRef, versions := range skillPins {
		if len(versions) > 1 {
			var parents []domain.MctRef
			for _, parent := range versions {
				parents = append(parents, parent)
			}
			conflicts = append(conflicts, domain.Conflict{
				Type:        "dep-version-mismatch",
				Refs:        append([]domain.MctRef{skillRef}, parents...),
				Description: fmt.Sprintf("skill %s required at %d different versions", skillRef, len(versions)),
				Severity:    "error",
			})
		}
	}

	for _, ms := range cfg.ManagedSkills {
		marketName := ms.Ref.Market()
		relPath := ms.Ref.RelPath()
		clonePath := a.clonePath(marketName)
		mc := findMarketConfig(cfg, marketName)
		if mc == nil {
			continue
		}
		_, err := a.git.ReadFileAtRef(clonePath, mc.Branch, relPath, "HEAD")
		if err != nil {
			conflicts = append(conflicts, domain.Conflict{
				Type:        "dep-deleted",
				Refs:        []domain.MctRef{ms.Ref, ms.ManagedBy},
				Description: fmt.Sprintf("skill %s required by %s has been deleted", ms.Ref, ms.ManagedBy),
				Severity:    "error",
			})
		}
	}

	return conflicts, nil
}
