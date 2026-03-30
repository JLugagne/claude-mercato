package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

func (a *App) LintMarket(dir string) (service.LintResult, error) {
	var result service.LintResult

	type agentDeps struct {
		agentRel string
		deps     []domain.SkillDep
	}
	type profileData struct {
		agents    []string
		skills    []string
		hasReadme bool
		hasTags   bool
	}
	profiles := make(map[string]*profileData)
	var profileOrder []string
	knownPaths := make(map[string]struct{}) // all valid entry rel paths
	var agentDepsList []agentDeps

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		parts := strings.SplitN(rel, "/", 3)
		if len(parts) < 2 {
			return nil
		}
		profile := parts[0] + "/" + parts[1]

		// Profile-level README: exactly profile/subdir/README.md (3 parts, no deeper)
		if len(parts) == 3 && strings.EqualFold(parts[2], "README.md") {
			if _, ok := profiles[profile]; !ok {
				profiles[profile] = &profileData{}
				profileOrder = append(profileOrder, profile)
			}
			pd := profiles[profile]
			pd.hasReadme = true

			content, readErr := a.fs.ReadFile(path)
			if readErr == nil {
				if rfm, parseErr := domain.ParseReadmeFrontmatter(content); parseErr == nil {
					pd.hasTags = len(rfm.MctTags) > 0
				}
			}
			return nil
		}

		// Must be at least 3 parts: profile/subdir/file.md
		if len(parts) < 3 {
			return nil
		}

		content, readErr := a.fs.ReadFile(path)
		if readErr != nil {
			return nil
		}
		fm, parseErr := domain.ParseFrontmatter(content)
		if parseErr != nil {
			if _, ok := profiles[profile]; !ok {
				profiles[profile] = &profileData{}
				profileOrder = append(profileOrder, profile)
			}
			profiles[profile].agents = append(profiles[profile].agents, rel)
			result.Issues = append(result.Issues, service.LintIssue{
				Profile:  profile,
				Severity: "error",
				Message:  rel + ": invalid or missing frontmatter",
			})
			return nil
		}

		if _, ok := profiles[profile]; !ok {
			profiles[profile] = &profileData{}
			profileOrder = append(profileOrder, profile)
		}
		pd := profiles[profile]
		switch fm.Type {
		case domain.EntryTypeAgent:
			pd.agents = append(pd.agents, rel)
			knownPaths[rel] = struct{}{}
			if len(fm.RequiresSkills) > 0 {
				agentDepsList = append(agentDepsList, agentDeps{agentRel: rel, deps: fm.RequiresSkills})
			}
		case domain.EntryTypeSkill:
			pd.skills = append(pd.skills, rel)
			knownPaths[rel] = struct{}{}
		default:
			result.Issues = append(result.Issues, service.LintIssue{
				Profile:  profile,
				Severity: "error",
				Message:  rel + ": unknown type " + string(fm.Type),
			})
		}

		return nil
	})
	if err != nil {
		return result, err
	}

	result.Profiles = len(profileOrder)
	for _, profile := range profileOrder {
		pd := profiles[profile]
		result.Agents += len(pd.agents)
		result.Skills += len(pd.skills)

		if !pd.hasReadme {
			result.Issues = append(result.Issues, service.LintIssue{
				Profile:  profile,
				Severity: "warn",
				Message:  "missing README.md",
			})
		} else if !pd.hasTags {
			result.Issues = append(result.Issues, service.LintIssue{
				Profile:  profile,
				Severity: "warn",
				Message:  "README.md has no tags",
			})
		}

		if len(pd.agents)+len(pd.skills) == 0 {
			result.Issues = append(result.Issues, service.LintIssue{
				Profile:  profile,
				Severity: "warn",
				Message:  "profile has no agents or skills",
			})
		}
	}

	for _, ad := range agentDepsList {
		parts := strings.SplitN(ad.agentRel, "/", 3)
		profile := parts[0] + "/" + parts[1]
		for _, dep := range ad.deps {
			depPath := filepath.ToSlash(dep.File)
			if _, ok := knownPaths[depPath]; !ok {
				result.Issues = append(result.Issues, service.LintIssue{
					Profile:  profile,
					Severity: "error",
					Message:  ad.agentRel + ": requires_skills references missing file: " + depPath,
				})
			}
		}
	}

	return result, nil
}
