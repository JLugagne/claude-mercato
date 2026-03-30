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

func TestLoadConfig_Empty(t *testing.T) {
	cs, path := newConfigStore(t)

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load on nonexistent file: %v", err)
	}

	// Default values should be applied
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
		Entries: []domain.EntryConfig{
			{Ref: "core/my-skill", Pin: "abc123"},
		},
		ManagedSkills: []domain.ManagedSkillConfig{
			{Ref: "core/managed", ManagedBy: "core/manager", MctVersion: "1.0.0"},
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
	if len(got.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(got.Entries))
	}
	if got.Entries[0].Ref != "core/my-skill" {
		t.Errorf("Entries[0].Ref = %q, want %q", got.Entries[0].Ref, "core/my-skill")
	}
	if got.Entries[0].Pin != "abc123" {
		t.Errorf("Entries[0].Pin = %q, want %q", got.Entries[0].Pin, "abc123")
	}
	if len(got.ManagedSkills) != 1 {
		t.Fatalf("len(ManagedSkills) = %d, want 1", len(got.ManagedSkills))
	}
	if got.ManagedSkills[0].Ref != "core/managed" {
		t.Errorf("ManagedSkills[0].Ref = %q, want %q", got.ManagedSkills[0].Ref, "core/managed")
	}
}

func TestAddMarket(t *testing.T) {
	cs, path := newConfigStore(t)

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

func TestAddEntry(t *testing.T) {
	cs, path := newConfigStore(t)

	entry := domain.EntryConfig{Ref: "core/my-agent"}
	if err := cs.AddEntry(path, entry); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	found := false
	for _, e := range cfg.Entries {
		if e.Ref == "core/my-agent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("entry 'core/my-agent' not found after AddEntry")
	}
}

func TestRemoveEntry(t *testing.T) {
	cs, path := newConfigStore(t)

	entry := domain.EntryConfig{Ref: "core/to-remove"}
	if err := cs.AddEntry(path, entry); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	if err := cs.RemoveEntry(path, "core/to-remove"); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, e := range cfg.Entries {
		if e.Ref == "core/to-remove" {
			t.Error("entry 'core/to-remove' still present after RemoveEntry")
		}
	}
}

func TestAddManagedSkill(t *testing.T) {
	cs, path := newConfigStore(t)

	skill := domain.ManagedSkillConfig{Ref: "core/skill-a", ManagedBy: "core/manager", MctVersion: "2.0.0"}
	if err := cs.AddManagedSkill(path, skill); err != nil {
		t.Fatalf("AddManagedSkill: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	found := false
	for _, s := range cfg.ManagedSkills {
		if s.Ref == "core/skill-a" {
			found = true
			break
		}
	}
	if !found {
		t.Error("managed skill 'core/skill-a' not found after AddManagedSkill")
	}
}

func TestRemoveManagedSkill(t *testing.T) {
	cs, path := newConfigStore(t)

	skill := domain.ManagedSkillConfig{Ref: "core/skill-b", ManagedBy: "core/manager", MctVersion: "1.0.0"}
	if err := cs.AddManagedSkill(path, skill); err != nil {
		t.Fatalf("AddManagedSkill: %v", err)
	}

	if err := cs.RemoveManagedSkill(path, "core/skill-b"); err != nil {
		t.Fatalf("RemoveManagedSkill: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, s := range cfg.ManagedSkills {
		if s.Ref == "core/skill-b" {
			t.Error("managed skill 'core/skill-b' still present after RemoveManagedSkill")
		}
	}
}

func TestSetMarketProperty(t *testing.T) {
	cs, path := newConfigStore(t)

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

func TestSetEntryPin(t *testing.T) {
	cs, path := newConfigStore(t)

	entry := domain.EntryConfig{Ref: "core/pinme"}
	if err := cs.AddEntry(path, entry); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	if err := cs.SetEntryPin(path, "core/pinme", "abc123"); err != nil {
		t.Fatalf("SetEntryPin: %v", err)
	}

	cfg, err := cs.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, e := range cfg.Entries {
		if e.Ref == "core/pinme" {
			if e.Pin != "abc123" {
				t.Errorf("Pin = %q, want %q", e.Pin, "abc123")
			}
			return
		}
	}
	t.Error("entry 'core/pinme' not found after SetEntryPin")
}

func TestSetConfigField(t *testing.T) {
	cs, path := newConfigStore(t)

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
