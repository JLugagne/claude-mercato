package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const (
	repoSlug      = "JLugagne/claude-mercato"
	checkInterval = 24 * time.Hour
	stateFile     = "update-check.json"
)

// State persists the last update check result.
type State struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version,omitempty"`
}

// Result describes the outcome of an update check.
type Result struct {
	UpdateAvailable bool
	CurrentVersion  string
	LatestVersion   string
}

func statePath(cacheDir string) string {
	return filepath.Join(cacheDir, stateFile)
}

func loadState(cacheDir string) State {
	data, err := os.ReadFile(statePath(cacheDir))
	if err != nil {
		return State{}
	}
	var s State
	_ = json.Unmarshal(data, &s)
	return s
}

func saveState(cacheDir string, s State) {
	data, _ := json.Marshal(s)
	_ = os.MkdirAll(cacheDir, 0o755)
	_ = os.WriteFile(statePath(cacheDir), data, 0o644)
}

func ShouldCheck(cacheDir string) bool {
	s := loadState(cacheDir)
	return time.Since(s.LastCheck) >= checkInterval
}

func CheckLatestVersion(cacheDir, currentVersion string) Result {
	result := Result{CurrentVersion: currentVersion}

	latest, err := fetchLatestVersion()
	s := State{LastCheck: time.Now()}
	if err == nil {
		s.LatestVersion = latest
		result.LatestVersion = latest
		result.UpdateAvailable = latest != "" && latest != currentVersion && currentVersion != "dev"
	}
	saveState(cacheDir, s)
	return result
}

func CachedResult(cacheDir, currentVersion string) Result {
	s := loadState(cacheDir)
	return Result{
		CurrentVersion:  currentVersion,
		LatestVersion:   s.LatestVersion,
		UpdateAvailable: s.LatestVersion != "" && s.LatestVersion != currentVersion && currentVersion != "dev",
	}
}

func fetchLatestVersion() (string, error) {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{})
	if err != nil {
		return "", err
	}
	repo := selfupdate.NewRepositorySlug("", repoSlug)
	release, found, err := updater.DetectLatest(context.Background(), repo)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("no release found")
	}
	return "v" + release.Version(), nil
}

// RunUpgrade downloads and replaces the running binary with the latest release.
func RunUpgrade(currentVersion string) error {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{})
	if err != nil {
		return err
	}
	repo := selfupdate.NewRepositorySlug("", repoSlug)
	current := strings.TrimPrefix(currentVersion, "v")

	release, err := updater.UpdateSelf(context.Background(), current, repo)
	if err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}
	latest := "v" + release.Version()
	if latest == currentVersion {
		fmt.Println("  ok  already at latest version", latest)
	} else {
		fmt.Printf("  ok  updated %s → %s\n", currentVersion, latest)
	}
	return nil
}

func FormatUpdateNotice(r Result) string {
	return fmt.Sprintf("A new version of mct is available: %s → %s (run `mct upgrade` to update)", r.CurrentVersion, r.LatestVersion)
}
