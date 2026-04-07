package transform

import (
	"testing"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
)

func TestTransformerRegistry_AllRegistered(t *testing.T) {
	registry := domain.TransformerRegistry{
		"claude":      &ClaudeTransformer{},
		"cursor":      &CursorTransformer{},
		"windsurf":    &WindsurfTransformer{},
		"codex":       &CodexTransformer{},
		"gemini":      &GeminiTransformer{},
		"opencode":    &OpenCodeTransformer{},
		"copilot":     &CopilotTransformer{},
		"supermaven":  &SupermavenTransformer{},
		"pearai":      &PearAITransformer{},
		"roocode":     &RooCodeTransformer{},
		"continue":    &ContinueTransformer{},
	}

	expected := []string{"claude", "cursor", "windsurf", "codex", "gemini", "opencode", "copilot", "supermaven", "pearai", "roocode", "continue"}
	for _, name := range expected {
		tr, ok := registry.Get(name)
		if !ok {
			t.Errorf("transformer %q not found in registry", name)
			continue
		}
		if tr.ToolName() != name {
			t.Errorf("transformer %q: ToolName() = %q, want %q", name, tr.ToolName(), name)
		}
	}

	if len(registry) != 11 {
		t.Errorf("expected 11 transformers in registry, got %d", len(registry))
	}
}
