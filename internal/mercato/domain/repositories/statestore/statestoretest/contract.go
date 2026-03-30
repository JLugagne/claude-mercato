package statestoretest

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type MockStateStore struct {
	LoadSyncStateFn      func(cacheDir string) (domain.SyncState, error)
	SaveSyncStateFn      func(cacheDir string, state domain.SyncState) error
	SetMarketSyncDirtyFn func(cacheDir string, market string) error
	SetMarketSyncCleanFn func(cacheDir string, market string, newSHA string) error
}

func (m *MockStateStore) LoadSyncState(cacheDir string) (domain.SyncState, error) {
	if m.LoadSyncStateFn != nil {
		return m.LoadSyncStateFn(cacheDir)
	}
	return domain.SyncState{Version: 1, Markets: make(map[string]domain.MarketSyncState)}, nil
}

func (m *MockStateStore) SaveSyncState(cacheDir string, state domain.SyncState) error {
	if m.SaveSyncStateFn != nil {
		return m.SaveSyncStateFn(cacheDir, state)
	}
	return nil
}

func (m *MockStateStore) SetMarketSyncDirty(cacheDir string, market string) error {
	if m.SetMarketSyncDirtyFn != nil {
		return m.SetMarketSyncDirtyFn(cacheDir, market)
	}
	return nil
}

func (m *MockStateStore) SetMarketSyncClean(cacheDir string, market string, newSHA string) error {
	if m.SetMarketSyncCleanFn != nil {
		return m.SetMarketSyncCleanFn(cacheDir, market, newSHA)
	}
	return nil
}
