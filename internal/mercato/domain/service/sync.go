package service

import "github.com/JLugagne/claude-mercato/internal/mercato/domain"

type SyncQueries interface {
	Check(opts CheckOpts) ([]domain.EntryStatus, error)
	SyncState() (domain.SyncState, error)
}

type SyncCommands interface {
	SyncQueries
	Refresh(opts RefreshOpts) ([]RefreshResult, error)
	Update(opts UpdateOpts) ([]UpdateResult, error)
	Sync(opts SyncOpts) ([]SyncResult, error)
}

type CheckOpts struct {
	Market string
	Short  bool
	JSON   bool
	CI     bool
}

type RefreshOpts struct {
	Market string
	DryRun bool
	CI     bool
}

type UpdateOpts struct {
	Ref            domain.MctRef
	Market         string
	AgentsOnly     bool
	SkillsOnly     bool
	AllKeep        bool
	AllDelete      bool
	AllMerge       bool
	AcceptBreaking bool
	DryRun         bool
	CI             bool
}

type SyncOpts struct {
	Market         string
	DryRun         bool
	CI             bool
	AcceptBreaking bool
	AllMerge       bool
}

type RefreshResult struct {
	Market       string
	OldSHA       string
	NewSHA       string
	ChangedFiles int
	Err          error
}

type UpdateResult struct {
	Ref        domain.MctRef
	Action     string
	OldVersion domain.MctVersion
	NewVersion domain.MctVersion
	Err        error
}

type SyncResult struct {
	Refresh RefreshResult
	Updates []UpdateResult
}

