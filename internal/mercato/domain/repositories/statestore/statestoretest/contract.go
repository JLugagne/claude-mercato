package statestoretest

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/statestore"
)

var _ statestore.StateStore = (*MockStateStore)(nil)

type MockStateStore struct {
	LoadSyncStateFn      func(cacheDir string) (domain.SyncState, error)
	SaveSyncStateFn      func(cacheDir string, state domain.SyncState) error
	SetMarketSyncDirtyFn func(cacheDir string, market string) error
	SetMarketSyncCleanFn func(cacheDir string, market string, newSHA string) error
}

func (m *MockStateStore) LoadSyncState(cacheDir string) (domain.SyncState, error) {
	if m.LoadSyncStateFn == nil {
		panic("called not defined LoadSyncStateFn")
	}
	return m.LoadSyncStateFn(cacheDir)
}

func (m *MockStateStore) SaveSyncState(cacheDir string, state domain.SyncState) error {
	if m.SaveSyncStateFn == nil {
		panic("called not defined SaveSyncStateFn")
	}
	return m.SaveSyncStateFn(cacheDir, state)
}

func (m *MockStateStore) SetMarketSyncDirty(cacheDir string, market string) error {
	if m.SetMarketSyncDirtyFn == nil {
		panic("called not defined SetMarketSyncDirtyFn")
	}
	return m.SetMarketSyncDirtyFn(cacheDir, market)
}

func (m *MockStateStore) SetMarketSyncClean(cacheDir string, market string, newSHA string) error {
	if m.SetMarketSyncCleanFn == nil {
		panic("called not defined SetMarketSyncCleanFn")
	}
	return m.SetMarketSyncCleanFn(cacheDir, market, newSHA)
}
