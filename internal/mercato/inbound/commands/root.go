package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

const version = "0.1.0"

// GlobalOpts holds flags shared across all commands
type GlobalOpts struct {
	ConfigPath string
	CacheDir   string
	Offline    bool
	Verbose    bool
	Quiet      bool
	NoColor    bool
	CI         bool
}

// Services groups all service interfaces for command handlers
type Services struct {
	Markets service.MarketCommands
	Sync    service.SyncCommands
	Entries service.EntryCommands
	Search  service.SearchQueries
	Readmes service.ReadmeQueries
	Config  service.ConfigCommands
}

// NewRootCmd builds the root cobra command with all subcommands
func NewRootCmd(svc Services) *cobra.Command {
	opts := &GlobalOpts{}

	root := &cobra.Command{
		Use:           "mct",
		Short:         "Claude agent and skill market manager",
		Long:          "claude-mercato — manage Claude agent and skill definitions across Git-based markets",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	// Global flags
	root.PersistentFlags().StringVar(&opts.ConfigPath, "config", "~/.config/mct/config.yml", "path to config file")
	root.PersistentFlags().StringVar(&opts.CacheDir, "cache", "~/.cache/mct", "cache directory")
	root.PersistentFlags().BoolVar(&opts.Offline, "offline", false, "disable network operations")
	root.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "detailed operation log")
	root.PersistentFlags().BoolVar(&opts.Quiet, "quiet", false, "suppress all output except errors")
	root.PersistentFlags().BoolVar(&opts.NoColor, "no-color", false, "disable ANSI colours")
	root.PersistentFlags().BoolVar(&opts.CI, "ci", false, "non-interactive mode")

	// Register subcommands
	root.AddCommand(
		newMarketCmd(svc, opts),
		newRefreshCmd(svc, opts),
		newUpdateCmd(svc, opts),
		newSyncCmd(svc, opts),
		newCheckCmd(svc, opts),
		newAddCmd(svc, opts),
		newRemoveCmd(svc, opts),
		newPruneCmd(svc, opts),
		newPinCmd(svc, opts),
		newDiffCmd(svc, opts),
		newSearchCmd(svc, opts),
		newListCmd(svc, opts),
		newConflictsCmd(svc, opts),
		newSyncStateCmd(svc, opts),
		newIndexCmd(svc, opts),
		newInitCmd(svc, opts),
		newReadmeCmd(svc, opts),
		newConfigCmd(svc, opts),
		newExportCmd(svc, opts),
		newImportCmd(svc, opts),
		newLintCmd(svc, opts),
	)

	// Aliases
	root.AddCommand(newStatusCmd(svc, opts))  // alias for check
	root.AddCommand(newInstallCmd(svc, opts)) // alias for add
	root.AddCommand(newMarketsCmd(svc, opts)) // alias for market list
	root.AddCommand(newSaveCmd(svc, opts))    // alias for export .mct.json
	root.AddCommand(newRestoreCmd(svc, opts)) // alias for import .mct.json

	return root
}

// Execute runs the root command
func Execute(svc Services) {
	root := NewRootCmd(svc)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}
