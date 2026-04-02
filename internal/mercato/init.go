package mercato

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JLugagne/claude-mercato/internal/mercato/app"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
	"github.com/JLugagne/claude-mercato/internal/mercato/inbound/commands"
	"github.com/JLugagne/claude-mercato/internal/mercato/inbound/queries/tui"
	"github.com/JLugagne/claude-mercato/internal/mercato/outbound/cfgadapter"
	"github.com/JLugagne/claude-mercato/internal/mercato/outbound/fsadapter"
	"github.com/JLugagne/claude-mercato/internal/mercato/outbound/gitadapter"
)

func resolveSSHEnabled(configPath string) bool {
	// Environment variable takes precedence
	if env := os.Getenv("MCT_SSH_ENABLED"); env != "" {
		return strings.EqualFold(env, "true") || env == "1"
	}
	// Check config file
	cfgStore := cfgadapter.NewConfigStore()
	cfg, err := cfgStore.Load(configPath)
	if err != nil {
		return false // default: disabled
	}
	if cfg.SSHEnabled != nil {
		return *cfg.SSHEnabled
	}
	return false // default: disabled
}

func NewApp(configPath, cacheDir string) *cobra.Command {
	sshEnabled := resolveSSHEnabled(configPath)
	gitRepo := gitadapter.New(gitadapter.WithSSHEnabled(sshEnabled))
	fs := fsadapter.New()
	cfgStore := cfgadapter.NewConfigStore()
	stateStore := cfgadapter.NewStateStore()

	installDB := cfgadapter.NewInstallDB()
	application := app.New(gitRepo, fs, cfgStore, stateStore, installDB, configPath, cacheDir)

	svc := commands.Services{
		Markets: application,
		Sync:    application,
		Entries: application,
		Search:  application,
		Readmes: application,
		Config:  application,
	}
	rootCmd := commands.NewRootCmd(svc)

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "init" || cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}
		if !cfgStore.Exists(configPath) {
			fmt.Fprintln(cmd.ErrOrStderr(), "First run detected — initializing mct…")
			return application.Init(service.InitOpts{LocalPath: ".claude/"})
		}
		return nil
	}

	rootCmd.AddCommand(newTUICmd(application))

	return rootCmd
}

func newTUICmd(application *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunTUI(tui.TUIServices{
				Markets: application,
				Sync:    application,
				Entries: application,
				Search:  application,
			})
		},
	}
}
