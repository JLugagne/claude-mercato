package statestore

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type StateStore interface {
	LoadSyncState(cacheDir string) (domain.SyncState, error)
	SaveSyncState(cacheDir string, state domain.SyncState) error
	LoadChecksums(cacheDir string) (domain.ChecksumState, error)
	SaveChecksums(cacheDir string, state domain.ChecksumState) error
	SetMarketSyncDirty(cacheDir string, market string) error
	SetMarketSyncClean(cacheDir string, market string, newSHA string) error
	UpdateChecksum(cacheDir string, ref domain.MctRef, entry domain.ChecksumEntry) error
	RemoveChecksum(cacheDir string, ref domain.MctRef) error
}
