// Package update provides self-update checking and installation for mct.
package update

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	modulePath    = "github.com/JLugagne/claude-mercato/cmd/mct"
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

// ShouldCheck returns true if enough time has passed since the last check.
func ShouldCheck(cacheDir string) bool {
	s := loadState(cacheDir)
	return time.Since(s.LastCheck) >= checkInterval
}

// CheckLatestVersion queries the Go module proxy for the latest version.
// It updates the state file regardless of the outcome.
func CheckLatestVersion(cacheDir, currentVersion string) Result {
	result := Result{CurrentVersion: currentVersion}

	latest, err := fetchLatestVersion()
	s := State{LastCheck: time.Now()}
	if err == nil {
		s.LatestVersion = latest
		result.LatestVersion = latest
		result.UpdateAvailable = latest != "" && latest != currentVersion
	}
	saveState(cacheDir, s)
	return result
}

// CachedResult returns the last known update check result without hitting the network.
func CachedResult(cacheDir, currentVersion string) Result {
	s := loadState(cacheDir)
	return Result{
		CurrentVersion:  currentVersion,
		LatestVersion:   s.LatestVersion,
		UpdateAvailable: s.LatestVersion != "" && s.LatestVersion != currentVersion,
	}
}

// fetchLatestVersion uses `go list -m` to query the latest module version.
func fetchLatestVersion() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", modulePath+"@latest")
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunDistUpgrade runs `go install` to update mct to the latest version.
func RunDistUpgrade() error {
	cmd := exec.Command("go", "install", modulePath+"@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// FormatUpdateNotice returns a human-readable update message.
func FormatUpdateNotice(r Result) string {
	return fmt.Sprintf("A new version of mct is available: %s → %s (run `mct dist-upgrade` to update)", r.CurrentVersion, r.LatestVersion)
}
