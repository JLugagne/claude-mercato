package commands

import (
	"github.com/spf13/cobra"

	"github.com/JLugagne/agents-mercato/internal/mercato/update"
)

func newUpgradeCmd(opts *GlobalOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Update mct to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := update.RunUpgrade(Version); err != nil {
				return err
			}
			update.CheckLatestVersion(opts.CacheDir, Version)
			return nil
		},
	}
}

func newDistUpgradeCmd(opts *GlobalOpts) *cobra.Command {
	cmd := newUpgradeCmd(opts)
	cmd.Use = "dist-upgrade"
	cmd.Short = "Update mct to the latest release (alias for upgrade)"
	cmd.Hidden = true
	return cmd
}
