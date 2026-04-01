package domain

import (
	"strings"
	"time"
)

type Config struct {
	Markets        []MarketConfig `yaml:"markets"`
	LocalPath      string         `yaml:"local_path"`
	StaleAfter     time.Duration  `yaml:"stale_after"`
	ConflictPolicy string         `yaml:"conflict_policy"`
	DriftPolicy    string         `yaml:"drift_policy"`
	SSHEnabled     *bool          `yaml:"ssh_enabled,omitempty"`
}

type MarketConfig struct {
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
	Branch     string `yaml:"branch"`
	Trusted    bool   `yaml:"trusted,omitempty"`
	ReadOnly   bool   `yaml:"read_only,omitempty"`
	SkillsOnly bool   `yaml:"skills_only,omitempty"`
	SkillsPath string `yaml:"skills_path,omitempty"`
}

// NormalizeURL strips protocol prefixes, trailing .git, and trailing slashes
// so that "git@github.com:org/repo.git", "https://github.com/org/repo", and
// "https://github.com/org/repo.git" all compare as equal.
func NormalizeURL(u string) string {
	u = strings.TrimSpace(u)
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
