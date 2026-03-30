package service

import (
	"io/fs"
	"time"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

type MarketQueries interface {
	ListMarkets() ([]domain.Market, error)
	GetMarket(name string) (domain.Market, error)
	MarketInfo(name string) (MarketInfoResult, error)
}

type MarketCommands interface {
	MarketQueries
	AddMarket(name, url string, opts AddMarketOpts) (AddMarketResult, error)
	RemoveMarket(name string, opts RemoveMarketOpts) error
	RenameMarket(oldName, newName string) error
	SetMarketProperty(name, key, value string) error
	LintMarket(fsys fs.FS, dir string) (LintResult, error)
}

type LintResult struct {
	Profiles int
	Agents   int
	Skills   int
	Issues   []LintIssue
}

type LintIssue struct {
	Profile  string
	Severity string // "error" or "warn"
	Message  string
}

type AddMarketResult struct {
	Profiles int
	Agents   int
	Skills   int
}

type AddMarketOpts struct {
	Branch   string
	Trusted  bool
	ReadOnly bool
	NoClone  bool
}

type RemoveMarketOpts struct {
	Force     bool
	KeepCache bool
}

type MarketInfoResult struct {
	Market         domain.Market `json:"market"`
	EntryCount     int           `json:"entry_count"`
	InstalledCount int           `json:"installed_count"`
	LastSynced     time.Time     `json:"last_synced"`
	Status         string        `json:"status"`
}
