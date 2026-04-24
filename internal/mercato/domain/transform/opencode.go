package transform

import (
	"fmt"
	"strings"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// OpenCodeTransformer converts entries to OpenCode format.
// It supports both agents and skills.
type OpenCodeTransformer struct{}

func (t *OpenCodeTransformer) ToolName() string { return "opencode" }

func (t *OpenCodeTransformer) SupportsEntry(_ domain.EntryType) bool { return true }

func (t *OpenCodeTransformer) OutputPath(entry domain.Entry) string {
	name := entryName(entry)
	if entry.Type == domain.EntryTypeAgent {
		return ".opencode/agents/" + name + ".md"
	}
	return ".opencode/rules/" + name + ".md"
}

func (t *OpenCodeTransformer) Transform(entry domain.Entry, content []byte, mappings domain.ToolMapping) domain.TransformResult {
	if entry.Type == domain.EntryTypeAgent {
		return t.transformAgent(entry, content, mappings)
	}
	return t.transformSkill(entry, content)
}

func (t *OpenCodeTransformer) transformAgent(entry domain.Entry, content []byte, mappings domain.ToolMapping) domain.TransformResult {
	desc := parseDescription(content)
	model := parseModel(content)
	tools := parseTools(content)
	body := extractBody(content)

	var warnings []string
	fields := [][2]string{
		{"description", desc},
		{"mode", "primary"},
	}

	// Map model
	if model != "" {
		if mappings.Models != nil {
			if toolModels, ok := mappings.Models[model]; ok {
				mapped := toolModels["opencode"]
				if mapped != "" {
					fields = append(fields, [2]string{"model", mapped})
				}
				// empty string → strip silently
			} else {
				// model not in mapping at all → strip + warning
				warnings = append(warnings, fmt.Sprintf("unmapped model %q for opencode, stripping model field", model))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("unmapped model %q for opencode, stripping model field", model))
		}
	}

	// Map tools
	if len(tools) > 0 && mappings.Tools != nil {
		var mapped []string
		for _, tool := range tools {
			if toolMap, ok := mappings.Tools[tool]; ok {
				if v := toolMap["opencode"]; v != "" {
					mapped = append(mapped, v)
				}
			}
		}
		if len(mapped) > 0 {
			fields = append(fields, [2]string{"tools", strings.Join(mapped, ", ")})
		}
	}

	fm := buildFrontmatter(fields)
	var out []byte
	out = append(out, fm...)
	out = append(out, body...)

	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    out,
		OutputPath: t.OutputPath(entry),
		Warnings:   warnings,
	}
}

func (t *OpenCodeTransformer) transformSkill(entry domain.Entry, content []byte) domain.TransformResult {
	desc := parseDescription(content)
	body := extractBody(content)

	fm := buildFrontmatter([][2]string{
		{"description", desc},
	})

	var out []byte
	out = append(out, fm...)
	out = append(out, body...)

	return domain.TransformResult{
		ToolName:   t.ToolName(),
		Content:    out,
		OutputPath: t.OutputPath(entry),
	}
}
