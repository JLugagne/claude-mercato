package configstoretest

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type MockConfigStore struct {
	LoadFn               func(path string) (domain.Config, error)
	SaveFn               func(path string, cfg domain.Config) error
	AddMarketFn          func(path string, market domain.MarketConfig) error
	RemoveMarketFn       func(path string, name string) error
	AddEntryFn           func(path string, entry domain.EntryConfig) error
	RemoveEntryFn        func(path string, ref domain.MctRef) error
	AddManagedSkillFn    func(path string, skill domain.ManagedSkillConfig) error
	RemoveManagedSkillFn func(path string, ref domain.MctRef) error
	SetMarketPropertyFn  func(path string, marketName string, key string, value string) error
}

func (m *MockConfigStore) Load(path string) (domain.Config, error) {
	if m.LoadFn != nil {
		return m.LoadFn(path)
	}
	return domain.Config{}, nil
}

func (m *MockConfigStore) Save(path string, cfg domain.Config) error {
	if m.SaveFn != nil {
		return m.SaveFn(path, cfg)
	}
	return nil
}

func (m *MockConfigStore) AddMarket(path string, market domain.MarketConfig) error {
	if m.AddMarketFn != nil {
		return m.AddMarketFn(path, market)
	}
	return nil
}

func (m *MockConfigStore) RemoveMarket(path string, name string) error {
	if m.RemoveMarketFn != nil {
		return m.RemoveMarketFn(path, name)
	}
	return nil
}

func (m *MockConfigStore) AddEntry(path string, entry domain.EntryConfig) error {
	if m.AddEntryFn != nil {
		return m.AddEntryFn(path, entry)
	}
	return nil
}

func (m *MockConfigStore) RemoveEntry(path string, ref domain.MctRef) error {
	if m.RemoveEntryFn != nil {
		return m.RemoveEntryFn(path, ref)
	}
	return nil
}

func (m *MockConfigStore) AddManagedSkill(path string, skill domain.ManagedSkillConfig) error {
	if m.AddManagedSkillFn != nil {
		return m.AddManagedSkillFn(path, skill)
	}
	return nil
}

func (m *MockConfigStore) RemoveManagedSkill(path string, ref domain.MctRef) error {
	if m.RemoveManagedSkillFn != nil {
		return m.RemoveManagedSkillFn(path, ref)
	}
	return nil
}

func (m *MockConfigStore) SetMarketProperty(path string, marketName string, key string, value string) error {
	if m.SetMarketPropertyFn != nil {
		return m.SetMarketPropertyFn(path, marketName, key, value)
	}
	return nil
}
