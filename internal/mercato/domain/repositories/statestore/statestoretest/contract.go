package statestoretest

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type MockStateStore struct {
	LoadSyncStateFn      func(cacheDir string) (domain.SyncState, error)
	SaveSyncStateFn      func(cacheDir string, state domain.SyncState) error
	LoadChecksumsFn      func(cacheDir string) (domain.ChecksumState, error)
	SaveChecksumsFn      func(cacheDir string, state domain.ChecksumState) error
	SetMarketSyncDirtyFn func(cacheDir string, market string) error
	SetMarketSyncCleanFn func(cacheDir string, market string, newSHA string) error
	UpdateChecksumFn     func(cacheDir string, ref domain.MctRef, entry domain.ChecksumEntry) error
	RemoveChecksumFn     func(cacheDir string, ref domain.MctRef) error
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

func (m *MockStateStore) LoadChecksums(cacheDir string) (domain.ChecksumState, error) {
	if m.LoadChecksumsFn != nil {
		return m.LoadChecksumsFn(cacheDir)
	}
	return domain.ChecksumState{Version: 1, Entries: make(map[domain.MctRef]*domain.ChecksumEntry)}, nil
}

func (m *MockStateStore) SaveChecksums(cacheDir string, state domain.ChecksumState) error {
	if m.SaveChecksumsFn != nil {
		return m.SaveChecksumsFn(cacheDir, state)
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

func (m *MockStateStore) UpdateChecksum(cacheDir string, ref domain.MctRef, entry domain.ChecksumEntry) error {
	if m.UpdateChecksumFn != nil {
		return m.UpdateChecksumFn(cacheDir, ref, entry)
	}
	return nil
}

func (m *MockStateStore) RemoveChecksum(cacheDir string, ref domain.MctRef) error {
	if m.RemoveChecksumFn != nil {
		return m.RemoveChecksumFn(cacheDir, ref)
	}
	return nil
}
