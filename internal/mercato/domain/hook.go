package domain

import (
	"bytes"
	"encoding/json"
)

// HookSnippet represents a Claude Code hook definition stored as a JSON file
// under a market's hooks/ directory. The market file has the shape:
//
//	{
//	  "event":   "PreToolUse",
//	  "matcher": "Bash",
//	  "hooks": [
//	    { "type": "command", "command": "go vet ./..." }
//	  ]
//	}
//
// At install time the snippet's hooks[] entries are merged into the project's
// .claude/settings.json under settings.json["hooks"][Event]. The Matcher field
// is optional at the snippet level and is also propagated to each hook object
// in the merged settings.json (per Claude Code's hook configuration schema).
type HookSnippet struct {
	Event   string
	Matcher string
	// Hooks is the raw "hooks" array as it appears in the snippet — each
	// element a JSON object with at least a "type" key (and usually a
	// "command" key for "type":"command" hooks).
	Hooks []json.RawMessage
}

// ParseHookSnippet validates and decodes a hook snippet's bytes.
// Returns ErrInvalidHookSnippet when the snippet is missing the required
// fields or has unexpected shape. Numbers are decoded with json.Number to
// avoid silent integer→float64 mangling on round-trip.
func ParseHookSnippet(data []byte) (HookSnippet, error) {
	var s HookSnippet
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var raw struct {
		Event   string            `json:"event"`
		Matcher string            `json:"matcher"`
		Hooks   []json.RawMessage `json:"hooks"`
	}
	if err := dec.Decode(&raw); err != nil {
		return s, ErrInvalidHookSnippet.Wrap(err)
	}
	if raw.Event == "" {
		return s, ErrInvalidHookSnippet
	}
	if len(raw.Hooks) == 0 {
		return s, ErrInvalidHookSnippet
	}
	// Each hook object must be a JSON object with at least "type".
	for _, h := range raw.Hooks {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(h, &probe); err != nil {
			return s, ErrInvalidHookSnippet.Wrap(err)
		}
		if _, ok := probe["type"]; !ok {
			return s, ErrInvalidHookSnippet
		}
	}

	s.Event = raw.Event
	s.Matcher = raw.Matcher
	s.Hooks = raw.Hooks
	return s, nil
}
