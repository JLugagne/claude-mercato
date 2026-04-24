package statestore

import "github.com/JLugagne/agents-mercato/internal/mercato/domain"

type StateStore interface {
	LoadSyncState(cacheDir string) (domain.SyncState, error)
	SaveSyncState(cacheDir string, state domain.SyncState) error
	SetMarketSyncDirty(cacheDir string, market string) error
	SetMarketSyncClean(cacheDir string, market string, newSHA string) error
}
