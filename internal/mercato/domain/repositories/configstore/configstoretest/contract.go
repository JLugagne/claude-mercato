package configstoretest

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/configstore"
)

var _ configstore.ConfigStore = (*MockConfigStore)(nil)

type MockConfigStore struct {
	LoadFn              func(path string) (domain.Config, error)
	SaveFn              func(path string, cfg domain.Config) error
	AddMarketFn         func(path string, market domain.MarketConfig) error
	RemoveMarketFn      func(path string, name string) error
	SetMarketPropertyFn func(path string, marketName string, key string, value string) error
	SetConfigFieldFn    func(path string, key string, value string) error
}

func (m *MockConfigStore) Load(path string) (domain.Config, error) {
	if m.LoadFn == nil {
		panic("called not defined LoadFn")
	}
	return m.LoadFn(path)
}

func (m *MockConfigStore) Save(path string, cfg domain.Config) error {
	if m.SaveFn == nil {
		panic("called not defined SaveFn")
	}
	return m.SaveFn(path, cfg)
}

func (m *MockConfigStore) AddMarket(path string, market domain.MarketConfig) error {
	if m.AddMarketFn == nil {
		panic("called not defined AddMarketFn")
	}
	return m.AddMarketFn(path, market)
}

func (m *MockConfigStore) RemoveMarket(path string, name string) error {
	if m.RemoveMarketFn == nil {
		panic("called not defined RemoveMarketFn")
	}
	return m.RemoveMarketFn(path, name)
}

func (m *MockConfigStore) SetMarketProperty(path string, marketName string, key string, value string) error {
	if m.SetMarketPropertyFn == nil {
		panic("called not defined SetMarketPropertyFn")
	}
	return m.SetMarketPropertyFn(path, marketName, key, value)
}

func (m *MockConfigStore) SetConfigField(path string, key string, value string) error {
	if m.SetConfigFieldFn == nil {
		panic("called not defined SetConfigFieldFn")
	}
	return m.SetConfigFieldFn(path, key, value)
}
