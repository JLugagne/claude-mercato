package service

import "github.com/JLugagne/agents-mercato/internal/mercato/domain"

type SyncQueries interface {
	Check(opts CheckOpts) ([]domain.EntryStatus, error)
	SyncState() (domain.SyncState, error)
}

type SyncCommands interface {
	SyncQueries
	Refresh(opts RefreshOpts) ([]RefreshResult, error)
	Update(opts UpdateOpts) ([]UpdateResult, error)
	Sync(opts SyncOpts) ([]SyncResult, error)
	DetectDeleted(opts DetectDeletedOpts) ([]DeletedFile, error)
	RestoreDeleted(files []DeletedFile, opts RestoreOpts) ([]RestoreResult, error)
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
	Market           string   `json:"market"`
	OldSHA           string   `json:"old_sha"`
	NewSHA           string   `json:"new_sha"`
	ChangedFiles     int      `json:"changed_files"`
	UpdatesAvailable int      `json:"updates_available"`
	Err              error    `json:"error,omitempty"`
	PrunedLocations  []string `json:"pruned_locations,omitempty"`
	PrunedFiles      []string `json:"pruned_files,omitempty"`
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
	Refresh  RefreshResult   `json:"refresh"`
	Updates  []UpdateResult  `json:"updates"`
	Restored []RestoreResult `json:"restored,omitempty"`
}

// DeletedFile identifies a file that was recorded as installed in the
// install database but is no longer present on disk (locally deleted by
// the user, not pruned by sync).
type DeletedFile struct {
	Market   string `json:"market"`
	Profile  string `json:"profile"`
	Location string `json:"location"`
	RelPath  string `json:"rel_path"`
	XXH      string `json:"xxh"`
}

type DetectDeletedOpts struct {
	Market string
}

type RestoreOpts struct {
	DryRun bool
}

type RestoreResult struct {
	File    DeletedFile `json:"file"`
	Action  string      `json:"action"` // "restored" | "failed"
	Err     error       `json:"error,omitempty"`
}
