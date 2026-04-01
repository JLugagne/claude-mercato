package service

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type EntryQueries interface {
	List(opts ListOpts) ([]domain.Entry, error)
	GetEntry(ref domain.MctRef) (domain.Entry, error)
	ReadEntryContent(market, relPath string) ([]byte, error)
	Conflicts() ([]domain.Conflict, error)
}

type EntryCommands interface {
	EntryQueries
	Add(ref domain.MctRef, opts AddOpts) error
	Remove(ref domain.MctRef) error
	Prune(opts PruneOpts) ([]PruneResult, error)
	Diff(ref domain.MctRef) error
	Init(opts InitOpts) error
}

type ListOpts struct {
	Market    string
	Type      domain.EntryType
	Installed bool
}

// ConfirmMarketFunc is called when an agent requires a skill from an
// unregistered market. It receives the market URL and returns true if the
// user agrees to register it.
type ConfirmMarketFunc func(marketURL string) bool

type AddOpts struct {
	Profile        string
	AcceptBreaking bool
	NoDeps         bool
	DryRun         bool
	ConfirmMarket  ConfirmMarketFunc
}

type PruneOpts struct {
	Ref       domain.MctRef
	AllKeep   bool
	AllRemove bool
}

type PruneResult struct {
	Ref    domain.MctRef `json:"ref"`
	Action string        `json:"action"`
	Err    error         `json:"error,omitempty"`
}

type InitOpts struct {
	Markets   []string
	LocalPath string
	CI        bool
}
