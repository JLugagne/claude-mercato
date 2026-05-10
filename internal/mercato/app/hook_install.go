package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

// installHookSnippet merges a parsed hook snippet into the project's
// .claude/settings.json under settings.json["hooks"][snippet.Event].
// It injects mct_id into each hook object and records a single InstalledFile
// (Path = settingsAbsPath, XXH = body checksum) for drift detection.
//
// Returns ErrConflictHookEventMatcher when the existing settings.json
// already contains a hook with the same (event, matcher) carrying a
// different mct_id (i.e. installed by a different mct ref).
// installHookSnippet merges a parsed hook snippet into the project's
// .claude/settings.json under settings.json["hooks"][snippet.Event].
// It injects mct_id into each hook object and records a single InstalledFile
// for drift detection. The recorded Path uses a "<settings.json>#hooks/<file>"
// suffix so multiple hooks per package keep distinct per-hook checksums.
//
// Returns ErrConflictHookEventMatcher when the existing settings.json
// already contains a hook with the same (event, matcher) carrying a
// different mct_id (i.e. installed by a different mct ref).
func (a *App) installHookSnippet(
	w txWriter,
	ref domain.MctRef,
	snippet domain.HookSnippet,
	settingsAbsPath string,
	hookFile string,
) ([]domain.InstalledFile, error) {
	mctID := mctIDForRef(ref)

	existing, err := os.ReadFile(settingsAbsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	doc, err := readSettings(existing)
	if err != nil {
		return nil, err
	}

	hooks, err := doc.hooksByEvent()
	if err != nil {
		return nil, err
	}

	if conflictExists(hooks, snippet.Event, snippet.Matcher, mctID) {
		return nil, domain.ErrConflictHookEventMatcher
	}

	injected, err := injectMctID(snippet, mctID)
	if err != nil {
		return nil, err
	}

	hooks[snippet.Event] = append(hooks[snippet.Event], injected...)
	if err := doc.setHooksByEvent(hooks); err != nil {
		return nil, err
	}

	out, err := doc.marshal()
	if err != nil {
		return nil, err
	}
	if err := w.WriteFile(settingsAbsPath, out); err != nil {
		return nil, err
	}

	checksum, err := combinedBodyChecksum(injected)
	if err != nil {
		return nil, err
	}
	written := []domain.InstalledFile{
		{Path: relativeProjectPath(settingsAbsPath) + "#hooks/" + hookFile, XXH: checksum},
	}
	return written, nil
}

// conflictExists reports whether the given (event, matcher) pair is already
// claimed by a hook with a DIFFERENT mct_id in the current settings.json.
// Same mct_id means a re-install of the same ref, which is handled
// elsewhere (re-install of an already-installed entry returns
// ErrEntryAlreadyInstalled before reaching this code path).
func conflictExists(hooks map[string][]json.RawMessage, event, matcher, mctID string) bool {
	for _, obj := range hooks[event] {
		if extractMatcher(obj) != matcher {
			continue
		}
		other := extractMctID(obj)
		if other == "" || other == mctID {
			continue
		}
		return true
	}
	return false
}

// combinedBodyChecksum returns a stable hash over multiple hook bodies.
// The individual canonical bytes are concatenated in order before hashing
// so re-installing the same snippet yields the same checksum.
func combinedBodyChecksum(bodies []json.RawMessage) (string, error) {
	var concat []byte
	for _, b := range bodies {
		c, err := canonicalHookBody(b)
		if err != nil {
			return "", err
		}
		concat = append(concat, c...)
	}
	return xxhashHex(concat), nil
}

// relativeProjectPath turns an absolute settings.json path into the
// project-relative slash-separated form recorded in InstalledFile.Path
// (e.g. ".claude/settings.json").
func relativeProjectPath(absSettingsPath string) string {
	return projectRelFromClaude(absSettingsPath)
}

// addHook is the dedicated install path for hook entries. It parses the
// JSON snippet, opens a transaction, runs the settings.json merge through
// installHookSnippet, and records a single per-package leaf-name entry.
// addHook is the dedicated install path for hook entries. It parses the
// JSON snippet, opens a transaction, runs the settings.json merge through
// installHookSnippet, and records a single per-package leaf-name entry.
func (a *App) addHook(
	ref domain.MctRef,
	marketName, relPath, profile string,
	cfg domain.Config,
	mc *domain.MarketConfig,
	db *domain.InstallDatabase,
	projectPath string,
	opts service.AddOpts,
	result *service.AddResult,
) error {
	pkg := db.FindPackage(marketName, profile)
	if pkg != nil && pkg.FindLocation(projectPath) != nil {
		hookFile := filepath.Base(relPath)
		for _, h := range pkg.Files.Hooks {
			if h == hookFile {
				return domain.ErrEntryAlreadyInstalled
			}
		}
	}

	clonePath := a.clonePath(marketName)
	content, err := a.git.ReadFileAtRef(clonePath, mc.Branch, relPath, "HEAD")
	if err != nil {
		return err
	}

	snippet, err := domain.ParseHookSnippet(content)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	settingsAbsPath, err := a.resolveLocalPath(cfg, relPath)
	if err != nil {
		return err
	}
	hookFile := filepath.Base(relPath)

	w, commit, rollback, err := a.beginWriter("add-hook:" + string(ref))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = rollback()
		}
	}()

	written, err := a.installHookSnippet(w, ref, snippet, settingsAbsPath, hookFile)
	if err != nil {
		return err
	}

	headSHA, err := a.git.RemoteHEAD(clonePath, mc.Branch)
	if err != nil {
		return err
	}

	files := domain.InstalledFiles{Hooks: []string{hookFile}}
	existingPkg := db.FindPackage(marketName, profile)
	mergedPkgFiles := files
	if existingPkg != nil {
		mergedPkgFiles = domain.MergePackageFiles(existingPkg.Files, files)
	}

	mergedClaudeFiles := written
	if existingPkg != nil {
		if loc := findLocationByPathAndType(existingPkg, projectPath, domain.RuntimeTypeClaudeCode); loc != nil {
			mergedClaudeFiles = domain.MergeLocationFiles(loc.Files, written)
		}
	}

	db.AddOrUpdatePackage(marketName, profile, headSHA, mergedPkgFiles, domain.InstalledLocation{
		Path:  projectPath,
		Type:  domain.RuntimeTypeClaudeCode,
		Files: mergedClaudeFiles,
	})

	if err := a.stageDBSave(w, *db); err != nil {
		return err
	}
	if err := commit(); err != nil {
		return err
	}
	committed = true

	if result != nil {
		if result.ToolWrites == nil {
			result.ToolWrites = make(map[string]string)
		}
		result.ToolWrites[domain.RuntimeTypeClaudeCode] = relativeProjectPath(settingsAbsPath)
	}

	return nil
}
