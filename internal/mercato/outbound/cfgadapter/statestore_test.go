package cfgadapter

import (
	"os"
	"testing"
	"time"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

func newStateStore(t *testing.T) (*StateStoreAdapter, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "statestore-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return NewStateStore(), dir
}

func TestLoadSyncState_Empty(t *testing.T) {
	ss, dir := newStateStore(t)

	state, err := ss.LoadSyncState(dir)
	if err != nil {
		t.Fatalf("LoadSyncState on empty dir: %v", err)
	}

	if state.Version != 1 {
		t.Errorf("Version = %d, want 1", state.Version)
	}
	if state.Markets == nil {
		t.Error("Markets = nil, want empty map")
	}
	if len(state.Markets) != 0 {
		t.Errorf("len(Markets) = %d, want 0", len(state.Markets))
	}
}

func TestSaveAndLoadSyncState(t *testing.T) {
	ss, dir := newStateStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	state := domain.SyncState{
		Version: 1,
		Markets: map[string]domain.MarketSyncState{
			"core": {
				LastSyncedSHA: "deadbeef",
				Branch:        "main",
				Status:        "clean",
				LastSyncedAt:  now,
			},
		},
	}

	if err := ss.SaveSyncState(dir, state); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}

	got, err := ss.LoadSyncState(dir)
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}

	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}

	mkt, ok := got.Markets["core"]
	if !ok {
		t.Fatal("market 'core' not found in state")
	}
	if mkt.LastSyncedSHA != "deadbeef" {
		t.Errorf("LastSyncedSHA = %q, want %q", mkt.LastSyncedSHA, "deadbeef")
	}
	if mkt.Branch != "main" {
		t.Errorf("Branch = %q, want %q", mkt.Branch, "main")
	}
	if mkt.Status != "clean" {
		t.Errorf("Status = %q, want %q", mkt.Status, "clean")
	}
	if !mkt.LastSyncedAt.Equal(now) {
		t.Errorf("LastSyncedAt = %v, want %v", mkt.LastSyncedAt, now)
	}
}

func TestSetMarketSyncDirty(t *testing.T) {
	ss, dir := newStateStore(t)

	if err := ss.SetMarketSyncDirty(dir, "mkt"); err != nil {
		t.Fatalf("SetMarketSyncDirty: %v", err)
	}

	state, err := ss.LoadSyncState(dir)
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}

	mkt, ok := state.Markets["mkt"]
	if !ok {
		t.Fatal("market 'mkt' not found in state")
	}
	if mkt.Status != "dirty" {
		t.Errorf("Status = %q, want %q", mkt.Status, "dirty")
	}
}

func TestSetMarketSyncClean(t *testing.T) {
	ss, dir := newStateStore(t)

	before := time.Now().UTC()
	if err := ss.SetMarketSyncClean(dir, "mkt", "abc123"); err != nil {
		t.Fatalf("SetMarketSyncClean: %v", err)
	}
	after := time.Now().UTC()

	state, err := ss.LoadSyncState(dir)
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}

	mkt, ok := state.Markets["mkt"]
	if !ok {
		t.Fatal("market 'mkt' not found in state")
	}
	if mkt.LastSyncedSHA != "abc123" {
		t.Errorf("LastSyncedSHA = %q, want %q", mkt.LastSyncedSHA, "abc123")
	}
	if mkt.Status != "clean" {
		t.Errorf("Status = %q, want %q", mkt.Status, "clean")
	}
	if mkt.LastSyncedAt.Before(before) || mkt.LastSyncedAt.After(after) {
		t.Errorf("LastSyncedAt %v not between %v and %v", mkt.LastSyncedAt, before, after)
	}
}

func TestSetMarketSyncDirtyThenClean(t *testing.T) {
	ss, dir := newStateStore(t)

	if err := ss.SetMarketSyncDirty(dir, "mkt"); err != nil {
		t.Fatalf("SetMarketSyncDirty: %v", err)
	}

	if err := ss.SetMarketSyncClean(dir, "mkt", "newsha456"); err != nil {
		t.Fatalf("SetMarketSyncClean: %v", err)
	}

	state, err := ss.LoadSyncState(dir)
	if err != nil {
		t.Fatalf("LoadSyncState: %v", err)
	}

	mkt, ok := state.Markets["mkt"]
	if !ok {
		t.Fatal("market 'mkt' not found in state")
	}
	if mkt.Status != "clean" {
		t.Errorf("Status = %q, want %q", mkt.Status, "clean")
	}
	if mkt.LastSyncedSHA != "newsha456" {
		t.Errorf("LastSyncedSHA = %q, want %q", mkt.LastSyncedSHA, "newsha456")
	}
}
