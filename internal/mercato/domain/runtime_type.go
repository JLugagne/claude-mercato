package domain

import "strings"

// RuntimeTypeClaudeCode is the canonical type label for the built-in Claude
// path that does not flow through a Transformer. Other types come from the
// matching Transformer.ToolName() (cursor, windsurf, opencode, gemini, ...).
const RuntimeTypeClaudeCode = "claude-code"

// runtimeTypeByDotDir maps the leading dot-folder of a project-relative path
// to its runtime type. Used by the v1→v2 install database migration to
// classify legacy locations whose type was never recorded. New installs use
// the live transformer's ToolName() instead.
var runtimeTypeByDotDir = map[string]string{
	".claude":   RuntimeTypeClaudeCode,
	".cursor":   "cursor",
	".windsurf": "windsurf",
	".opencode": "opencode",
	".gemini":   "gemini",
	".codex":    "codex",
	".aider":    "aider",
	".continue": "continue",
}

// RuntimeTypeForDotDir returns the runtime type label for a project-relative
// file path based on its leading dot-folder, or "" if not recognized.
func RuntimeTypeForDotDir(relPath string) string {
	if i := strings.IndexByte(relPath, '/'); i >= 0 {
		return runtimeTypeByDotDir[relPath[:i]]
	}
	return runtimeTypeByDotDir[relPath]
}
