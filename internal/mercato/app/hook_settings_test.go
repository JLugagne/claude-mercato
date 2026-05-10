package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

func TestReadSettings_Empty(t *testing.T) {
	doc, err := readSettings(nil)
	if err != nil {
		t.Fatal(err)
	}
	hooks, err := doc.hooksByEvent()
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 0 {
		t.Errorf("expected empty hooks, got %v", hooks)
	}
}

func TestReadSettings_PreservesSiblings(t *testing.T) {
	src := []byte(`{
		"theme": "dark",
		"keybindings": {"submit": "ctrl+enter"},
		"max_tokens": 4096
	}`)
	doc, err := readSettings(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.marshal()
	if err != nil {
		t.Fatal(err)
	}
	// All sibling keys must round-trip.
	for _, k := range []string{`"theme"`, `"keybindings"`, `"max_tokens"`, `"dark"`, `"submit"`, `4096`} {
		if !strings.Contains(string(out), k) {
			t.Errorf("output missing %q:\n%s", k, out)
		}
	}
}

func TestSettingsDoc_AppendAndRemoveHook(t *testing.T) {
	doc, _ := readSettings(nil)
	hooks, _ := doc.hooksByEvent()

	snippet, err := domain.ParseHookSnippet([]byte(`{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"go vet ./..."}]}`))
	if err != nil {
		t.Fatal(err)
	}
	mctID := mctIDForRef("market@hooks/go-vet.json")
	injected, err := injectMctID(snippet, mctID)
	if err != nil {
		t.Fatal(err)
	}
	hooks[snippet.Event] = append(hooks[snippet.Event], injected...)
	if err := doc.setHooksByEvent(hooks); err != nil {
		t.Fatal(err)
	}

	out, _ := doc.marshal()
	if !strings.Contains(string(out), mctID) {
		t.Errorf("output missing mct_id:\n%s", out)
	}
	if !strings.Contains(string(out), "go vet") {
		t.Errorf("output missing command:\n%s", out)
	}

	// Now remove by mct_id.
	doc2, _ := readSettings(out)
	hooks2, _ := doc2.hooksByEvent()
	for event, list := range hooks2 {
		filtered := make([]json.RawMessage, 0, len(list))
		for _, obj := range list {
			if extractMctID(obj) != mctID {
				filtered = append(filtered, obj)
			}
		}
		hooks2[event] = filtered
	}
	if err := doc2.setHooksByEvent(hooks2); err != nil {
		t.Fatal(err)
	}
	out2, _ := doc2.marshal()
	if strings.Contains(string(out2), mctID) {
		t.Errorf("expected mct_id removed:\n%s", out2)
	}
	if strings.Contains(string(out2), `"hooks"`) {
		t.Errorf("expected hooks key dropped after removal:\n%s", out2)
	}
}

func TestConflictExists(t *testing.T) {
	hooks := map[string][]json.RawMessage{
		"PreToolUse": {
			json.RawMessage(`{"type":"command","command":"x","matcher":"Bash","mct_id":"abc"}`),
		},
	}
	if !conflictExists(hooks, "PreToolUse", "Bash", "different-id") {
		t.Error("expected conflict for different mct_id with same event+matcher")
	}
	if conflictExists(hooks, "PreToolUse", "Bash", "abc") {
		t.Error("same mct_id should not conflict with itself")
	}
	if conflictExists(hooks, "PreToolUse", "Edit", "different-id") {
		t.Error("different matcher should not conflict")
	}
	if conflictExists(hooks, "PostToolUse", "Bash", "different-id") {
		t.Error("different event should not conflict")
	}
}

func TestMctIDForRef_Deterministic(t *testing.T) {
	a := mctIDForRef("m@hooks/foo.json")
	b := mctIDForRef("m@hooks/foo.json")
	if a != b {
		t.Errorf("expected determinism: %q vs %q", a, b)
	}
	c := mctIDForRef("m@hooks/bar.json")
	if a == c {
		t.Errorf("different refs should give different ids")
	}
}

func TestCanonicalHookBody_StripsMctID(t *testing.T) {
	body := json.RawMessage(`{"type":"command","mct_id":"abc","command":"x"}`)
	canonical, err := canonicalHookBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(canonical), "mct_id") {
		t.Errorf("canonical should not contain mct_id: %s", canonical)
	}
	// Different mct_id but same body should produce same canonical bytes.
	body2 := json.RawMessage(`{"type":"command","mct_id":"xyz","command":"x"}`)
	canonical2, _ := canonicalHookBody(body2)
	if string(canonical) != string(canonical2) {
		t.Errorf("canonical should be stable across mct_id changes:\n%s\nvs\n%s", canonical, canonical2)
	}
}
