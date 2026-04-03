package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldCheck_NoPriorState(t *testing.T) {
	dir := t.TempDir()
	if !ShouldCheck(dir) {
		t.Error("expected ShouldCheck=true with no prior state")
	}
}

func TestShouldCheck_RecentCheck(t *testing.T) {
	dir := t.TempDir()
	saveState(dir, State{LastCheck: time.Now()})
	if ShouldCheck(dir) {
		t.Error("expected ShouldCheck=false right after a check")
	}
}

func TestShouldCheck_StaleCheck(t *testing.T) {
	dir := t.TempDir()
	saveState(dir, State{LastCheck: time.Now().Add(-25 * time.Hour)})
	if !ShouldCheck(dir) {
		t.Error("expected ShouldCheck=true after 25 hours")
	}
}

func TestCachedResult_NoUpdate(t *testing.T) {
	dir := t.TempDir()
	saveState(dir, State{LastCheck: time.Now(), LatestVersion: "v1.3.1"})
	r := CachedResult(dir, "v1.3.1")
	if r.UpdateAvailable {
		t.Error("expected no update when versions match")
	}
}

func TestCachedResult_UpdateAvailable(t *testing.T) {
	dir := t.TempDir()
	saveState(dir, State{LastCheck: time.Now(), LatestVersion: "v1.4.0"})
	r := CachedResult(dir, "v1.3.1")
	if !r.UpdateAvailable {
		t.Error("expected update available when versions differ")
	}
	if r.LatestVersion != "v1.4.0" {
		t.Errorf("expected LatestVersion=v1.4.0, got %s", r.LatestVersion)
	}
}

func TestSaveLoadState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second)
	saveState(dir, State{LastCheck: now, LatestVersion: "v2.0.0"})

	s := loadState(dir)
	if !s.LastCheck.Truncate(time.Second).Equal(now) {
		t.Errorf("expected LastCheck=%v, got %v", now, s.LastCheck)
	}
	if s.LatestVersion != "v2.0.0" {
		t.Errorf("expected LatestVersion=v2.0.0, got %s", s.LatestVersion)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, stateFile)); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}

func TestFormatUpdateNotice(t *testing.T) {
	r := Result{CurrentVersion: "v1.3.1", LatestVersion: "v1.4.0", UpdateAvailable: true}
	notice := FormatUpdateNotice(r)
	if notice == "" {
		t.Error("expected non-empty notice")
	}
}
