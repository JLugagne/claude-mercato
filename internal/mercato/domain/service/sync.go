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
	AllLocations   bool
}

type SyncOpts struct {
	Market         string
	DryRun         bool
	CI             bool
	AcceptBreaking bool
	AllMerge       bool
	AllLocations   bool
}

type RefreshResult struct {
	Market           string `json:"market"`
	OldSHA           string `json:"old_sha"`
	NewSHA           string `json:"new_sha"`
	ChangedFiles     int    `json:"changed_files"`
	UpdatesAvailable int    `json:"updates_available"`
	Err              error  `json:"error,omitempty"`
}

type UpdateResult struct {
	Ref        domain.MctRef     `json:"ref"`
	Location   string            `json:"location,omitempty"`
	Action     string            `json:"action"`
	OldVersion domain.MctVersion `json:"old_version"`
	NewVersion domain.MctVersion `json:"new_version"`
	DriftFiles []string          `json:"drift_files,omitempty"`
	Err        error             `json:"error,omitempty"`
}

type SyncResult struct {
	Refresh RefreshResult  `json:"refresh"`
	Updates []UpdateResult `json:"updates"`
}
