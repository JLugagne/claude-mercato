package domain

import (
	"strings"
	"testing"
)

func TestParseHookSnippet_Valid(t *testing.T) {
	data := []byte(`{
		"event": "PreToolUse",
		"matcher": "Bash",
		"hooks": [
			{ "type": "command", "command": "go vet ./..." }
		]
	}`)
	s, err := ParseHookSnippet(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Event != "PreToolUse" {
		t.Errorf("Event = %q, want PreToolUse", s.Event)
	}
	if s.Matcher != "Bash" {
		t.Errorf("Matcher = %q, want Bash", s.Matcher)
	}
	if len(s.Hooks) != 1 {
		t.Fatalf("len(Hooks) = %d, want 1", len(s.Hooks))
	}
	if !strings.Contains(string(s.Hooks[0]), `"command"`) {
		t.Errorf("hook body missing 'command': %s", s.Hooks[0])
	}
}

func TestParseHookSnippet_NoMatcher(t *testing.T) {
	// matcher is optional
	data := []byte(`{"event":"Stop","hooks":[{"type":"command","command":"echo hi"}]}`)
	s, err := ParseHookSnippet(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Matcher != "" {
		t.Errorf("Matcher should be empty, got %q", s.Matcher)
	}
}

func TestParseHookSnippet_MissingEvent(t *testing.T) {
	data := []byte(`{"hooks":[{"type":"command","command":"x"}]}`)
	_, err := ParseHookSnippet(data)
	if err == nil {
		t.Fatal("expected ErrInvalidHookSnippet")
	}
	if !isCode(err, "INVALID_HOOK_SNIPPET") {
		t.Errorf("expected INVALID_HOOK_SNIPPET, got %v", err)
	}
}

func TestParseHookSnippet_EmptyHooks(t *testing.T) {
	data := []byte(`{"event":"Stop","hooks":[]}`)
	_, err := ParseHookSnippet(data)
	if err == nil {
		t.Fatal("expected ErrInvalidHookSnippet")
	}
}

func TestParseHookSnippet_HookMissingType(t *testing.T) {
	data := []byte(`{"event":"Stop","hooks":[{"command":"x"}]}`)
	_, err := ParseHookSnippet(data)
	if err == nil {
		t.Fatal("expected ErrInvalidHookSnippet for hook without type")
	}
}

func TestParseHookSnippet_BadJSON(t *testing.T) {
	_, err := ParseHookSnippet([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func isCode(err error, code string) bool {
	if err == nil {
		return false
	}
	type coder interface{ DomainCode() string }
	if c, ok := err.(coder); ok {
		return c.DomainCode() == code
	}
	// Fall back to substring match on error message for *DomainError.
	return strings.Contains(err.Error(), code)
}
