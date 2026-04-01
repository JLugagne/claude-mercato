package tui

import (
	"time"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

type MarketLoadedMsg struct {
	Market  string
	Entries []domain.Entry
	Err     error
}

type IndexReadyMsg struct {
	Results  []service.SearchResult
	Statuses []domain.EntryStatus
	Elapsed  time.Duration
}

type StartupDoneMsg struct{}

type FetchCompleteMsg struct {
	Market string
	NewSHA string
	Err    error
}

type DiffCompleteMsg struct {
	Ref domain.MctRef
	Err error
}

type InstallCompleteMsg struct {
	Ref domain.MctRef
	Err error
}

type UpdateCompleteMsg struct {
	Ref        domain.MctRef
	NewVersion domain.MctVersion
	Err        error
}

type PruneCompleteMsg struct {
	Ref    domain.MctRef
	Action string
	Err    error
}

type SearchResultMsg struct {
	Query   string
	Results []service.SearchResult
}

type SearchTickMsg struct{}

type MarketAddedMsg struct {
	Name string
	Err  error
}

type MarketRemovedMsg struct {
	Name string
	Err  error
}

type MarketRenamedMsg struct {
	OldName string
	NewName string
	Err     error
}

type EntryContentMsg struct {
	Ref     domain.MctRef
	Content string
	Err     error
}

type DetailContentMsg struct {
	Content string
}

type ProfileInstallMsg struct {
	Profile string
	Errors  []error
}

type ProfileRemoveMsg struct {
	Profile string
	Errors  []error
}

type SkillDirFilesMsg struct {
	Market string
	Dir    string
	Files  []domain.SkillDirFile
	Err    error
}
