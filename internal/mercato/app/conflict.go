package app

import (
	"fmt"
	"path/filepath"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
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
			if mc.SkillsOnly && !isSkillPath(f, mc.SkillsPath) {
				continue
			}
			base := filepath.Base(f)
			ref := domain.MctRef(mc.Name + "@" + f)
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

	return conflicts, nil
}
