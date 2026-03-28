package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newConfigCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and update configuration",
	}

	cmd.AddCommand(
		newConfigSetCmd(svc),
		newConfigGetCmd(svc),
	)

	return cmd
}

func newConfigSetCmd(svc Services) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value. Available keys:
  ssh_enabled    Enable/disable SSH for git operations (true/false)
  local_path     Local directory for installed entries
  conflict_policy  How to handle ref collisions (block/skip)
  drift_policy     How to handle local modifications (prompt/force/skip)
  difftool         External diff tool command`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.Config.SetConfigField(args[0], args[1]); err != nil {
				return err
			}
			cmd.Printf("  ok  %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigGetCmd(svc Services) *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Show configuration values",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := svc.Config.GetConfig()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				switch args[0] {
				case "ssh_enabled":
					v := false
					if cfg.SSHEnabled != nil {
						v = *cfg.SSHEnabled
					}
					cmd.Printf("%v\n", v)
				case "local_path":
					cmd.Println(cfg.LocalPath)
				case "conflict_policy":
					cmd.Println(cfg.ConflictPolicy)
				case "drift_policy":
					cmd.Println(cfg.DriftPolicy)
				case "difftool":
					cmd.Println(cfg.Difftool)
				default:
					return fmt.Errorf("unknown config key: %s", args[0])
				}
				return nil
			}

			sshEnabled := false
			if cfg.SSHEnabled != nil {
				sshEnabled = *cfg.SSHEnabled
			}
			cmd.Printf("  ssh_enabled:      %v\n", sshEnabled)
			cmd.Printf("  local_path:       %s\n", cfg.LocalPath)
			cmd.Printf("  conflict_policy:  %s\n", cfg.ConflictPolicy)
			cmd.Printf("  drift_policy:     %s\n", cfg.DriftPolicy)
			if cfg.Difftool != "" {
				cmd.Printf("  difftool:         %s\n", cfg.Difftool)
			}
			return nil
		},
	}
}
