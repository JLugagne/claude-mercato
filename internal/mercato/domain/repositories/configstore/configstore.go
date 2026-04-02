package configstore

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type ConfigStore interface {
	Load(path string) (domain.Config, error)
	Save(path string, cfg domain.Config) error
	Exists(path string) bool
	AddMarket(path string, market domain.MarketConfig) error
	RemoveMarket(path string, name string) error
	SetMarketProperty(path string, marketName string, key string, value string) error
	SetConfigField(path string, key string, value string) error
	LoadProjectConfig(projectDir string) (domain.ProjectConfig, error)
}
