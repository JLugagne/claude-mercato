package domain

import "time"

type Config struct {
	Markets        []MarketConfig       `yaml:"markets"`
	LocalPath      string               `yaml:"local_path"`
	NamespaceDirs  bool                 `yaml:"namespace_dirs"`
	StaleAfter     time.Duration        `yaml:"stale_after"`
	ConflictPolicy string               `yaml:"conflict_policy"`
	DriftPolicy    string               `yaml:"drift_policy"`
	Difftool       string               `yaml:"difftool"`
	SSHEnabled     *bool                `yaml:"ssh_enabled,omitempty"`
	Entries        []EntryConfig        `yaml:"entries"`
	ManagedSkills  []ManagedSkillConfig `yaml:"managed_skills"`
}

type MarketConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Branch   string `yaml:"branch"`
	Trusted  bool   `yaml:"trusted,omitempty"`
	ReadOnly bool   `yaml:"read_only,omitempty"`
}

type EntryConfig struct {
	Ref          MctRef `yaml:"ref"`
	Pin          string `yaml:"pin,omitempty"`
	DriftAllowed bool   `yaml:"drift_allowed,omitempty"`
}

type ManagedSkillConfig struct {
	Ref        MctRef     `yaml:"ref"`
	ManagedBy  MctRef     `yaml:"managed_by"`
	MctVersion MctVersion `yaml:"mct_version"`
}
