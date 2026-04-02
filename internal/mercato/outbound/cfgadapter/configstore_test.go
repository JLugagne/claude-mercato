package cfgadapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

func newConfigStore(t *testing.T) (*ConfigStoreAdapter, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "cfgadapter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return NewConfigStore(), filepath.Join(dir, "config.yaml")
}

func TestLoadConfig_NonExistent(t *testing.T) {
	cs, path := newConfigStore(t)

	_, err := cs.Load(path)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestLoadConfig_EmptyDefaults(t *testing.T) {
	cs, path := newConfigStore(t)

	// Save an empty config, then load to verify defaults are applied.
	if err := cs.Save(path, domain.Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.LocalPath != ".claude/" {
		t.Errorf("default LocalPath = %q, want %q", cfg.LocalPath, ".claude/")
	}
	if cfg.ConflictPolicy != "block" {
		t.Errorf("default ConflictPolicy = %q, want %q", cfg.ConflictPolicy, "block")
	}
	if cfg.DriftPolicy != "prompt" {
		t.Errorf("default DriftPolicy = %q, want %q", cfg.DriftPolicy, "prompt")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	cs, path := newConfigStore(t)

	cfg := domain.Config{
		LocalPath:      "/custom/path",
		ConflictPolicy: "overwrite",
		DriftPolicy:    "ignore",
		Markets: []domain.MarketConfig{
			{Name: "core", URL: "https://example.com/core.git", Branch: "main", Trusted: true, ReadOnly: false},
		},
	}

	if err := cs.Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.LocalPath != cfg.LocalPath {
		t.Errorf("LocalPath = %q, want %q", got.LocalPath, cfg.LocalPath)
	}
	if got.ConflictPolicy != cfg.ConflictPolicy {
		t.Errorf("ConflictPolicy = %q, want %q", got.ConflictPolicy, cfg.ConflictPolicy)
	}
	if len(got.Markets) != 1 {
		t.Fatalf("len(Markets) = %d, want 1", len(got.Markets))
	}
	if got.Markets[0].Name != "core" {
		t.Errorf("Markets[0].Name = %q, want %q", got.Markets[0].Name, "core")
	}
	if got.Markets[0].URL != "https://example.com/core.git" {
		t.Errorf("Markets[0].URL = %q, want %q", got.Markets[0].URL, "https://example.com/core.git")
	}
	if !got.Markets[0].Trusted {
		t.Error("Markets[0].Trusted = false, want true")
	}
}

func TestAddMarket(t *testing.T) {
	cs, path := newConfigStore(t)
	if err := cs.Save(path, domain.Config{}); err != nil {
		t.Fatalf("Save seed config: %v", err)
	}

	market := domain.MarketConfig{Name: "mymarket", URL: "https://example.com/market.git", Branch: "main"}
	if err := cs.AddMarket(path, market); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	found := false
	for _, m := range cfg.Markets {
		if m.Name == "mymarket" {
			found = true
			break
		}
	}
	if !found {
		t.Error("market 'mymarket' not found after AddMarket")
	}
}

func TestRemoveMarket(t *testing.T) {
	cs, path := newConfigStore(t)
	if err := cs.Save(path, domain.Config{}); err != nil {
		t.Fatalf("Save seed config: %v", err)
	}

	market := domain.MarketConfig{Name: "removeme", URL: "https://example.com/rm.git", Branch: "main"}
	if err := cs.AddMarket(path, market); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}

	if err := cs.RemoveMarket(path, "removeme"); err != nil {
		t.Fatalf("RemoveMarket: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, m := range cfg.Markets {
		if m.Name == "removeme" {
			t.Error("market 'removeme' still present after RemoveMarket")
		}
	}
}

func TestSetMarketProperty(t *testing.T) {
	cs, path := newConfigStore(t)
	if err := cs.Save(path, domain.Config{}); err != nil {
		t.Fatalf("Save seed config: %v", err)
	}

	market := domain.MarketConfig{Name: "testmkt", URL: "https://example.com/test.git", Branch: "main"}
	if err := cs.AddMarket(path, market); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}

	// Set branch
	if err := cs.SetMarketProperty(path, "testmkt", "branch", "develop"); err != nil {
		t.Fatalf("SetMarketProperty(branch): %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var found *domain.MarketConfig
	for i := range cfg.Markets {
		if cfg.Markets[i].Name == "testmkt" {
			found = &cfg.Markets[i]
			break
		}
	}
	if found == nil {
		t.Fatal("market 'testmkt' not found")
	}
	if found.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", found.Branch, "develop")
	}

	// Set trusted
	if err := cs.SetMarketProperty(path, "testmkt", "trusted", "true"); err != nil {
		t.Fatalf("SetMarketProperty(trusted): %v", err)
	}

	cfg, err = cs.Load(path)
	if err != nil {
		t.Fatalf("Load after SetMarketProperty(trusted): %v", err)
	}
	for _, m := range cfg.Markets {
		if m.Name == "testmkt" {
			if !m.Trusted {
				t.Error("Trusted = false after SetMarketProperty(trusted, true)")
			}
			break
		}
	}
}

func TestSetConfigField(t *testing.T) {
	cs, path := newConfigStore(t)
	if err := cs.Save(path, domain.Config{}); err != nil {
		t.Fatalf("Save seed config: %v", err)
	}

	// Set local_path
	if err := cs.SetConfigField(path, "local_path", "/custom"); err != nil {
		t.Fatalf("SetConfigField(local_path): %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LocalPath != "/custom" {
		t.Errorf("LocalPath = %q, want %q", cfg.LocalPath, "/custom")
	}

	// Set ssh_enabled = true
	if err := cs.SetConfigField(path, "ssh_enabled", "true"); err != nil {
		t.Fatalf("SetConfigField(ssh_enabled): %v", err)
	}

	cfg, err = cs.Load(path)
	if err != nil {
		t.Fatalf("Load after SetConfigField(ssh_enabled): %v", err)
	}
	if cfg.SSHEnabled == nil {
		t.Fatal("SSHEnabled = nil, want *true")
	}
	if !*cfg.SSHEnabled {
		t.Error("SSHEnabled = false, want true")
	}
}
