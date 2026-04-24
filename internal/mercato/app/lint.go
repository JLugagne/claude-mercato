package app

import (
	"io/fs"
	"strings"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

func (a *App) LintMarket(fsys fs.FS, dir string) (service.LintResult, error) {
	var result service.LintResult

	walkResult, err := walkMarketEntries(fsys, dir, &result)
	if err != nil {
		return result, err
	}

	validateProfiles(walkResult, &result)
	validateDeps(walkResult, &result)

	return result, nil
}

// lintWalkResult holds the intermediate state from walking a market directory.
type lintWalkResult struct {
	profiles     map[string]*lintProfileData
	profileOrder []string
	knownPaths   map[string]struct{}
	agentDeps    []lintAgentDeps
}

type lintProfileData struct {
	agents    []string
	skills    []string
	hasReadme bool
	hasTags   bool
}

type lintAgentDeps struct {
	agentRel string
	deps     []domain.SkillDep
}

// walkMarketEntries walks the market filesystem and collects profile, agent,
// skill, and dependency data. Frontmatter parse errors are appended to result.
func walkMarketEntries(fsys fs.FS, dir string, result *service.LintResult) (lintWalkResult, error) {
	w := lintWalkResult{
		profiles:   make(map[string]*lintProfileData),
		knownPaths: make(map[string]struct{}),
	}

	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		rel := path
		if dir != "." && dir != "" {
			rel = strings.TrimPrefix(path, dir+"/")
		}

		parts := strings.SplitN(rel, "/", 3)
		if len(parts) < 2 {
			return nil
		}
		profile := parts[0] + "/" + parts[1]

		// Profile-level README
		if len(parts) == 3 && strings.EqualFold(parts[2], "README.md") {
			pd := w.ensureProfile(profile)
			pd.hasReadme = true
			content, readErr := fs.ReadFile(fsys, path)
			if readErr == nil {
				if rfm, parseErr := domain.ParseReadmeFrontmatter(content); parseErr == nil {
					pd.hasTags = len(rfm.MctTags) > 0
				}
			}
			return nil
		}

		if len(parts) < 3 {
			return nil
		}

		content, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return nil
		}
		fm, parseErr := domain.ParseFrontmatter(content)
		if parseErr != nil {
			w.ensureProfile(profile).agents = append(w.ensureProfile(profile).agents, rel)
			result.Issues = append(result.Issues, service.LintIssue{
				Profile:  profile,
				Severity: "error",
				Message:  rel + ": invalid or missing frontmatter",
			})
			return nil
		}

		pd := w.ensureProfile(profile)
		switch inferEntryType(rel) {
		case domain.EntryTypeAgent:
			pd.agents = append(pd.agents, rel)
			w.knownPaths[rel] = struct{}{}
			if len(fm.RequiresSkills) > 0 {
				w.agentDeps = append(w.agentDeps, lintAgentDeps{agentRel: rel, deps: fm.RequiresSkills})
			}
		case domain.EntryTypeSkill:
			pd.skills = append(pd.skills, rel)
			w.knownPaths[rel] = struct{}{}
		default:
			result.Issues = append(result.Issues, service.LintIssue{
				Profile:  profile,
				Severity: "error",
				Message:  rel + ": cannot determine type from path",
			})
		}

		return nil
	})

	return w, err
}

func (w *lintWalkResult) ensureProfile(profile string) *lintProfileData {
	if _, ok := w.profiles[profile]; !ok {
		w.profiles[profile] = &lintProfileData{}
		w.profileOrder = append(w.profileOrder, profile)
	}
	return w.profiles[profile]
}

// validateProfiles checks each profile for missing READMEs, missing tags,
// and empty profiles.
func validateProfiles(w lintWalkResult, result *service.LintResult) {
	result.Profiles = len(w.profileOrder)
	for _, profile := range w.profileOrder {
		pd := w.profiles[profile]
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
}

// validateDeps checks that all requires_skills references point to known paths.
func validateDeps(w lintWalkResult, result *service.LintResult) {
	for _, ad := range w.agentDeps {
		parts := strings.SplitN(ad.agentRel, "/", 3)
		profile := parts[0] + "/" + parts[1]
		for _, dep := range ad.deps {
			if _, ok := w.knownPaths[dep.File]; !ok {
				result.Issues = append(result.Issues, service.LintIssue{
					Profile:  profile,
					Severity: "error",
					Message:  ad.agentRel + ": requires_skills references missing file: " + dep.File,
				})
			}
		}
	}
}
