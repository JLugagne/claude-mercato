package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatalf("git init: %v", err)
	}
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHookInstall_PostPull(t *testing.T) {
	dir := initGitRepo(t)
	svc := mockServices()

	out, err := runCmd(t, svc, "hook", "install", "post-pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "installed post-pull hook") {
		t.Errorf("unexpected output: %s", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".git", "hooks", "post-merge"))
	if err != nil {
		t.Fatalf("hook file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "#!/bin/sh") {
		t.Error("missing shebang")
	}
	if !strings.Contains(content, "mct restore") {
		t.Error("missing mct restore")
	}
}

func TestHookInstall_PrePush(t *testing.T) {
	dir := initGitRepo(t)
	svc := mockServices()

	out, err := runCmd(t, svc, "hook", "install", "pre-push")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "installed pre-push hook") {
		t.Errorf("unexpected output: %s", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".git", "hooks", "pre-push"))
	if err != nil {
		t.Fatalf("hook file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "mct save") {
		t.Error("missing mct save")
	}
	if !strings.Contains(content, "git add .mct.json") {
		t.Error("missing git add")
	}
}

func TestHookInstall_AlreadyInstalled(t *testing.T) {
	initGitRepo(t)
	svc := mockServices()

	_, _ = runCmd(t, svc, "hook", "install", "post-pull")
	out, err := runCmd(t, svc, "hook", "install", "post-pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "already installed") {
		t.Errorf("expected already installed message, got: %s", out)
	}
}

func TestHookInstall_AppendsToExisting(t *testing.T) {
	dir := initGitRepo(t)
	svc := mockServices()

	hookFile := filepath.Join(dir, ".git", "hooks", "post-merge")
	_ = os.MkdirAll(filepath.Dir(hookFile), 0o755)
	_ = os.WriteFile(hookFile, []byte("#!/bin/sh\necho existing\n"), 0o755)

	_, err := runCmd(t, svc, "hook", "install", "post-pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(hookFile)
	content := string(data)
	if !strings.Contains(content, "echo existing") {
		t.Error("lost existing content")
	}
	if !strings.Contains(content, "mct restore") {
		t.Error("missing mct restore")
	}
}

func TestHookInstall_InvalidName(t *testing.T) {
	initGitRepo(t)
	svc := mockServices()

	_, err := runCmd(t, svc, "hook", "install", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid hook name")
	}
}

func TestHookUninstall_PostPull(t *testing.T) {
	initGitRepo(t)
	svc := mockServices()

	_, _ = runCmd(t, svc, "hook", "install", "post-pull")
	out, err := runCmd(t, svc, "hook", "uninstall", "post-pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "removed post-pull hook") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestHookUninstall_NotInstalled(t *testing.T) {
	initGitRepo(t)
	svc := mockServices()

	out, err := runCmd(t, svc, "hook", "uninstall", "post-pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "not installed") {
		t.Errorf("expected not installed message, got: %s", out)
	}
}

func TestHookUninstall_PreservesOtherContent(t *testing.T) {
	dir := initGitRepo(t)
	svc := mockServices()

	hookFile := filepath.Join(dir, ".git", "hooks", "post-merge")
	_ = os.MkdirAll(filepath.Dir(hookFile), 0o755)
	_ = os.WriteFile(hookFile, []byte("#!/bin/sh\necho existing\n"), 0o755)

	_, _ = runCmd(t, svc, "hook", "install", "post-pull")
	out, err := runCmd(t, svc, "hook", "uninstall", "post-pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "removed mct snippet") {
		t.Errorf("unexpected output: %s", out)
	}

	data, _ := os.ReadFile(hookFile)
	content := string(data)
	if !strings.Contains(content, "echo existing") {
		t.Error("lost existing content")
	}
	if strings.Contains(content, "mct restore") {
		t.Error("mct snippet not removed")
	}
}

func TestRemoveMarkedBlock(t *testing.T) {
	input := "#!/bin/sh\necho before\n# mct-managed-hook:post-merge\nmct restore\necho after\n"
	got := removeMarkedBlock(input, "# mct-managed-hook:post-merge")
	if strings.Contains(got, "mct restore") {
		t.Error("block not removed")
	}
	if !strings.Contains(got, "echo before") || !strings.Contains(got, "echo after") {
		t.Error("surrounding content lost")
	}
}
