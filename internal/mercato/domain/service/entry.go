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
	Pin(ref domain.MctRef, sha string) error
	Diff(ref domain.MctRef) error
	Init(opts InitOpts) error
}

type ListOpts struct {
	Market    string
	Type      domain.EntryType
	Installed bool
}

type AddOpts struct {
	Pin            string
	AcceptBreaking bool
	NoDeps         bool
	DryRun         bool
}

type PruneOpts struct {
	Ref       domain.MctRef
	AllKeep   bool
	AllRemove bool
}

type PruneResult struct {
	Ref    domain.MctRef
	Action string
	Err    error
}

type InitOpts struct {
	Markets   map[string]string
	LocalPath string
	CI        bool
}
