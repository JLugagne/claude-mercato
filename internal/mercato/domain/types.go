package domain

import (
	"strings"
	"time"
)

type MctRef string
type MctVersion string
type EntryType string

// Parse splits a ref into its market name and relative path components.
// Returns an error if the ref is not in "market/path" format.
func (r MctRef) Parse() (market, relPath string, err error) {
	s := string(r)
	idx := strings.Index(s, "/")
	if idx <= 0 || idx == len(s)-1 {
		return "", "", &DomainError{
			Code:    "INVALID_REF",
			Message: "ref must be in format market/path",
		}
	}
	return s[:idx], s[idx+1:], nil
}

// Market returns the market portion of a ref, or empty string if invalid.
func (r MctRef) Market() string {
	m, _, _ := r.Parse()
	return m
}

// RelPath returns the relative path portion of a ref, or the full ref if invalid.
func (r MctRef) RelPath() string {
	_, p, err := r.Parse()
	if err != nil {
		return string(r)
	}
	return p
}

const (
	EntryTypeAgent EntryType = "agent"
	EntryTypeSkill EntryType = "skill"
)

type Market struct {
	Name      string
	URL       string
	Branch    string
	ClonePath string
	Trusted   bool
	ReadOnly  bool
}

type Entry struct {
	Ref            MctRef
	Market         string
	RelPath        string
	Filename       string
	Category       string
	Type           EntryType
	Description    string
	Author         string
	MctTags        []string // inherited from profile README.md frontmatter
	Version        MctVersion
	Deleted        bool
	Installed      bool
	BreakingChange bool
	Deprecated     bool
	RequiresSkills []SkillDep
	ReadmeContext      string `json:"-"`
	ProfileDescription string `json:"-"`
}

type SkillDep struct {
	File string `yaml:"file"`
	Pin  string `yaml:"pin,omitempty"`
}

type Tombstone struct {
	Ref            MctRef
	Type           EntryType
	Description    string
	Deleted        bool
	DeletedAt      time.Time
	DeletedSHA     string
	KeptLocally    bool
	RemovedLocally bool
}

type ChecksumEntry struct {
	LocalPath         string     `json:"local_path"`
	MctRef            MctRef     `json:"mct_ref"`
	MctVersion        MctVersion `json:"mct_version"`
	InstalledAt       time.Time  `json:"installed_at"`
	ChecksumAtInstall string     `json:"checksum_at_install"`
	Status            string     `json:"status"`
}

type EntryState int

const (
	StateClean           EntryState = iota
	StateUpdateAvailable
	StateDrift
	StateUpdateAndDrift
	StateDeleted
	StateNewInRegistry
	StateOrphaned
	StateUnknown
)

type EntryStatus struct {
	Ref        MctRef
	State      EntryState
	NewVersion MctVersion
}

type Conflict struct {
	Type        string
	Refs        []MctRef
	Description string
	Severity    string
}

type ReadmeEntry struct {
	Market  string
	Path    string // relative path in market repo (e.g. "dev/go/README.md")
	Content string
	MctTags []string // parsed from README.md frontmatter
}
