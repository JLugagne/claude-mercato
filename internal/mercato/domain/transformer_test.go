package domain

import (
	"sort"
	"testing"
)

// stubTransformer is a minimal Transformer implementation for testing.
type stubTransformer struct {
	name           string
	supportedTypes map[EntryType]bool
}

func (s *stubTransformer) Transform(entry Entry, content []byte, mappings ToolMapping) TransformResult {
	return TransformResult{
		ToolName: s.name,
		Content:  content,
	}
}

func (s *stubTransformer) ToolName() string { return s.name }

func (s *stubTransformer) SupportsEntry(entryType EntryType) bool {
	return s.supportedTypes[entryType]
}

func (s *stubTransformer) OutputPath(entry Entry) string {
	return ".rules/" + entry.Filename
}

func TestTransformerRegistryGet(t *testing.T) {
	cursor := &stubTransformer{name: "cursor"}
	reg := TransformerRegistry{"cursor": cursor}

	t.Run("found", func(t *testing.T) {
		tr, ok := reg.Get("cursor")
		if !ok {
			t.Fatal("expected to find transformer")
		}
		if tr.ToolName() != "cursor" {
			t.Fatalf("got %q, want %q", tr.ToolName(), "cursor")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := reg.Get("windsurf")
		if ok {
			t.Fatal("expected not found")
		}
	})
}

func TestTransformerRegistryEnabledTransformers(t *testing.T) {
	cursor := &stubTransformer{name: "cursor"}
	windsurf := &stubTransformer{name: "windsurf"}
	codex := &stubTransformer{name: "codex"}
	reg := TransformerRegistry{
		"cursor":   cursor,
		"windsurf": windsurf,
		"codex":    codex,
	}

	t.Run("returns only enabled", func(t *testing.T) {
		tools := map[string]bool{"cursor": true, "codex": true, "windsurf": false}
		result := reg.EnabledTransformers(tools)
		if len(result) != 2 {
			t.Fatalf("got %d transformers, want 2", len(result))
		}
		names := []string{result[0].ToolName(), result[1].ToolName()}
		sort.Strings(names)
		if names[0] != "codex" || names[1] != "cursor" {
			t.Fatalf("got %v, want [codex cursor]", names)
		}
	})

	t.Run("nil map returns empty", func(t *testing.T) {
		result := reg.EnabledTransformers(nil)
		if len(result) != 0 {
			t.Fatalf("got %d transformers, want 0", len(result))
		}
	})

	t.Run("empty map returns empty", func(t *testing.T) {
		result := reg.EnabledTransformers(map[string]bool{})
		if len(result) != 0 {
			t.Fatalf("got %d transformers, want 0", len(result))
		}
	})

	t.Run("unknown tool in map is ignored", func(t *testing.T) {
		tools := map[string]bool{"nonexistent": true}
		result := reg.EnabledTransformers(tools)
		if len(result) != 0 {
			t.Fatalf("got %d transformers, want 0", len(result))
		}
	})
}

func TestToolTargetValidation(t *testing.T) {
	t.Run("directory strategy", func(t *testing.T) {
		target := ToolTarget{
			Name:           "claude",
			Enabled:        true,
			DetectDir:      ".claude",
			OutputStrategy: OutputStrategyDirectory,
			SupportsAgents: true,
			SupportsSkills: true,
			FileExtension:  ".md",
		}
		if target.OutputStrategy != OutputStrategyDirectory {
			t.Fatalf("got %q, want %q", target.OutputStrategy, OutputStrategyDirectory)
		}
		if !target.SupportsAgents || !target.SupportsSkills {
			t.Fatal("expected both agents and skills supported")
		}
	})

	t.Run("flat strategy", func(t *testing.T) {
		target := ToolTarget{
			Name:           "cursor",
			Enabled:        false,
			DetectDir:      ".cursor",
			OutputStrategy: OutputStrategyFlat,
			SupportsAgents: true,
			SupportsSkills: false,
			FileExtension:  ".mdc",
		}
		if target.OutputStrategy != OutputStrategyFlat {
			t.Fatalf("got %q, want %q", target.OutputStrategy, OutputStrategyFlat)
		}
		if target.SupportsSkills {
			t.Fatal("expected skills not supported")
		}
	})
}

func TestStubTransformerSupportsEntry(t *testing.T) {
	tr := &stubTransformer{
		name:           "test",
		supportedTypes: map[EntryType]bool{EntryTypeAgent: true},
	}

	if !tr.SupportsEntry(EntryTypeAgent) {
		t.Fatal("expected agent supported")
	}
	if tr.SupportsEntry(EntryTypeSkill) {
		t.Fatal("expected skill not supported")
	}
}

func TestTransformResultFields(t *testing.T) {
	result := TransformResult{
		ToolName:   "cursor",
		Content:    []byte("rules content"),
		OutputPath: ".cursor/rules/agent.mdc",
		Warnings:   []string{"model not mapped"},
		Skipped:    false,
		SkipReason: "",
	}
	if result.ToolName != "cursor" {
		t.Fatalf("got %q, want %q", result.ToolName, "cursor")
	}
	if result.Skipped {
		t.Fatal("expected not skipped")
	}

	skipped := TransformResult{
		ToolName:   "windsurf",
		Skipped:    true,
		SkipReason: "skills not supported",
	}
	if !skipped.Skipped {
		t.Fatal("expected skipped")
	}
	if skipped.SkipReason != "skills not supported" {
		t.Fatalf("got %q, want %q", skipped.SkipReason, "skills not supported")
	}
}

func TestToolMappingFields(t *testing.T) {
	mapping := ToolMapping{
		Models: map[string]map[string]string{
			"claude-sonnet-4-20250514": {
				"cursor": "claude-sonnet-4-20250514",
				"codex":  "o3",
			},
		},
		Tools: map[string]map[string]string{
			"Bash": {
				"cursor": "terminal",
			},
		},
	}

	if mapping.Models["claude-sonnet-4-20250514"]["cursor"] != "claude-sonnet-4-20250514" {
		t.Fatal("model mapping mismatch")
	}
	if mapping.Tools["Bash"]["cursor"] != "terminal" {
		t.Fatal("tool mapping mismatch")
	}
}
