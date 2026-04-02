package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

func newMarketCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Manage markets",
	}

	cmd.AddCommand(
		newMarketAddCmd(svc, opts),
		newMarketRemoveCmd(svc, opts),
		newMarketListCmd(svc, opts),
		newMarketInfoCmd(svc, opts),
		newMarketSetCmd(svc, opts),
	)

	return cmd
}

func newMarketAddCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <url> [skills-path]",
		Short: "Register a new market",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch, _ := cmd.Flags().GetString("branch")
			trusted, _ := cmd.Flags().GetBool("trusted")
			readOnly, _ := cmd.Flags().GetBool("read-only")
			noClone, _ := cmd.Flags().GetBool("no-clone")
			jsonOut, _ := cmd.Flags().GetBool("json")
			var skillsPath string
			if len(args) > 1 {
				skillsPath = args[1]
			}
			url := args[0]
			if !strings.Contains(url, "://") {
				url = "https://" + url
			}
			result, err := svc.Markets.AddMarket(url, service.AddMarketOpts{
				Branch:     branch,
				Trusted:    trusted,
				ReadOnly:   readOnly,
				NoClone:    noClone,
				SkillsPath: skillsPath,
			})
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(cmd.OutOrStdout(), map[string]any{
					"url":      url,
					"profiles": result.Profiles,
					"agents":   result.Agents,
					"skills":   result.Skills,
				})
			}
			cmd.Printf("  %d profiles, %d agents, %d skills\n", result.Profiles, result.Agents, result.Skills)
			return nil
		},
	}
	cmd.Flags().String("branch", "main", "branch to track")
	cmd.Flags().Bool("trusted", false, "skip breaking change confirmation")
	cmd.Flags().Bool("read-only", false, "index only, never install")
	cmd.Flags().Bool("no-clone", false, "register without cloning")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func newMarketRemoveCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Unregister a market",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			keepCache, _ := cmd.Flags().GetBool("keep-cache")
			return svc.Markets.RemoveMarket(args[0], service.RemoveMarketOpts{
				Force:     force,
				KeepCache: keepCache,
			})
		},
	}
	cmd.Flags().Bool("force", false, "skip installed entries check")
	cmd.Flags().Bool("keep-cache", false, "keep clone directory")
	return cmd
}

func newMarketListCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured markets",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			markets, err := svc.Markets.ListMarkets()
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(cmd.OutOrStdout(), markets)
			}
			for _, m := range markets {
				status := "●"
				if m.ReadOnly {
					status = "○"
				}
				cmd.Printf("  %s  %-20s  %s  (%s)\n", status, m.Name, m.URL, m.Branch)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func newMarketInfoCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show market details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			info, err := svc.Markets.MarketInfo(args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(cmd.OutOrStdout(), info)
			}
			cmd.Printf("  Name:       %s\n", info.Market.Name)
			cmd.Printf("  URL:        %s\n", info.Market.URL)
			cmd.Printf("  Branch:     %s\n", info.Market.Branch)
			cmd.Printf("  Trusted:    %v\n", info.Market.Trusted)
			cmd.Printf("  ReadOnly:   %v\n", info.Market.ReadOnly)
			cmd.Printf("  SkillsOnly: %v\n", info.Market.SkillsOnly)
			if info.Market.SkillsPath != "" {
				cmd.Printf("  SkillsPath: %s\n", info.Market.SkillsPath)
			}
			cmd.Printf("  Entries:   %d\n", info.EntryCount)
			cmd.Printf("  Installed: %d\n", info.InstalledCount)
			cmd.Printf("  Status:    %s\n", info.Status)
			if !info.LastSynced.IsZero() {
				cmd.Printf("  Synced:    %s\n", info.LastSynced.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func newMarketSetCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "set <name> <key> <value>",
		Short: "Update a market property",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.Markets.SetMarketProperty(args[0], args[1], args[2]); err != nil {
				return err
			}
			cmd.Printf("  ✓  %s.%s = %s\n", args[0], args[1], args[2])
			return nil
		},
	}
}
