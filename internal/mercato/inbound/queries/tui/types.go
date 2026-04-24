package tui

import (
	"strings"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

// profileDisplayName strips the leading directory prefix from a category
// (e.g. "skills/lint" -> "lint", "dev/go-hexagonal" -> "go-hexagonal").
func profileDisplayName(category string) string {
	if idx := strings.LastIndex(category, "/"); idx >= 0 {
		return category[idx+1:]
	}
	return category
}

type Mode int

const (
	ModeNormal Mode = iota
	ModeSearch
	ModeConfirm
	ModeHelp
	ModeCommandPalette
	ModeLoading
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

type SkillFileItem struct {
	File domain.SkillDirFile
}

func (s SkillFileItem) Title() string       { return s.File.Name }
func (s SkillFileItem) Description() string { return "" }
func (s SkillFileItem) FilterValue() string { return s.File.Name }

type MarketItem struct {
	Market domain.Market
	Status string
}

func (m MarketItem) Title() string       { return m.Market.Name }
func (m MarketItem) Description() string { return m.Market.URL }
func (m MarketItem) FilterValue() string { return m.Market.Name }

type TUIServices struct {
	Markets service.MarketCommands
	Sync    service.SyncCommands
	Entries service.EntryCommands
	Search  service.SearchQueries
}
