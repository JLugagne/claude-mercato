package commands

import (
	"github.com/spf13/cobra"
)

func newReadmeCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "readme <market> [path]",
		Short: "Show README.md from a market",
		Long:  "Display a README.md file from a market repository.\nPath defaults to README.md (root). Common paths: skills/README.md, agents/README.md",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			listAll, _ := cmd.Flags().GetBool("list")

			market := args[0]

			if listAll {
				readmes, err := svc.Readmes.ListReadmes(market)
				if err != nil {
					return err
				}
				if len(readmes) == 0 {
					cmd.Println("  No README.md files found in market " + market)
					return nil
				}
				for _, r := range readmes {
					cmd.Printf("  %s/%s\n", r.Market, r.Path)
				}
				return nil
			}

			path := "README.md"
			if len(args) == 2 {
				path = args[1]
			}

			readme, err := svc.Readmes.Readme(market, path)
			if err != nil {
				return err
			}

			cmd.Printf("  %s/%s\n\n", readme.Market, readme.Path)
			cmd.Print(readme.Content)
			if len(readme.Content) > 0 && readme.Content[len(readme.Content)-1] != '\n' {
				cmd.Println()
			}
			return nil
		},
	}
	cmd.Flags().Bool("list", false, "list all README.md files in the market")
	return cmd
}
