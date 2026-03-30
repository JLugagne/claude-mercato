package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLintCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "lint [dir]",
		Short: "Check a local directory as a market",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			result, err := svc.Markets.LintMarket(dir)
			if err != nil {
				return err
			}

			cmd.Printf("  profiles: %d  agents: %d  skills: %d\n\n", result.Profiles, result.Agents, result.Skills)

			if len(result.Issues) == 0 {
				cmd.Println("  ok  no issues found")
				return nil
			}

			errors, warns := 0, 0
			for _, issue := range result.Issues {
				switch issue.Severity {
				case "error":
					cmd.Printf("  x  [%s] %s\n", issue.Profile, issue.Message)
					errors++
				case "warn":
					cmd.Printf("  ~  [%s] %s\n", issue.Profile, issue.Message)
					warns++
				}
			}

			cmd.Println()
			cmd.Printf("  %d error(s)  %d warning(s)\n", errors, warns)

			if errors > 0 {
				return fmt.Errorf("lint failed with %d error(s)", errors)
			}
			return nil
		},
	}
}
