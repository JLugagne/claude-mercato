package configstore

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type ConfigStore interface {
	Load(path string) (domain.Config, error)
	Save(path string, cfg domain.Config) error
	AddMarket(path string, market domain.MarketConfig) error
	RemoveMarket(path string, name string) error
	AddEntry(path string, entry domain.EntryConfig) error
	RemoveEntry(path string, ref domain.MctRef) error
	AddManagedSkill(path string, skill domain.ManagedSkillConfig) error
	RemoveManagedSkill(path string, ref domain.MctRef) error
	SetMarketProperty(path string, marketName string, key string, value string) error
	SetEntryPin(path string, ref domain.MctRef, pin string) error
	SetConfigField(path string, key string, value string) error
}
