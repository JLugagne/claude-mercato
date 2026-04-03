package commands

import (
	"github.com/spf13/cobra"

	"github.com/JLugagne/claude-mercato/internal/mercato/update"
)

func newDistUpgradeCmd(opts *GlobalOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "dist-upgrade",
		Short: "Update mct to the latest version",
		Long:  "Runs `go install github.com/JLugagne/claude-mercato/cmd/mct@latest` to install the latest version.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Updating mct to latest version...")
			if err := update.RunDistUpgrade(); err != nil {
				return err
			}
			// Reset the update check state so we don't nag after upgrading.
			update.CheckLatestVersion(opts.CacheDir, "")
			cmd.Println("mct updated successfully.")
			return nil
		},
	}
}
