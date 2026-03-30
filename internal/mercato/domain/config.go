package domain

import "time"

type Config struct {
	Markets        []MarketConfig `yaml:"markets"`
	LocalPath      string         `yaml:"local_path"`
	StaleAfter     time.Duration  `yaml:"stale_after"`
	ConflictPolicy string         `yaml:"conflict_policy"`
	DriftPolicy    string         `yaml:"drift_policy"`
	Difftool       string         `yaml:"difftool"`
	SSHEnabled     *bool          `yaml:"ssh_enabled,omitempty"`
}

type MarketConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Branch   string `yaml:"branch"`
	Trusted  bool   `yaml:"trusted,omitempty"`
	ReadOnly bool   `yaml:"read_only,omitempty"`
}
