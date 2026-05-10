package app

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
)

// removeHookSnippet splices out every hook object whose mct_id matches the
// recorded id for ref from .claude/settings.json. Empty event arrays and a
// fully-empty hooks key are pruned. Sibling top-level keys are left alone.
//
// Tolerant: missing settings.json or missing entries are not errors — the
// install record is the source of truth for "this ref was installed".
func (a *App) removeHookSnippet(w txWriter, ref domain.MctRef, settingsAbsPath string) error {
	mctID := mctIDForRef(ref)

	existing, err := os.ReadFile(settingsAbsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	doc, err := readSettings(existing)
	if err != nil {
		return err
	}
	hooks, err := doc.hooksByEvent()
	if err != nil {
		return err
	}

	changed := false
	for event, list := range hooks {
		filtered := make([]json.RawMessage, 0, len(list))
		for _, obj := range list {
			if extractMctID(obj) == mctID {
				changed = true
				continue
			}
			filtered = append(filtered, obj)
		}
		hooks[event] = filtered
	}

	if !changed {
		return nil
	}

	if err := doc.setHooksByEvent(hooks); err != nil {
		return err
	}
	out, err := doc.marshal()
	if err != nil {
		return err
	}
	return w.WriteFile(settingsAbsPath, out)
}

// dropHookFromPackage removes hookFile from pkg.Files.Hooks and from the
// matching location's Files list (the entry whose Path ends in
// "#hooks/<hookFile>"). If the package's Hooks slice becomes empty AND
// there are no other files in the package at this location, the location
// itself is removed.
func (a *App) dropHookFromPackage(db *domain.InstallDatabase, market, profile, location, hookFile string) error {
	pkg := db.FindPackage(market, profile)
	if pkg == nil {
		return nil
	}
	// Remove the hook leaf from package-wide list.
	filtered := pkg.Files.Hooks[:0]
	for _, h := range pkg.Files.Hooks {
		if h != hookFile {
			filtered = append(filtered, h)
		}
	}
	pkg.Files.Hooks = filtered

	// Remove the per-location InstalledFile entry whose Path encodes this
	// hookFile. Other hooks in the same package keep their entries.
	loc := pkg.FindLocation(location)
	if loc != nil {
		suffix := "#hooks/" + hookFile
		filteredFiles := loc.Files[:0]
		for _, f := range loc.Files {
			if len(f.Path) >= len(suffix) && f.Path[len(f.Path)-len(suffix):] == suffix {
				continue
			}
			filteredFiles = append(filteredFiles, f)
		}
		loc.Files = filteredFiles
		// If the package has no remaining files at this location AND no
		// remaining package-level files of any kind, drop the location
		// entirely so List/Check don't keep returning it.
		if len(loc.Files) == 0 && len(pkg.Files.Skills) == 0 && len(pkg.Files.Agents) == 0 && len(pkg.Files.Commands) == 0 && len(pkg.Files.Hooks) == 0 {
			db.RemoveLocation(market, profile, location)
		}
	}
	return nil
}
