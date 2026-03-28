package domain

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	Type           EntryType  `yaml:"type"`
	Description    string     `yaml:"description"`
	Author         string     `yaml:"author"`
	BreakingChange bool       `yaml:"breaking_change"`
	Deprecated     bool       `yaml:"deprecated"`
	RequiresSkills []SkillDep `yaml:"requires_skills"`
	MctRef         MctRef     `yaml:"mct_ref"`
	MctVersion     MctVersion `yaml:"mct_version"`
	MctMarket      string     `yaml:"mct_market"`
	MctInstalledAt time.Time  `yaml:"mct_installed_at"`
}

type ReadmeFrontmatter struct {
	MctTags     []string `yaml:"tags"`
	Description string   `yaml:"description"`
}

func ParseReadmeFrontmatter(content []byte) (ReadmeFrontmatter, error) {
	var fm ReadmeFrontmatter
	fmBytes, err := ExtractFrontmatterBytes(content)
	if err != nil {
		return fm, err
	}
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return fm, fmt.Errorf("parse readme frontmatter: %w", err)
	}
	return fm, nil
}

func ParseFrontmatter(content []byte) (Frontmatter, error) {
	var fm Frontmatter
	fmBytes, err := ExtractFrontmatterBytes(content)
	if err != nil {
		return fm, err
	}
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return fm, fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, nil
}

func ExtractFrontmatterBytes(content []byte) ([]byte, error) {
	s := string(content)
	if !strings.HasPrefix(s, "---") {
		return nil, fmt.Errorf("content does not start with ---")
	}
	end := strings.Index(s[3:], "\n---")
	if end == -1 {
		return nil, fmt.Errorf("closing --- not found")
	}
	raw := s[3 : end+3]
	raw = strings.TrimPrefix(raw, "\n")
	return []byte(raw), nil
}

func InjectMctFields(content []byte, ref MctRef, version MctVersion, market string) ([]byte, error) {
	fmBytes, err := ExtractFrontmatterBytes(content)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	mctBlock := fmt.Sprintf("mct_ref: %s\nmct_version: %s\nmct_market: %s\nmct_installed_at: %s",
		string(ref), string(version), market, now)
	oldFM := string(fmBytes)
	newFM := mctBlock + "\n\n" + oldFM
	result := bytes.Replace(content, fmBytes, []byte(newFM), 1)
	return result, nil
}

var reMctVersion = regexp.MustCompile(`(?m)^mct_version:.*$`)

func PatchMctVersion(content []byte, newVersion MctVersion) ([]byte, error) {
	if !reMctVersion.Match(content) {
		return nil, fmt.Errorf("mct_version field not found in content")
	}
	return reMctVersion.ReplaceAll(content, []byte("mct_version: "+string(newVersion))), nil
}
