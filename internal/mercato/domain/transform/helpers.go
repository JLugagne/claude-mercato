package transform

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"gopkg.in/yaml.v3"
)

// extractBody returns everything after the closing --- of the frontmatter.
func extractBody(content []byte) []byte {
	s := string(content)
	if !strings.HasPrefix(s, "---") {
		return content
	}
	end := strings.Index(s[3:], "\n---")
	if end == -1 {
		return content
	}
	// end is offset within s[3:] where "\n---" starts
	// absolute position of closing "---" ends at: 3 + end + len("\n---") = 3 + end + 4
	closingEnd := 3 + end + 4 // position right after the closing "---"
	if closingEnd < len(s) && s[closingEnd] == '\n' {
		closingEnd++ // skip the newline after closing ---
	}
	return []byte(s[closingEnd:])
}

// buildFrontmatter builds ---\nkey: val\n---\n from a map of fields.
// Fields are written in the order provided by using a slice of pairs.
func buildFrontmatter(fields [][2]string) []byte {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	for _, kv := range fields {
		fmt.Fprintf(&buf, "%s: %s\n", kv[0], kv[1])
	}
	buf.WriteString("---\n")
	return buf.Bytes()
}

// parseDescription extracts the description field from raw content frontmatter.
func parseDescription(content []byte) string {
	fmBytes, err := domain.ExtractFrontmatterBytes(content)
	if err != nil {
		return ""
	}
	var fm struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return ""
	}
	return fm.Description
}

// parseModel extracts the model field from raw content frontmatter.
func parseModel(content []byte) string {
	fmBytes, err := domain.ExtractFrontmatterBytes(content)
	if err != nil {
		return ""
	}
	var fm struct {
		Model string `yaml:"model"`
	}
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return ""
	}
	return fm.Model
}

// parseTools extracts the tools field from raw content frontmatter.
func parseTools(content []byte) []string {
	fmBytes, err := domain.ExtractFrontmatterBytes(content)
	if err != nil {
		return nil
	}
	var fm struct {
		Tools []string `yaml:"tools"`
	}
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return nil
	}
	return fm.Tools
}

// entryName returns the base name of an entry (without .md extension for agents).
func entryName(entry domain.Entry) string {
	name := entry.Filename
	if entry.Type == domain.EntryTypeAgent {
		name = strings.TrimSuffix(name, ".md")
	}
	return name
}

