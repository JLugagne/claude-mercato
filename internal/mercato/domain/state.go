package domain

import "time"

type SyncState struct {
	Version int                        `json:"version"`
	Markets map[string]MarketSyncState `json:"markets"`
}

type MarketSyncState struct {
	LastSyncedSHA string    `json:"last_synced_sha"`
	LastSyncedAt  time.Time `json:"last_synced_at"`
	ClonePath     string    `json:"clone_path"`
	Branch        string    `json:"branch"`
	Status        string    `json:"status"`
}

type ChecksumState struct {
	Version int                       `json:"version"`
	Entries map[MctRef]*ChecksumEntry `json:"entries"`
}
