package commands

import (
	"fmt"
	"strings"

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
  ssh_enabled      Enable/disable SSH for git operations (true/false)
  local_path       Local directory for installed entries
  conflict_policy  How to handle ref collisions (block/skip)
  drift_policy     How to handle local modifications (prompt/force/skip)
  tools.<name>     Enable/disable a tool target (true/false), e.g. tools.cursor true`,
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
	cmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Show configuration values",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			cfg, err := svc.Config.GetConfig()
			if err != nil {
				return err
			}

			if jsonOut {
				return printJSON(cmd.OutOrStdout(), cfg)
			}

			if len(args) == 1 {
				key := args[0]
				switch key {
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
				case "tools":
					if len(cfg.Tools) == 0 {
						cmd.Println("  (none)")
					} else {
						for tool, enabled := range cfg.Tools {
							cmd.Printf("  %s: %v\n", tool, enabled)
						}
					}
				default:
					// Check for tools.<name> dotted key
					if strings.HasPrefix(key, "tools.") {
						toolName := strings.TrimPrefix(key, "tools.")
						if cfg.Tools != nil {
							cmd.Printf("%v\n", cfg.Tools[toolName])
						} else {
							cmd.Println("false")
						}
					} else {
						return fmt.Errorf("unknown config key: %s", key)
					}
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
			if len(cfg.Tools) > 0 {
				cmd.Printf("  tools:\n")
				for tool, enabled := range cfg.Tools {
					cmd.Printf("    %s: %v\n", tool, enabled)
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}
