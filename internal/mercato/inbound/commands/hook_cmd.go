package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const mctHookMarker = "# mct-managed-hook"

var hookScripts = map[string]string{
	"post-merge": mctHookMarker + `:post-merge
mct restore
`,
	"pre-push": mctHookMarker + `:pre-push
mct save && git add .mct.json && git commit -m "mct: save state" --allow-empty
`,
}

func newHookCmd(_ Services, _ *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage git hooks",
	}

	cmd.AddCommand(
		newHookInstallCmd(),
		newHookUninstallCmd(),
	)

	return cmd
}

func newHookInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "install <post-pull|pre-push>",
		Short:     "Install a git hook",
		Long:      "Install a git hook. post-pull runs mct restore after pull. pre-push runs mct save + git add + git commit before push.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"post-pull", "pre-push"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			hookFile, snippet, err := resolveHook(name, gitHooksDir)
			if err != nil {
				return err
			}
			return installHookSnippet(cmd, hookFile, snippet, name)
		},
	}
}

func newHookUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "uninstall <post-pull|pre-push>",
		Short:     "Remove an mct-managed git hook snippet",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"post-pull", "pre-push"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			gitHookName := name
			if name == "post-pull" {
				gitHookName = "post-merge"
			}
			hooksDir, err := gitHooksDir()
			if err != nil {
				return err
			}
			return uninstallHookSnippet(cmd, hooksDir, gitHookName, name)
		},
	}
}

func resolveHook(name string, hooksDirFn func() (string, error)) (hookFile, snippet string, err error) {
	gitHookName := name
	if name == "post-pull" {
		gitHookName = "post-merge"
	}

	snippet, ok := hookScripts[gitHookName]
	if !ok {
		return "", "", fmt.Errorf("unknown hook %q (use post-pull or pre-push)", name)
	}

	hooksDir, err := hooksDirFn()
	if err != nil {
		return "", "", err
	}

	return filepath.Join(hooksDir, gitHookName), snippet, nil
}

func installHookSnippet(cmd *cobra.Command, hookFile, snippet, displayName string) error {
	marker := strings.SplitN(snippet, "\n", 2)[0]

	data, err := os.ReadFile(hookFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err == nil && strings.Contains(string(data), marker) {
		cmd.Printf("  hook %s is already installed\n", displayName)
		return nil
	}

	var content string
	if os.IsNotExist(err) {
		content = "#!/bin/sh\n" + snippet
	} else {
		content = strings.TrimRight(string(data), "\n") + "\n" + snippet
	}

	if err := os.MkdirAll(filepath.Dir(hookFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(hookFile, []byte(content), 0o755); err != nil {
		return err
	}

	cmd.Printf("  installed %s hook\n", displayName)
	return nil
}

func uninstallHookSnippet(cmd *cobra.Command, hooksDir, gitHookName, displayName string) error {
	hookFile := filepath.Join(hooksDir, gitHookName)
	marker := mctHookMarker + ":" + gitHookName

	data, err := os.ReadFile(hookFile)
	if err != nil {
		if os.IsNotExist(err) {
			cmd.Printf("  hook %s is not installed\n", displayName)
			return nil
		}
		return err
	}

	content := string(data)
	if !strings.Contains(content, marker) {
		cmd.Printf("  hook %s is not installed\n", displayName)
		return nil
	}

	cleaned := removeMarkedBlock(content, marker)
	cleaned = strings.TrimRight(cleaned, "\n") + "\n"

	trimmed := strings.TrimSpace(cleaned)
	if trimmed == "#!/bin/sh" || trimmed == "" {
		if err := os.Remove(hookFile); err != nil {
			return err
		}
		cmd.Printf("  removed %s hook\n", displayName)
		return nil
	}

	if err := os.WriteFile(hookFile, []byte(cleaned), 0o755); err != nil {
		return err
	}
	cmd.Printf("  removed mct snippet from %s hook\n", displayName)
	return nil
}

func gitHooksDir() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	return filepath.Join(strings.TrimSpace(string(out)), "hooks"), nil
}

// removeMarkedBlock removes the marker line and the line immediately following it.
func removeMarkedBlock(content, marker string) string {
	lines := strings.Split(content, "\n")
	var out []string
	skip := false
	for _, line := range lines {
		if line == marker {
			skip = true
			continue
		}
		if skip {
			skip = false
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
