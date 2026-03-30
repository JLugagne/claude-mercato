package cfgadapter

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

type ConfigStoreAdapter struct{}

func NewConfigStore() *ConfigStoreAdapter { return &ConfigStoreAdapter{} }

func (c *ConfigStoreAdapter) Load(path string) (domain.Config, error) {
	var cfg domain.Config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = domain.Config{}
			if saveErr := c.Save(path, cfg); saveErr != nil {
				return cfg, fmt.Errorf("create default config: %w", saveErr)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				return cfg, fmt.Errorf("load config: %w", err)
			}
		} else {
			return cfg, fmt.Errorf("load config: %w", err)
		}
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.LocalPath == "" {
		cfg.LocalPath = ".claude/"
	}
	if cfg.ConflictPolicy == "" {
		cfg.ConflictPolicy = "block"
	}
	if cfg.DriftPolicy == "" {
		cfg.DriftPolicy = "prompt"
	}
	if cfg.StaleAfter == 0 {
		cfg.StaleAfter = 7 * 24 * 60 * 60 * 1e9
	}
	for i := range cfg.Markets {
		if cfg.Markets[i].Branch == "" {
			cfg.Markets[i].Branch = "main"
		}
	}
	return cfg, nil
}

func (c *ConfigStoreAdapter) Save(path string, cfg domain.Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func (c *ConfigStoreAdapter) AddMarket(path string, market domain.MarketConfig) error {
	cfg, err := c.Load(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	cfg.Markets = append(cfg.Markets, market)
	return c.Save(path, cfg)
}

func (c *ConfigStoreAdapter) RemoveMarket(path string, name string) error {
	cfg, err := c.Load(path)
	if err != nil {
		return err
	}
	markets := make([]domain.MarketConfig, 0, len(cfg.Markets))
	for _, m := range cfg.Markets {
		if m.Name != name {
			markets = append(markets, m)
		}
	}
	cfg.Markets = markets
	return c.Save(path, cfg)
}

func (c *ConfigStoreAdapter) SetConfigField(path string, key string, value string) error {
	cfg, err := c.Load(path)
	if err != nil {
		return err
	}
	switch key {
	case "ssh_enabled":
		v := value == "true" || value == "1"
		cfg.SSHEnabled = &v
	case "local_path":
		cfg.LocalPath = value
	case "conflict_policy":
		cfg.ConflictPolicy = value
	case "drift_policy":
		cfg.DriftPolicy = value
	case "difftool":
		cfg.Difftool = value
	default:
		return fmt.Errorf("unknown config field: %s", key)
	}
	return c.Save(path, cfg)
}

func (c *ConfigStoreAdapter) SetMarketProperty(path string, marketName string, key string, value string) error {
	cfg, err := c.Load(path)
	if err != nil {
		return err
	}
	found := false
	for i := range cfg.Markets {
		if cfg.Markets[i].Name == marketName {
			found = true
			switch key {
			case "name":
				cfg.Markets[i].Name = value
			case "branch":
				cfg.Markets[i].Branch = value
			case "url":
				cfg.Markets[i].URL = value
			case "trusted":
				cfg.Markets[i].Trusted = value == "true"
			case "read_only":
				cfg.Markets[i].ReadOnly = value == "true"
			default:
				return fmt.Errorf("unknown market property: %s", key)
			}
			break
		}
	}
	if !found {
		return fmt.Errorf("market %q not found", marketName)
	}
	return c.Save(path, cfg)
}
