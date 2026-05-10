package commands

import (
	"github.com/spf13/cobra"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

func newDoctorCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Read-only health audit of every installed package",
		Long: `doctor performs a read-only audit and surfaces every issue it finds:

  - modified files (drift)
  - locally-deleted files
  - stale install locations (project dir gone)
  - upstream-removed files (using the cached clone, no fetch)
  - orphaned packages (market removed from config)

doctor never touches disk or the install database, never performs network
I/O, and is safe to run at any time. Run mct refresh first if you need a
fresh upstream view.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			market, _ := cmd.Flags().GetString("market")
			jsonOut, _ := cmd.Flags().GetBool("json")

			report, err := svc.Doctor.Doctor(service.DoctorOpts{Market: market})
			if err != nil {
				return err
			}

			if jsonOut {
				return printJSON(cmd.OutOrStdout(), report)
			}

			if !report.HasFindings() {
				cmd.Println("ok  everything looks healthy")
				return nil
			}

			if len(report.StaleLocations) > 0 {
				cmd.Println("\n--  Stale install locations (project dir gone):")
				for _, l := range report.StaleLocations {
					cmd.Printf("    %s@%s -> %s\n", l.Market, l.Profile, l.Location)
				}
			}
			if len(report.OrphanedPackages) > 0 {
				cmd.Println("\n!!  Orphaned packages (market removed from config):")
				for _, p := range report.OrphanedPackages {
					cmd.Printf("    %s@%s in %s\n", p.Market, p.Profile, p.Location)
				}
			}
			if len(report.LocallyDeleted) > 0 {
				cmd.Println("\n--  Locally-deleted files:")
				for _, f := range report.LocallyDeleted {
					cmd.Printf("    %s@%s %s in %s\n", f.Market, f.Profile, f.Path, f.Location)
				}
			}
			if len(report.ModifiedFiles) > 0 {
				cmd.Println("\n~~  Locally-modified files (drift):")
				for _, f := range report.ModifiedFiles {
					cmd.Printf("    %s@%s %s in %s\n", f.Market, f.Profile, f.Path, f.Location)
				}
			}
			if len(report.UpstreamRemoved) > 0 {
				cmd.Println("\n>>  Upstream-removed files (still local):")
				for _, f := range report.UpstreamRemoved {
					cmd.Printf("    %s@%s %s\n", f.Market, f.Profile, f.Path)
				}
			}

			cmd.Println("\nRun mct sync to remediate (interactive restore + prune).")
			_ = opts
			return nil
		},
	}
	cmd.Flags().String("market", "", "filter to market")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}
