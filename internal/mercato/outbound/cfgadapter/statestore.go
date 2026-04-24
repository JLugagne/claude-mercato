package cfgadapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/statestore"
)

var _ statestore.StateStore = (*StateStoreAdapter)(nil)

type StateStoreAdapter struct{}

func NewStateStore() *StateStoreAdapter { return &StateStoreAdapter{} }

func (s *StateStoreAdapter) LoadSyncState(cacheDir string) (domain.SyncState, error) {
	path := filepath.Join(cacheDir, "sync-state.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return domain.SyncState{
			Version: 1,
			Markets: make(map[string]domain.MarketSyncState),
		}, nil
	}
	if err != nil {
		return domain.SyncState{}, fmt.Errorf("read sync state: %w", err)
	}
	var state domain.SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.SyncState{}, fmt.Errorf("parse sync state: %w", err)
	}
	if state.Markets == nil {
		state.Markets = make(map[string]domain.MarketSyncState)
	}
	return state, nil
}

func (s *StateStoreAdapter) SaveSyncState(cacheDir string, state domain.SyncState) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, "sync-state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sync state: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func (s *StateStoreAdapter) SetMarketSyncDirty(cacheDir string, market string) error {
	state, err := s.LoadSyncState(cacheDir)
	if err != nil {
		return err
	}
	ms := state.Markets[market]
	ms.Status = "dirty"
	state.Markets[market] = ms
	return s.SaveSyncState(cacheDir, state)
}

func (s *StateStoreAdapter) SetMarketSyncClean(cacheDir string, market string, newSHA string) error {
	state, err := s.LoadSyncState(cacheDir)
	if err != nil {
		return err
	}
	ms := state.Markets[market]
	ms.Status = "clean"
	ms.LastSyncedSHA = newSHA
	ms.LastSyncedAt = time.Now().UTC()
	state.Markets[market] = ms
	return s.SaveSyncState(cacheDir, state)
}
