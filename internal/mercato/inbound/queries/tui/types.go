package tui

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

type Mode int

const (
	ModeNormal         Mode = iota
	ModeSearch
	ModeConfirm
	ModeHelp
	ModeCommandPalette
	ModeLoading
	ModeDiff
	ModeMarketPopup
	ModeProfileAction
)

type Focus int

const (
	FocusProfiles Focus = iota
	FocusDetail
	FocusEntries
	FocusContent
)

type EntryItem struct {
	Entry  domain.Entry
	Status domain.EntryState
}

func (e EntryItem) Title() string       { return string(e.Entry.Ref) }
func (e EntryItem) Description() string { return e.Entry.Description }
func (e EntryItem) FilterValue() string { return string(e.Entry.Ref) + " " + e.Entry.Description }

type ProfileItem struct {
	Name         string // e.g. "dev/go"
	Market       string
	Desc         string // from README frontmatter "description"
	Entries      []domain.Entry
	Readme       string
	Tags         []string
	HasInstalled bool
	HasOutdated  bool
}

func (p ProfileItem) Title() string       { return p.Name + "@" + p.Market }
func (p ProfileItem) Description() string { return "" }
func (p ProfileItem) FilterValue() string { return p.Name + " " + p.Market }

type MarketItem struct {
	Market domain.Market
	Status string
}

func (m MarketItem) Title() string       { return m.Market.Name }
func (m MarketItem) Description() string { return m.Market.URL }
func (m MarketItem) FilterValue() string { return m.Market.Name }

type EntryContentReader interface {
	ReadEntryContent(market, relPath string) ([]byte, error)
}

type TUIServices struct {
	Markets service.MarketCommands
	Sync    service.SyncCommands
	Entries service.EntryCommands
	Search  service.SearchQueries
	Content EntryContentReader
	Check   service.SyncQueries
}
