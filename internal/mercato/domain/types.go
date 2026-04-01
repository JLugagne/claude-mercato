package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type MctRef string
type MctVersion string
type EntryType string

// Parse splits a ref into its market name and relative path components.
// Returns an error if the ref is not in "market@path" format.
func (r MctRef) Parse() (market, relPath string, err error) {
	s := string(r)
	idx := strings.Index(s, "@")
	if idx <= 0 || idx == len(s)-1 {
		return "", "", &DomainError{
			Code:    "INVALID_REF",
			Message: "ref must be in format market@path",
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
	Name       string `json:"name"`
	URL        string `json:"url"`
	Branch     string `json:"branch"`
	ClonePath  string `json:"clone_path,omitempty"`
	Trusted    bool   `json:"trusted"`
	ReadOnly   bool   `json:"read_only"`
	SkillsOnly bool   `json:"skills_only"`
	SkillsPath string `json:"skills_path,omitempty"`
}

type Entry struct {
	Ref                MctRef     `json:"ref"`
	Market             string     `json:"market"`
	RelPath            string     `json:"rel_path"`
	Filename           string     `json:"filename"`
	Category           string     `json:"category"`
	Type               EntryType  `json:"type"`
	Description        string     `json:"description"`
	Author             string     `json:"author,omitempty"`
	MctTags            []string   `json:"mct_tags,omitempty"`
	Version            MctVersion `json:"version"`
	Profile            string     `json:"profile,omitempty"`
	Deleted            bool       `json:"deleted"`
	Installed          bool       `json:"installed"`
	BreakingChange     bool       `json:"breaking_change"`
	Deprecated         bool       `json:"deprecated"`
	RequiresSkills     []SkillDep `json:"requires_skills,omitempty"`
	ReadmeContext      string     `json:"-"`
	ProfileDescription string     `json:"-"`
}

type SkillDep struct {
	File   string `yaml:"file" json:"file"`
	Pin    string `yaml:"pin,omitempty" json:"pin,omitempty"`
	Market string `yaml:"market,omitempty" json:"market,omitempty"`
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
	MctProfile        string     `json:"mct_profile"`
	InstalledAt       time.Time  `json:"installed_at"`
	ChecksumAtInstall string     `json:"checksum_at_install"`
}

type EntryState int

const (
	StateClean EntryState = iota
	StateUpdateAvailable
	StateDrift
	StateUpdateAndDrift
	StateDeleted
	StateNewInRegistry
	StateOrphaned
	StateUnknown
)

var entryStateNames = map[EntryState]string{
	StateClean:           "clean",
	StateUpdateAvailable: "update_available",
	StateDrift:           "drift",
	StateUpdateAndDrift:  "update_and_drift",
	StateDeleted:         "deleted",
	StateNewInRegistry:   "new",
	StateOrphaned:        "orphaned",
	StateUnknown:         "unknown",
}

func (s EntryState) String() string {
	if name, ok := entryStateNames[s]; ok {
		return name
	}
	return "unknown"
}

func (s EntryState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

type EntryStatus struct {
	Ref        MctRef     `json:"ref"`
	State      EntryState `json:"state"`
	NewVersion MctVersion `json:"new_version,omitempty"`
}

type Conflict struct {
	Type        string   `json:"type"`
	Refs        []MctRef `json:"refs"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
}

// SkillDirFile represents a file inside a skill directory in a market repo.
type SkillDirFile struct {
	Path    string // relative path within the market (e.g. "skills/azure-ai/SKILL.md")
	Name    string // filename only (e.g. "SKILL.md")
	Content string // populated only for .md files
}

type ReadmeEntry struct {
	Market  string
	Path    string // relative path in market repo (e.g. "dev/go/README.md")
	Content string
	MctTags []string // parsed from README.md frontmatter
}
