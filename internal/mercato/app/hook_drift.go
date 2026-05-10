package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// detectHookDrift compares each installed hook in pkg.Files.Hooks against
// the live .claude/settings.json. Returns a list of hook leaf names that
// have drifted, are missing, or whose body checksum no longer matches the
// recorded value.
//
// The returned slice contains synthetic relative paths of the form
// "hooks/<name>" for parity with skill/agent drift output (consumers
// rendering drift lists treat them as opaque labels).
func (a *App) detectHookDrift(market string, pkg domain.InstalledPackage, location string) []string {
	if len(pkg.Files.Hooks) == 0 {
		return nil
	}
	settingsPath := filepath.Join(location, ".claude", "settings.json")

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			out := make([]string, 0, len(pkg.Files.Hooks))
			for _, h := range pkg.Files.Hooks {
				out = append(out, "hooks/"+h)
			}
			return out
		}
		// Any other read error: report all hooks as drifted to be safe;
		// the user will see the message and can investigate.
		out := make([]string, 0, len(pkg.Files.Hooks))
		for _, h := range pkg.Files.Hooks {
			out = append(out, "hooks/"+h)
		}
		return out
	}

	doc, err := readSettings(content)
	if err != nil {
		out := make([]string, 0, len(pkg.Files.Hooks))
		for _, h := range pkg.Files.Hooks {
			out = append(out, "hooks/"+h)
		}
		return out
	}
	hooks, err := doc.hooksByEvent()
	if err != nil {
		return nil // malformed but unrecoverable; surfaced elsewhere
	}

	var drifted []string
	loc := pkg.FindLocation(location)
	for _, hookFile := range pkg.Files.Hooks {
		ref := domain.MctRef(market + "@" + hookFileRepoPathStandalone(pkg.Profile, hookFile))
		mctID := mctIDForRef(ref)
		// Find every hook object across all events that carries this
		// mct_id. There should be at least one. Compare checksum of the
		// concatenated canonical bodies against the recorded value.
		matches := matchesByID(hooks, mctID)
		if len(matches) == 0 {
			drifted = append(drifted, "hooks/"+hookFile)
			continue
		}
		recorded := lookupHookChecksum(loc, hookFile)
		current, err := combinedBodyChecksum(matches)
		if err != nil || current != recorded {
			drifted = append(drifted, "hooks/"+hookFile)
		}
	}
	return drifted
}

func matchesByID(hooks map[string][]json.RawMessage, mctID string) []json.RawMessage {
	var out []json.RawMessage
	for _, list := range hooks {
		for _, obj := range list {
			if extractMctID(obj) == mctID {
				out = append(out, obj)
			}
		}
	}
	return out
}

// lookupHookChecksum reads the recorded body checksum for hookFile from the
// per-location InstalledFile list. The hook installer records exactly one
// InstalledFile per hook, so we match by Path == ".claude/settings.json"
// AND by hookFile encoded in a "hooks/<file>" suffix on the Path. To keep
// schema additive we don't extend InstalledFile; instead the install path
// records each hook's own checksum and we match by the slot-of-installation
// order, which is fragile across upgrades — TODO when we revisit hook
// drift detection.
//
// Pragmatic shortcut: when there is exactly one installed hook in this
// location, return the only Path-matching XXH. When there are several, we
// compute a fresh checksum over the combined bodies and treat any single
// per-file mismatch as drift on every hook in the package — coarser, but
// safe (and the most common case is one hook per package).
// lookupHookChecksum returns the per-hook body checksum recorded by
// installHookSnippet. The InstalledFile.Path is encoded as
// "<settings.json>#hooks/<file>" so multiple hooks in one package retain
// distinct per-hook checksums.
func lookupHookChecksum(loc *domain.InstalledLocation, hookFile string) string {
	if loc == nil {
		return ""
	}
	suffix := "#hooks/" + hookFile
	for _, f := range loc.Files {
		if len(f.Path) >= len(suffix) && f.Path[len(f.Path)-len(suffix):] == suffix {
			return f.XXH
		}
	}
	return ""
}
